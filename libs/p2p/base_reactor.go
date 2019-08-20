package p2p

import (
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/p2p/conn"
)

//--------------------------------------

//BaseReactor Provides Start, Stop, .Quit
type BaseReactor struct {
	cmn.BaseService
}

//NewBaseReactor new BaseReactor
func NewBaseReactor(name string, impl Reactor) *BaseReactor {
	return &BaseReactor{
		BaseService: *cmn.NewBaseService(nil, name, impl),
	}
}

func (*BaseReactor) GetChannels() []*conn.ChannelDescriptor        { return nil }
func (*BaseReactor) AddPeer(peer Peer)                             {}
func (*BaseReactor) RemovePeer(peer Peer, reason interface{})      {}
func (*BaseReactor) Receive(chID byte, peer Peer, msgBytes []byte) {}
