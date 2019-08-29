package types

import (
	"sync"
	"fmt"
	"encoding/json"

	"github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
)

var (
	once      sync.Once
	bInstance *blacklist
)

const (
	optDelBlackAddress = uint32(0)
	optAddBlackAddress = uint32(1)

	strOptDelBlackAddress = "delBlackAddress"
	strOptAddBlackAddress = "addBlackAddress"
	strBlackAddressOptLen = 15
	strAddressLength      = 42

	blackAddrKeyPre = "bl_"
)

type blacklistContractRet struct {
	BlacklistChangesStr string `json:"ret"`
}

type blacklistChange struct {
	addr common.Address
	opt  uint32
}

type blacklist struct {
	addrs            map[common.Address]struct{}
	rwMu             sync.RWMutex
	blacklistChanges []blacklistChange
	changesMu        sync.Mutex
	db               dbm.DB
}

func BlacklistInstance() *blacklist {
	once.Do(func() {
		bInstance = &blacklist{
			addrs:            make(map[common.Address]struct{}, 0),
			blacklistChanges: make([]blacklistChange, 0),
		}
	})
	return bInstance
}

func (b *blacklist) Init(db dbm.DB) {
	b.setDB(db)
	b.getBlacklistFromDB()
}

func (b *blacklist) setDB(db dbm.DB) {
	b.db = db
}

func (b *blacklist) getBlacklistFromDB() {
	b.rwMu.Lock()
	defer b.rwMu.Unlock()
	iter := b.db.NewIteratorWithPrefix([]byte(blackAddrKeyPre))
	for ;iter.Valid(); iter.Next() {
		b.addrs[common.BytesToAddress(iter.Value())] = struct{}{}
	}
}

func (b *blacklist) DealBlackAddrsChanges(msg []byte) {
	bc := blacklistContractRet{}
	err := json.Unmarshal(msg, &bc)
	if err != nil {
		return
	}
	strBlackAddrsChanges := bc.BlacklistChangesStr
	if len(strBlackAddrsChanges) <= strBlackAddressOptLen {
		return
	}
	strOpt   := strBlackAddrsChanges[:strBlackAddressOptLen]
	strAddrs := strBlackAddrsChanges[strBlackAddressOptLen:]
	var opt uint32
	if strOpt == strOptAddBlackAddress {
		opt = optAddBlackAddress
	} else if strOpt == strOptDelBlackAddress {
		opt = optDelBlackAddress
	}
	if len(strAddrs) == 0 || len(strAddrs)%strAddressLength != 0 {
		fmt.Println("strAddrs length is ", len(strAddrs), "strAddrs", strAddrs)
		return
	}
	changes := make([]blacklistChange, 0)
	for index := 0; index < len(strAddrs); index += strAddressLength {
		change := blacklistChange{
			addr: common.HexToAddress(strAddrs[index:index+strAddressLength]),
			opt:  opt,
		}
		changes = append(changes, change)
	}
	b.changesMu.Lock()
	defer b.changesMu.Unlock()
	b.blacklistChanges = append(b.blacklistChanges, changes...)
}

func (b *blacklist) UpdateBlacklist() error {
	b.changesMu.Lock()
	defer b.changesMu.Unlock()
	if len(b.blacklistChanges) == 0 {
		return nil
	}
	batch := b.db.NewBatch()
	for _, change := range b.blacklistChanges {
		if change.opt == optAddBlackAddress {
			batch.Set(genBlackAddrKeyPre(change.addr.Hex()), change.addr.Bytes())
		} else if change.opt == optDelBlackAddress {
			batch.Delete(genBlackAddrKeyPre(change.addr.Hex()))
		}
	}
	err := batch.Commit()
	if err != nil {
		return err
	}

	b.rwMu.Lock()
	defer b.rwMu.Unlock()
	for _, change := range b.blacklistChanges {
		if change.opt == optAddBlackAddress {
			b.addrs[change.addr] = struct{}{}
		} else if change.opt == optDelBlackAddress {
			delete(b.addrs, change.addr)
		}
	}
	b.blacklistChanges = make([]blacklistChange, 0)

	return nil
}

func (b *blacklist) GetBlackAddrs() []common.Address {
	b.rwMu.RLock()
	defer b.rwMu.RUnlock()
	blacklist := make([]common.Address, 0)
	for blackAddr, _ := range b.addrs {
		blacklist = append(blacklist, blackAddr)
	}
	return blacklist
}

func (b *blacklist) IsBlackAddress(addrFrom common.Address, addrTo common.Address, tokenId common.Address) bool {
	b.rwMu.RLock()
	defer b.rwMu.RUnlock()

	if _, ok := b.addrs[addrFrom]; ok {
		return true
	}
	if _, ok := b.addrs[addrTo]; ok {
		return true
	}
	if _, ok := b.addrs[tokenId]; ok {
		return true
	}

	return false
}

func genBlackAddrKeyPre(addr string) []byte {
	return []byte(fmt.Sprintf("%s%s", blackAddrKeyPre, addr))
}