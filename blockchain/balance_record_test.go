package blockchain

import (
	"testing"

	"github.com/lianxiangcloud/linkchain/types"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
)

func TestBalanceRecord(t *testing.T) {
	type balanceStoreTest struct {
		height uint64
		val    *types.BlockBalanceRecords
	}
	db := dbm.NewMemDB()
	brs := NewBalanceRecordStore(db, true)

	notExistKvs := []balanceStoreTest{
		{0, nil},
		{1, nil},
		{2, nil},
		{3, nil},
		{4, nil},
	}
	for _, kv := range notExistKvs {
		if kv.val != brs.Get(kv.height) {
			t.Fatal("Balance Get Faield.", "height", kv.height)
		}
	}

	kvs := []balanceStoreTest {
		{0, types.NewBlockBalanceRecords()},
		{1, types.NewBlockBalanceRecords()},
		{3, types.NewBlockBalanceRecords()},
		{4, types.NewBlockBalanceRecords()},
	}
	for _, kv := range kvs {
		brs.Save(kv.height, kv.val)
	}
	for _, kv := range kvs {
		if brs.Get(kv.height) == nil {
			t.Fatal("Balance Get return nil.", "height", kv.height)
		}
	}
}