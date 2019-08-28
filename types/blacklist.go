package types

import (
	"sync"
	"github.com/lianxiangcloud/linkchain/libs/common"
)

var (
	once sync.Once
	bInstance *blacklist
)

type blacklist struct {
	addrs []common.Address
	mu    sync.Mutex
}

func BlacklistInstance() *blacklist {
	once.Do(func() {
		bInstance = &blacklist{
			addrs: make([]common.Address, 0),
		}
	})
	return bInstance
}

func (b *blacklist) SetBlackAddrs(blackAddrs []common.Address) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.addrs = blackAddrs
}

func (b *blacklist) GetBlackAddrs() []common.Address {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.addrs
}

func (b *blacklist) IsBlackAddress(addrFrom common.Address, addrTo common.Address) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, blackAddr := range b.addrs {
		if blackAddr.String() == addrFrom.String() ||
			blackAddr.String() == addrTo.String() {
			return true
		}
	}
	return false
}