package p2p

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"time"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p/netutil"
	"github.com/lianxiangcloud/linkchain/libs/p2p/upnp"
	"github.com/lianxiangcloud/linkchain/types"
)

var (
	//ListenerBindFunc is the gloable fuction
	ListenerBindFunc = DefaultBindListener
)

// Listener is a network listener for stream-oriented protocols, providing
// convenient methods to get listener's internal and external addresses.
// Clients are supposed to read incoming connections from a channel, returned
// by Connections() method.
type Listener interface {
	Connections() <-chan net.Conn
	ExternalAddress() *NetAddress
	ExternalAddressHost() string
	String() string
	Stop() error
}

// DefaultListener is a cmn.Service, running net.Listener underneath.
// Optionally, UPnP is used upon calling NewDefaultListener to resolve external
// address.
type DefaultListener struct {
	cmn.BaseService

	listener    net.Listener
	extAddr     *NetAddress
	connections chan net.Conn
}

var _ Listener = (*DefaultListener)(nil)

const (
	numBufferedConnections = 10
	DefaultExternalPort    = 8770
	maxTryCount            = 2000
	maxPort                = 65535
	upnpMaxDuration        = (time.Duration(60) * time.Second)
	maxBindFailedNum       = 500
)

func SplitHostPort(addr string) (host string, port int) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	port, err = strconv.Atoi(portStr)
	if err != nil {
		panic(err)
	}
	return host, port
}

// DefaultBindListener creates a new DefaultListener on lAddr, optionally trying
// to determine external address using UPnP.

func DefaultBindListener(nodeType types.NodeType, fullListenAddrString string, externalAddrString string, logger log.Logger) (tcpListener net.Listener, extAddr *NetAddress, udpConn *net.UDPConn, isUpnpSuccess bool) {
	var listenTcpAddr string
	var isDHTNet = false
	if nodeType == types.NodePeer {
		isDHTNet = true
	}
	if len(fullListenAddrString) > 0 {
		listenTcpAddr = fullListenAddrString
	} else {
		listenTcpAddr = fmt.Sprintf(":%d", DefaultExternalPort)
	}

	// Split protocol, address, and port.
	protocol, lAddr := cmn.ProtocolAndAddress(listenTcpAddr)
	lAddrIP, lAddrPort := SplitHostPort(lAddr)

	// Create listener
	tcpListener = listenFreePort(lAddrPort, lAddrIP, protocol, logger)
	if tcpListener == nil { //bind port from lAddrPort to
		return nil, nil, nil, false
	}
	if externalAddrString != "" {
		var err error
		_, listenerPort := SplitHostPort(tcpListener.Addr().String())
		tmpExtAddrAndPort := fmt.Sprintf("%s:%d", externalAddrString, listenerPort)
		logger.Info("DefaultBindListener", "tmpExtAddr", tmpExtAddrAndPort)
		extAddr, err = NewNetAddressString(tmpExtAddrAndPort)
		if err != nil {
			panic(fmt.Sprintf("Error in ExternalAddress: %v", err))
		}
	} 
	// Actual listener local IP & port
	listenerIP, listenerPort := SplitHostPort(tcpListener.Addr().String())
	logger.Info("tcpListener", "ip", listenerIP, "port", listenerPort)

	inAddrAny := lAddrIP == "" || lAddrIP == "0.0.0.0"

	// Otherwise just use the local address.
	if extAddr == nil {
		defaultToIPv4 := inAddrAny
		extAddr = GetNaiveExternalAddress(defaultToIPv4, listenerPort, false, logger)
	}
	if extAddr == nil {
		panic("Could not determine external address!")
	}
	if isDHTNet {
		var bindUdpAddr string
		if isUpnpSuccess {
			bindUdpAddr = extAddr.String()
		} else {
			bindUdpAddr = fmt.Sprintf(":%d", listenerPort)
		}
		logger.Debug("DefaultBindListener", "bindUdpAddr", bindUdpAddr)
		addr, err := net.ResolveUDPAddr("udp", bindUdpAddr)
		if err != nil {
			logger.Error("NewDefaultListener", "ResolveUDPAddr err", err, "addr", addr)
		} else {
			udpConn, err = net.ListenUDP("udp", addr)
			if err != nil {
				logger.Error("NewDefaultListener", "ListenUDP err", err, "addr", addr)
			}
		}
	}
	return
}

// NewDefaultListener creates a new DefaultListener on lAddr
func NewDefaultListener(
	nodeType types.NodeType,
	fullListenAddrString string,
	externalAddrString string,
	logger log.Logger) (Listener, *net.UDPConn, bool) {

	if logger == nil {
		logger = log.NewNopLogger()
	}
	logger.Info("NewDefaultListener", "nodeType", nodeType, "fullListenAddrString", fullListenAddrString, "externalAddrString", externalAddrString)
	tcpListener, extAddr, udpConn, upnpFlag := ListenerBindFunc(nodeType, fullListenAddrString, externalAddrString, logger)
	if tcpListener == nil {
		return nil, nil, false
	}

	dl := &DefaultListener{
		listener:    tcpListener,
		extAddr:     extAddr,
		connections: make(chan net.Conn, numBufferedConnections),
	}
	dl.BaseService = *cmn.NewBaseService(logger, "DefaultListener", dl)
	err := dl.Start() // Started upon construction
	if err != nil {
		logger.Error("Error starting base service", "err", err)
	}
	return dl, udpConn, upnpFlag
}

func listenFreePort(internalPort int, addr string, protocol string, logger log.Logger) net.Listener {
	var startPort = internalPort
	var i, maxTryCount = 0, maxTryCount
	for ; i < maxTryCount && startPort < maxPort; i++ {
		bindAddr := fmt.Sprintf("%s:%d", addr, startPort)
		listener, err := net.Listen(protocol, bindAddr)
		if err != nil {
			logger.Info("ListenDirectCon failed,maybe you should change another port", "err", err)
			startPort++
		} else {
			return listener
		}
	}
	return nil
}

func StartUpnpLoop(oldTcpListener net.Listener, log log.Logger) (net.Listener, *NetAddress) {
	log.Debug("startUpnpLoop")
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Error("InterfaceAddrs get failed", "err", err)
		return oldTcpListener, nil
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				if !netutil.IsLAN(ipnet.IP) {
					log.Info("all ready in public network,do not need upnp", "addr", oldTcpListener.Addr().String())
					listenerIP, listenerPort := SplitHostPort(oldTcpListener.Addr().String())
					inAddrAny := listenerIP == "" || listenerIP == "0.0.0.0"
					if inAddrAny {
						log.Info("startUpnpLoop inAddrAny", "ipnet.IP", ipnet.IP.String())
						return oldTcpListener, NewNetAddressIPPort(ipnet.IP, uint16(listenerPort))
					} else {
						log.Info("startUpnpLoop ", "listenerIP", listenerIP)
						return oldTcpListener, NewNetAddressIPPort(net.ParseIP(listenerIP), uint16(listenerPort))
					}
				}
			}
		}
	}
	_, listenerPort := SplitHostPort(oldTcpListener.Addr().String())
	var bindPort uint16 = uint16(listenerPort)
	var newListener net.Listener = oldTcpListener
Loop:
	failedcount := 0
	proxy := upnp.NewProxy(bindPort, "blockchain_p2pUpnp", log)
	proxy.Start()
	tick := time.NewTicker(upnpMaxDuration)
	select {
	case addr, ok := <-proxy.ExtAddrChan:
		if ok {
			if len(addr.IP) == 0 {
				log.Info("len(addr.IP) is 0")
				return newListener, nil
			} else {
				if addr.Port != bindPort { //directSvrinfo.port have been mapped,we should close pre bindport socket and  try to bind the port that have success mapped
					log.Debug("upnp success,try to rebind to the upnp port", "upnp_port", addr.Port)
					newListener.Close()
					newListener = listenFreePort(int(addr.Port), addr.IP, "tcp", log) //Try to bind from the addr.Port
					if newListener == nil {
						log.Warn("startUpnpLoop listen failed")
						return nil, nil
					} else {
						_, listenerPort := SplitHostPort(newListener.Addr().String())
						bindPort = uint16(listenerPort)
						if bindPort != addr.Port { //upnp map success,but mapped port bind failed,so we should try to remapped directSvrinfo.port
							failedcount++
							if failedcount < maxBindFailedNum {
								goto Loop
							} else {
								log.Warn("all mapped port are bind failed")
								return newListener, nil
							}
						} else { //success
							log.Debug(" rebind to the upnp port success", "port", addr.Port)
							return newListener, NewNetAddressIPPort(net.ParseIP(addr.IP), addr.Port)
						}
					}
				} else {
					log.Debug("upnp success,add port mapping", "ip", addr.IP, "port", addr.Port)
					return newListener, NewNetAddressIPPort(net.ParseIP(addr.IP), addr.Port)
				}
			}
		} else {
			log.Info("upnp failed")
			return newListener, nil
		}
	case <-tick.C:
		return newListener, nil
	}
}

// OnStart implements cmn.Service by spinning a goroutine, listening for new
// connections.
func (l *DefaultListener) OnStart() error {
	if err := l.BaseService.OnStart(); err != nil {
		return err
	}
	go l.listenRoutine()
	return nil
}

// OnStop implements cmn.Service by closing the listener.
func (l *DefaultListener) OnStop() {
	l.BaseService.OnStop()
	l.listener.Close() // nolint: errcheck
}

// Accept connections and pass on the channel
func (l *DefaultListener) listenRoutine() {
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			log.Error("listenRoutine: failed to accept connection", "err", err)
		} else {
			l.Logger.Debug("listenRoutine", "recv new con", conn.RemoteAddr(), "len(connections)", len(l.connections))
		}
		if !l.IsRunning() {
			l.Logger.Info("listenRoutine exit")
			break // Go to cleanup
		}

		// listener wasn't stopped,
		// yet we encountered an error.
		if err != nil {
			panic(err)
		}
		if conn != nil {
			l.Logger.Info("listenRoutine", "recv new con", conn.RemoteAddr(), "len(connections)", len(l.connections))
		}
		l.connections <- conn
	}

	// Cleanup
	close(l.connections)
	for range l.connections {
		// Drain
	}
}

// Connections returns a channel of inbound connections.
// It gets closed when the listener closes.
// It is the callers responsibility to close any connections received
// over this channel.
func (l *DefaultListener) Connections() <-chan net.Conn {
	return l.connections
}

// ExternalAddress returns the external NetAddress (publicly available,
// determined using either UPnP or local resolver).
func (l *DefaultListener) ExternalAddress() *NetAddress {
	return l.extAddr
}

// ExternalAddressHost returns the external NetAddress IP string. If an IP is
// IPv6, it's wrapped in brackets ("[2001:db8:1f70::999:de8:7648:6e8]").
func (l *DefaultListener) ExternalAddressHost() string {
	ip := l.ExternalAddress().IP
	if isIpv6(ip) {
		// Means it's ipv6, so format it with brackets
		return "[" + ip.String() + "]"
	}
	return ip.String()
}

func (l *DefaultListener) String() string {
	return fmt.Sprintf("Listener(@%v)", l.extAddr)
}

func isIpv6(ip net.IP) bool {
	v4 := ip.To4()
	if v4 != nil {
		return false
	}

	ipString := ip.String()

	// Extra check just to be sure it's IPv6
	return (strings.Contains(ipString, ":") && !strings.Contains(ipString, "."))
}

// TODO: use syscalls: see issue #712
func GetNaiveExternalAddress(defaultToIPv4 bool, port int, settleForLocal bool, logger log.Logger) *NetAddress {
	logger.Info("Getting Native external address")
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(cmn.Fmt("Could not fetch interface addresses: %v", err))
	}

	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if defaultToIPv4 || !isIpv6(ipnet.IP) {
			v4 := ipnet.IP.To4()
			if v4 == nil || (!settleForLocal && v4[0] == 127) {
				// loopback
				continue
			}
		} else if !settleForLocal && ipnet.IP.IsLoopback() {
			// IPv6, check for loopback
			continue
		}

		na := &NetAddress{IP: ipnet.IP}
		if (!settleForLocal) && (!na.Routable()) {
			continue
		}

		return NewNetAddressIPPort(ipnet.IP, uint16(port))
	}

	// try again, but settle for local
	logger.Info("Node may not be connected to internet. Settling for local address")
	return GetNaiveExternalAddress(defaultToIPv4, port, true, logger)
}
