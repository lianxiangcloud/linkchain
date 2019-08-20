package p2p

import (
	"net"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/p2p/conn"
)

type ChannelDescriptor = conn.ChannelDescriptor
type ConnectionStatus = conn.ConnectionStatus

//P2PManager to control all the p2p connections
type P2PManager interface {
	cmn.Service
	GetByID(peerID string) Peer
	StopPeerForError(peer Peer, reason interface{})
	Reactor(name string) Reactor
	AddReactor(name string, reactor Reactor) Reactor
	Broadcast(chID byte, msgEncodeBytes []byte) chan bool //broadcast  msgEncodeBytes from chID channel to all nodes that already connected
	BroadcastE(chID byte, peerID string, msgEncodeBytes []byte) chan bool
	Peers() IPeerSet //all peers that already connected
	LocalNodeInfo() NodeInfo
	NumPeers() (outbound, inbound, dialing int) //return the num of  out\in\dialing connections
	MarkBadNode(nodeInfo NodeInfo)              //mark bad node,when badnode connect us next time,we will disconnect it
	CloseAllConnection()                        //close all the connection that my node already connected
}

//Peer is the single connection with other node
type Peer interface {
	cmn.Service
	ID() string // peer's ID
	RemoteAddr() net.Addr
	NodeInfo() NodeInfo // peer's info
	IsOutbound() bool   //IsOutbound returns true if the connection is outbound, false otherwise.
	Status() ConnectionStatus
	Send(channelID byte, msgEncodeBytes []byte) bool //try send  msgEncodeBytes blocked until send success or timeout
	TrySend(chID byte, msgEncodeBytes []byte) bool   //try send  msgEncodeBytes unblocked
	Close() error                                    //support close many times(if already closed,when call Close again,return error)
	Set(key string, data interface{})
	Get(key string) interface{}
}

//Reactor is the interface to achieve different business logic processing by control the same conneciton
type Reactor interface {
	cmn.Service                                          // Start, Stop
	RemovePeer(peer Peer, reason interface{})            //peer alraedy disconnect peer,just a notifacation
	AddPeer(peer Peer)                                   //recieve new connection
	Receive(chID byte, peer Peer, msgEncodeBytes []byte) //recieve data from  peer's chID channel,it should not blocked
	GetChannels() []*ChannelDescriptor
}
