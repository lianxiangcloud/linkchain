package blockchain

import (
	"fmt"

	"github.com/lianxiangcloud/linkchain/types"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)


const (
	key_pre string = "bbr_"
)

type BalanceRecordStore struct {
	db     dbm.DB
	isOpen bool
}

func NewBalanceRecordStore(db dbm.DB, openFlag bool) *BalanceRecordStore {
	return &BalanceRecordStore{
		db:     db,
		isOpen: openFlag,
	}
}

func (b *BalanceRecordStore) Save(blockHeight uint64, bbr *types.BlockBalanceRecords) {
	if !b.isOpen {
		return
	}
	key := calBlockBalanceRecordsKey(blockHeight)
	val, err := ser.EncodeToBytes(bbr)
	if err != nil {
		panic(err)
	}
	b.db.Set(key, val)
}

func (b *BalanceRecordStore) Get(blockHeight uint64) *types.BlockBalanceRecords {
	val := b.db.Get(calBlockBalanceRecordsKey(blockHeight))
	if len(val) == 0 {
		return nil
	}
	blr := &types.BlockBalanceRecords{}
	ser.DecodeBytes(val, blr)
	return blr
}

func calBlockBalanceRecordsKey(height uint64) []byte {
	return []byte(fmt.Sprintf("%s%d", key_pre, height))
}
