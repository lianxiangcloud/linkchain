package discover

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/libs/p2p/netutil"

	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

// Errors
var (
	errPacketTooSmall   = errors.New("too small")
	errBadHash          = errors.New("bad hash")
	errExpired          = errors.New("expired")
	errUnsolicitedReply = errors.New("unsolicited reply")
	errUnknownNode      = errors.New("unknown node")
	errTimeout          = errors.New("RPC timeout")
	errClockWarp        = errors.New("reply deadline too far in the future")
	errClosed           = errors.New("socket closed")
)

var (
	headSpace    = make([]byte, headSize)
	maxNeighbors int
)

const (
	respTimeout    = 500 * time.Millisecond
	expiration     = 20 * time.Second
	bondExpiration = 12 * time.Hour

	maxFindnodeFailures = 5                // nodes exceeding this limit are dropped
	ntpFailureThreshold = 32               // Continuous timeouts after which to check NTP
	ntpWarningCooldown  = 10 * time.Minute // Minimum amount of time to pass before repeating NTP warning
	// Discovery packets are defined to be no larger than 1280 bytes.
	// Packets larger than this size will be cut at the end and treated
	// as invalid because their hash won't match.
	maxPacketSize = 1280
)

const (
	macSize          = 256 / 8
	sigSize          = 73
	encodePubKeySize = 40
	headSize         = macSize + sigSize + encodePubKeySize // space of packet frame data
	pingVersion      = 4
)

// RPC packet types
const (
	pPing = iota + 1 // zero is 'reserved'
	pPong
	pFindnode
	pNeighbors
)

// RPC request structures
type (
	ping struct {
		Version    uint
		From, To   rpcEndpoint
		Expiration uint64
	}

	// pong is the reply to ping.
	pong struct {
		// This field should mirror the UDP envelope address
		// of the ping packet, which provides a way to discover the
		// the external address (after NAT).
		MyNodeInfo rpcNode

		ReplyTok   []byte // This contains the hash of the ping packet.
		Expiration uint64 // Absolute timestamp at which the packet becomes invalid.
		// Ignore additional fields (for forward compatibility).
	}

	// findnode is a query for nodes close to the given target.
	findnode struct {
		Target     common.NodeID
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
	}

	// neighbors is the reply to findnode.
	neighbors struct {
		Nodes      []rpcNode
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
	}
	rpcNode struct {
		IP  net.IP // len 4 for IPv4 or 16 for IPv6
		UDP uint16 // for discovery protocol
		TCP uint16 // for TCP protocol
		ID  common.NodeID
	}

	rpcEndpoint struct {
		IP  net.IP // len 4 for IPv4 or 16 for IPv6
		UDP uint16 // for discovery protocol
		TCP uint16 // for TCP protocol
	}
)

// replyMatcher represents a pending reply.
//
// Some implementations of the protocol wish to send more than one
// reply packet to findnode. In general, any neighbors packet cannot
// be matched up with a specific findnode packet.
//
// Our implementation handles this by storing a callback function for
// each pending reply. Incoming packets from a node are dispatched
// to all callback functions for that node.
type replyMatcher struct {
	// these fields must match in the reply.
	from  common.NodeID
	ip    net.IP
	ptype byte

	// time when the request must complete
	deadline time.Time

	// callback is called when a matching reply arrives. If it returns matched == true, the
	// reply was acceptable. The second return value indicates whether the callback should
	// be removed from the pending reply queue. If it returns false, the reply is considered
	// incomplete and the callback will be invoked again for the next matching reply.
	callback replyMatchFunc

	// errc receives nil when the callback indicates completion or an
	// error if no further reply is received within the timeout.
	errc chan error

	// reply contains the most recent reply. This field is safe for reading after errc has
	// received a value.
	reply packet
}

// reply is a reply packet from a certain node.
type reply struct {
	from common.NodeID
	ip   net.IP
	data packet
	// loop indicates whether there was
	// a matching request by sending on this channel.
	matched chan<- bool
}

// packet is implemented by all  protocol messages.
type packet interface {
	// preverify checks whether the packet is valid and should be handled at all.
	preverify(t *udp, from *net.UDPAddr, fromID common.NodeID, fromKey []byte) error
	// handle handles the packet.
	handle(t *udp, from *net.UDPAddr, fromID common.NodeID, mac []byte) //mac:Unique ID of the received message(hash)
	// packet name and type for logging purposes.
	name() string
	kind() byte
}

type replyMatchFunc func(interface{}) (matched bool, requestDone bool)

type udp struct {
	tab         *DhtTable
	conn        common.UDPConn
	db          common.P2pDBManager
	log         log.Logger
	localNode   *common.Node // metadata of the local node
	priv        crypto.PrivKey
	netrestrict *netutil.Netlist
	wg          sync.WaitGroup

	addReplyMatcher chan *replyMatcher
	gotreply        chan reply
	closing         chan struct{}
}

func init() {
	ser.RegisterInterface((*packet)(nil), nil)
	ser.RegisterConcrete(&ping{}, "udp_ping", nil)
	ser.RegisterConcrete(&pong{}, "udp_pong", nil)
	ser.RegisterConcrete(&findnode{}, "udp_findnode", nil)
	ser.RegisterConcrete(&neighbors{}, "udp_neighbors", nil)

	p := neighbors{Expiration: ^uint64(0)}
	maxSizeNode := rpcNode{IP: make(net.IP, 16), UDP: ^uint16(0), TCP: ^uint16(0)}
	for n := 0; ; n++ {
		p.Nodes = append(p.Nodes, maxSizeNode)
		data, err := ser.EncodeToBytesWithType(p)
		if err != nil {
			// If this ever happens, it will be caught by the unit tests.
			panic("cannot encode: " + err.Error())
		}
		if headSize+len(data)+1 >= maxPacketSize {
			maxNeighbors = n
			break
		}
	}
}

func newUDP(tab *DhtTable, con common.UDPConn, db common.P2pDBManager, self *common.Node, cfg common.Config, log log.Logger) *udp {
	log.Debug("newUDP")
	t := &udp{
		tab:             tab,
		conn:            con,
		db:              db,
		log:             log,
		priv:            cfg.PrivateKey,
		netrestrict:     cfg.NetRestrict,
		closing:         make(chan struct{}),
		gotreply:        make(chan reply),
		addReplyMatcher: make(chan *replyMatcher),
		localNode:       self,
	}
	return t
}

func (t *udp) start() {
	t.wg.Add(2)
	go t.loop()
	go t.readLoop()
}

// Close shuts down the socket and aborts any running queries.
func (t *udp) close() {
	close(t.closing)
	t.conn.Close()
	t.wg.Wait()
}

// readLoop runs in its own goroutine. it handles incoming UDP packets.
func (t *udp) readLoop() {
	t.log.Debug("readLoop")
	defer t.wg.Done()

	buf := make([]byte, maxPacketSize)
	for {
		nbytes, from, err := t.conn.ReadFromUDP(buf)
		if netutil.IsTemporaryError(err) {
			// Ignore temporary read errors.
			t.log.Debug("Temporary UDP read error", "err", err)
			continue
		} else if err != nil {
			// Shut down the loop for permament errors.
			if err != io.EOF {
				t.log.Debug("UDP read error", "err", err)
			}
			return
		}
		t.handlePacket(from, buf[:nbytes])
	}
}

func (t *udp) handlePacket(from *net.UDPAddr, buf []byte) error {
	packet, fromKey, hash, err := decode(buf, t.log)
	if err != nil {
		t.log.Info("Bad disc packet", "addr", from, "err", err)
		return err
	}
	fromID := common.TransPkbyteToNodeID(fromKey)
	err = packet.preverify(t, from, fromID, fromKey)
	t.log.Trace("<< "+packet.name(), "id", fromID, "addr", from, "err", err)
	if err == nil {
		packet.handle(t, from, fromID, hash)
	}
	return err
}

// loop runs in its own goroutine. it keeps track of
// the refresh timer and the pending reply queue.
func (t *udp) loop() {
	defer t.wg.Done()

	var (
		plist        = list.New()
		timeout      = time.NewTimer(0)
		nextTimeout  *replyMatcher // head of plist when timeout was last reset
		contTimeouts = 0           // number of continuous timeouts to do NTP checks
		ntpWarnTime  = time.Unix(0, 0)
	)
	<-timeout.C // ignore first timeout
	defer timeout.Stop()

	resetTimeout := func() {
		if plist.Front() == nil || nextTimeout == plist.Front().Value {
			return
		}
		// Start the timer so it fires when the next pending reply has expired.
		now := time.Now()
		for el := plist.Front(); el != nil; el = el.Next() {
			nextTimeout = el.Value.(*replyMatcher)
			if dist := nextTimeout.deadline.Sub(now); dist < 2*respTimeout {
				timeout.Reset(dist)
				return
			}
			// Remove pending replies whose deadline is too far in the
			// future. These can occur if the system clock jumped
			// backwards after the deadline was assigned.
			nextTimeout.errc <- errClockWarp
			plist.Remove(el)
		}
		nextTimeout = nil
		timeout.Stop()
	}

	for {
		resetTimeout()

		select {
		case <-t.closing:
			for el := plist.Front(); el != nil; el = el.Next() {
				el.Value.(*replyMatcher).errc <- errClosed
			}
			return

		case p := <-t.addReplyMatcher:
			p.deadline = time.Now().Add(respTimeout)
			plist.PushBack(p)

		case r := <-t.gotreply:
			var matched bool // whether any replyMatcher considered the reply acceptable.
			for el := plist.Front(); el != nil; el = el.Next() {
				p := el.Value.(*replyMatcher)
				if p.from == r.from && p.ptype == r.data.kind() && p.ip.Equal(r.ip) {
					ok, requestDone := p.callback(r.data)
					matched = matched || ok
					// Remove the matcher if callback indicates that all replies have been received.
					if requestDone {
						p.reply = r.data
						p.errc <- nil
						plist.Remove(el)
					}
					// Reset the continuous timeout counter (time drift detection)
					contTimeouts = 0
				}
			}
			r.matched <- matched

		case now := <-timeout.C:
			nextTimeout = nil

			// Notify and remove callbacks whose deadline is in the past.
			for el := plist.Front(); el != nil; el = el.Next() {
				p := el.Value.(*replyMatcher)
				if now.After(p.deadline) || now.Equal(p.deadline) {
					p.errc <- errTimeout
					plist.Remove(el)
					contTimeouts++
				}
			}
			// If we've accumulated too many timeouts, do an NTP time sync check
			if contTimeouts > ntpFailureThreshold {
				if time.Since(ntpWarnTime) >= ntpWarningCooldown {
					ntpWarnTime = time.Now()
					go checkClockDrift()
				}
				contTimeouts = 0
			}
		}
	}
}

// ensureBond solicits a ping from a node if we haven't seen a ping from it for a while.
// This ensures there is a valid endpoint proof on the remote end.
func (t *udp) ensureBond(toid common.NodeID, toaddr *net.UDPAddr) {
	tooOld := time.Since(t.db.LastPingReceived(toid, toaddr.IP)) > bondExpiration
	faileNum := t.db.FindFails(toid, toaddr.IP)
	if tooOld || faileNum > maxFindnodeFailures {
		t.log.Info("ensureBond", "tooOld", tooOld, "faile num", faileNum, "toid", toid.String(), "toaddr", toaddr)
		rm := t.sendPing(toid, toaddr, nil)
		err := <-rm.errc
		if err != nil {
			t.log.Info("sendPing to toid failed", "toid", toid, "IP", toaddr.IP, "err", err)
		}
		// Wait for them to ping back and process our pong.
		time.Sleep(respTimeout)
	}
}

func makeEndpoint(addr *net.UDPAddr, tcpPort uint16) rpcEndpoint {
	ip := net.IP{}
	if ip4 := addr.IP.To4(); ip4 != nil {
		ip = ip4
	} else if ip6 := addr.IP.To16(); ip6 != nil {
		ip = ip6
	}
	return rpcEndpoint{IP: ip, UDP: uint16(addr.Port), TCP: tcpPort}
}

func (t *udp) ourEndpoint() rpcEndpoint {
	n := t.localNode
	a := &net.UDPAddr{IP: n.IP, Port: int(n.UDP_Port)}
	return makeEndpoint(a, n.TCP_Port)
}

func (t *udp) write(toaddr *net.UDPAddr, toid common.NodeID, what string, packet []byte) error {
	_, err := t.conn.WriteToUDP(packet, toaddr)
	t.log.Trace(">> "+what, "id", toid, "addr", toaddr, "err", err)
	return err
}

func encode(priv crypto.PrivKey, req packet, logger log.Logger) (packet, hash []byte, err error) {
	name := req.name()
	b := new(bytes.Buffer)
	b.Write(headSpace)
	b.WriteByte(req.kind())
	msg := &req
	data, err := ser.EncodeToBytesWithType(msg)
	if err != nil {
		logger.Error(fmt.Sprintf("Can't encode %s packet", name), "err", err)
		return nil, nil, err
	}
	b.Write(data)
	packet = b.Bytes()
	sig, err := priv.Sign(crypto.Keccak256(packet[headSize:]))
	if err != nil {
		logger.Error(fmt.Sprintf("Can't sign %s packet", name), "err", err)
		return nil, nil, err
	}
	copy(packet[macSize:], sig.Bytes()) //macSize-sigSize:sig hash
	copy(packet[macSize+sigSize:], priv.PubKey().Bytes())
	// add the hash to the front. Note: this doesn't protect the
	// packet in any way. Our public key will be part of this hash in
	// The future.
	hash = crypto.Keccak256(packet[macSize:])
	copy(packet, hash) //0-macSize:mac hash  macSize should equel Keccak256
	return packet, hash, nil
}

func decode(buf []byte, logger log.Logger) (packet, []byte, []byte, error) {
	//logger.Debug("decode")
	if len(buf) < headSize+1 {
		return nil, nil, nil, errPacketTooSmall
	}
	hash, sig, fromKey, sigdata := buf[:macSize], buf[macSize:macSize+sigSize], buf[macSize+sigSize:headSize], buf[headSize:]
	//logger.Debug("decode", "fromKey", fromKey, "sig", sig)
	shouldhash := crypto.Keccak256(buf[macSize:])
	if !bytes.Equal(hash, shouldhash) {
		return nil, nil, nil, errBadHash
	}

	pkey, err := crypto.PubKeyFromBytes(fromKey)
	if err != nil {
		logger.Info("PubKeyFromBytes err")
		return nil, nil, hash, err
	}
	signature, err := crypto.SignatureFromBytes(sig)
	if err != nil {
		logger.Info("SignatureFromBytes err")
		return nil, nil, hash, err
	}
	if !pkey.VerifyBytes(crypto.Keccak256(sigdata), signature) {
		return nil, nil, hash, fmt.Errorf("decode VerifyBytes failed")
	}

	var req packet
	switch ptype := sigdata[0]; ptype {
	case pPing:
		req = new(ping)
	case pPong:
		req = new(pong)
	case pFindnode:
		req = new(findnode)
	case pNeighbors:
		req = new(neighbors)
	default:
		return nil, fromKey, hash, fmt.Errorf("unknown type: %d", ptype)
	}

	err = ser.DecodeBytesWithType(sigdata[1:], &req)
	if err != nil {
		logger.Info("DecodeBytesWithType err")
		return req, fromKey, hash, err
	}
	return req, fromKey, hash, nil
}

func (t *udp) send(toaddr *net.UDPAddr, toid common.NodeID, req packet) ([]byte, error) {
	packet, hash, err := encode(t.priv, req, t.log)
	if err != nil {
		return hash, err
	}
	return hash, t.write(toaddr, toid, req.name(), packet)
}

// handleReply dispatches a reply packet, invoking reply matchers. It returns
// whether any matcher considered the packet acceptable.
func (t *udp) handleReply(from common.NodeID, fromIP net.IP, req packet) bool {
	matched := make(chan bool, 1)
	select {
	case t.gotreply <- reply{from, fromIP, req, matched}:
		// loop will handle it
		return <-matched
	case <-t.closing:
		return false
	}
}

func (t *udp) makePing(toaddr *net.UDPAddr) *ping {
	return &ping{
		Version:    pingVersion,
		From:       t.ourEndpoint(),
		To:         makeEndpoint(toaddr, 0),
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}
}

// sendPing sends a ping message to the given node and invokes the callback
// when the reply arrives.
func (t *udp) sendPing(toid common.NodeID, toaddr *net.UDPAddr, callback func()) *replyMatcher {
	req := t.makePing(toaddr)
	packet, hash, err := encode(t.priv, req, t.log)
	if err != nil {
		errc := make(chan error, 1)
		errc <- err
		return &replyMatcher{errc: errc}
	}
	// Add a matcher for the reply to the pending reply queue. Pongs are matched if they
	// reference the ping we're about to send.
	rm := t.pending(toid, toaddr.IP, pPong, func(p interface{}) (matched bool, requestDone bool) {
		matched = bytes.Equal(p.(*pong).ReplyTok, hash)
		if matched && callback != nil {
			callback()
		}
		return matched, matched
	})
	// Send the packet.
	t.write(toaddr, toid, req.name(), packet)
	return rm
}

// pending adds a reply matcher to the pending reply queue.
// see the documentation of type replyMatcher for a detailed explanation.
func (t *udp) pending(id common.NodeID, ip net.IP, ptype byte, callback replyMatchFunc) *replyMatcher {
	ch := make(chan error, 1)
	p := &replyMatcher{from: id, ip: ip, ptype: ptype, callback: callback, errc: ch}
	select {
	case t.addReplyMatcher <- p:
		// loop will handle it
	case <-t.closing:
		ch <- errClosed
	}
	return p
}

func (t *udp) nodeFromRPC(sender *net.UDPAddr, rn rpcNode) (*node, error) {
	if rn.UDP == 0 || rn.TCP == 0 {
		return nil, errors.New("UDPPort==0 or TCPPort==0")
	}
	if err := netutil.CheckRelayIP(sender.IP, rn.IP); err != nil {
		return nil, err
	}
	if t.netrestrict != nil && !t.netrestrict.Contains(rn.IP) {
		return nil, errors.New("not contained in netrestrict whitelist")
	}
	n := wrapNode(&common.Node{IP: rn.IP, UDP_Port: rn.UDP, TCP_Port: rn.TCP, ID: rn.ID})
	err := n.ValidateComplete()
	return n, err
}

// findnode sends a findnode request to the given node and waits until
// the node has sent up to k neighbors.
func (t *udp) findnode(toid common.NodeID, toaddr *net.UDPAddr, target common.NodeID) ([]*node, error) {
	t.ensureBond(toid, toaddr)

	// Add a matcher for 'neighbours' replies to the pending reply queue. The matcher is
	// active until enough nodes have been received.
	nodes := make([]*node, 0, bucketSize)
	nreceived := 0
	rm := t.pending(toid, toaddr.IP, pNeighbors, func(r interface{}) (matched bool, requestDone bool) {
		reply := r.(*neighbors)
		t.log.Trace("findnode", "len(reply.Nodes)", len(reply.Nodes))
		for _, rn := range reply.Nodes {
			nreceived++
			n, err := t.nodeFromRPC(toaddr, rn)
			if err != nil {
				t.log.Info("Invalid neighbor node received", "id", rn.ID, "ip", rn.IP, "udp_port", rn.UDP, "fromAddr", toaddr, "err", err)
				continue
			}
			nodes = append(nodes, n)
		}
		return true, nreceived >= bucketSize
	})
	t.send(toaddr, toid, &findnode{
		Target:     target,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	})
	return nodes, <-rm.errc //Blocked until timeout or receive a specified number of node
}

// ping sends a ping message to the given node and waits for a reply.
func (t *udp) ping(n *common.Node) (err error, reply *pong) {
	rm := t.sendPing(n.ID, &net.UDPAddr{IP: n.IP, Port: int(n.UDP_Port)}, nil)
	err = <-rm.errc
	value, ok := rm.reply.(*pong)
	if ok {
		reply = value
	}
	return
}

// expired checks whether the given UNIX time stamp is in the past.
func expired(ts uint64) bool {
	return time.Unix(int64(ts), 0).Before(time.Now())
}

// checkBond checks if the given node has a recent enough endpoint proof.
func (t *udp) checkBond(id common.NodeID, ip net.IP) bool {
	lastPongTime := t.db.LastPongReceived(id, ip)
	sinceTime := time.Since(lastPongTime)
	t.log.Trace("checkBond", "lastPongTime", lastPongTime.Unix(), "sinceTime", sinceTime, "id", id, "ip", ip.String())
	return sinceTime < bondExpiration
}

func (req *ping) name() string { return "PING" }
func (req *ping) kind() byte   { return pPing }

func (req *ping) preverify(t *udp, from *net.UDPAddr, fromID common.NodeID, fromKey []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	return nil
}

func rapToRpcNode(node *common.Node) rpcNode {
	var copyNode rpcNode
	copyNode.ID = node.ID
	copyNode.IP = node.IP
	copyNode.UDP = node.UDP_Port
	copyNode.TCP = node.TCP_Port
	return copyNode
}

func (req *ping) handle(t *udp, from *net.UDPAddr, fromID common.NodeID, mac []byte) {
	// Reply.
	myNodeInfo := rapToRpcNode(t.localNode)
	t.send(from, fromID, &pong{
		MyNodeInfo: myNodeInfo,
		ReplyTok:   mac,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	})

	// Ping back if our last pong on file is too far in the past.
	n := wrapNode(&common.Node{IP: from.IP, UDP_Port: uint16(from.Port), TCP_Port: req.From.TCP, ID: fromID})
	if time.Since(t.db.LastPongReceived(n.ID, from.IP)) > bondExpiration {
		t.sendPing(fromID, from, func() {
			t.tab.addVerifiedNode(n)
		})
	} else {
		t.tab.addVerifiedNode(n)
	}

	// Update node database and endpoint predictor.
	t.db.UpdateLastPingReceived(n.ID, from.IP, time.Now())
}

// PONG
func (req *pong) name() string { return "PONG" }
func (req *pong) kind() byte   { return pPong }

func (req *pong) preverify(t *udp, from *net.UDPAddr, fromID common.NodeID, fromKey []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	if !t.handleReply(fromID, from.IP, req) {
		return errUnsolicitedReply
	}
	return nil
}

func (req *pong) handle(t *udp, from *net.UDPAddr, fromID common.NodeID, mac []byte) {
	//t.localNode.UDPEndpointStatement(from, &net.UDPAddr{IP: req.To.IP, Port: int(req.To.UDP)})
	now := time.Now()
	t.db.UpdateLastPongReceived(fromID, from.IP, now)
}

// FINDNODE

func nodeToRPC(n *node) rpcNode {
	return rpcNode{ID: n.ID, IP: n.IP, UDP: n.UDP_Port, TCP: n.TCP_Port}
}

func (req *findnode) name() string { return "FINDNODE" }
func (req *findnode) kind() byte   { return pFindnode }

func (req *findnode) preverify(t *udp, from *net.UDPAddr, fromID common.NodeID, fromKey []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	if !t.checkBond(fromID, from.IP) {
		// No endpoint proof pong exists, we don't process the packet. This prevents an
		// attack vector where the discovery protocol could be used to amplify traffic in a
		// DDOS attack. A malicious actor would send a findnode request with the IP address
		// and UDP port of the target as the source address. The recipient of the findnode
		// packet would then send a neighbors packet (which is a much bigger packet than
		// findnode) to the victim.
		return errUnknownNode
	}
	return nil
}

func (req *findnode) handle(t *udp, from *net.UDPAddr, fromID common.NodeID, mac []byte) {
	// Determine closest nodes.
	target := req.Target
	t.tab.mutex.Lock()
	closest := t.tab.closest(target, bucketSize, true).entries
	t.log.Trace("findnode", "len(closest)", len(closest), "myID", t.localNode.ID)
	t.tab.mutex.Unlock()

	// Send neighbors in chunks with at most maxNeighbors per packet
	// to stay below the packet size limit.
	p := neighbors{Expiration: uint64(time.Now().Add(expiration).Unix())}
	var sent bool
	for _, n := range closest {
		if netutil.CheckRelayIP(from.IP, n.IP) == nil {
			p.Nodes = append(p.Nodes, nodeToRPC(n))
		}
		if len(p.Nodes) == maxNeighbors {
			t.send(from, fromID, &p)
			p.Nodes = p.Nodes[:0] //Reply to up to maxNeighbors Nodes at a time
			sent = true
		}
	}
	t.log.Trace("findnode", "len(p.Nodes)", len(p.Nodes))
	if len(p.Nodes) > 0 || !sent {
		t.send(from, fromID, &p)
	}
}

// NEIGHBORS

func (req *neighbors) name() string { return "NEIGHBORS" }
func (req *neighbors) kind() byte   { return pNeighbors }

func (req *neighbors) preverify(t *udp, from *net.UDPAddr, fromID common.NodeID, fromKey []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	if !t.handleReply(fromID, from.IP, req) {
		return errUnsolicitedReply
	}
	return nil
}

func (req *neighbors) handle(t *udp, from *net.UDPAddr, fromID common.NodeID, mac []byte) {
}
