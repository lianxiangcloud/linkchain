package main

import (
	"testing"
	"errors"
	"bytes"
	"os"
	"fmt"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/trie"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"

)

func TestDumper(t *testing.T) {
	addr := common.HexToAddress("ddd6e3a99f896cfefff1ff371b237bf5b5a5d776")
	storageRoot := common.HexToHash("0x683d8897ce3998f04f157051d14612cab5ce0cc7b3f6ca157af892db31663286")
	//err := doStorageDump(addr, storageRoot)
	//if err != nil {
	//	fmt.Println("doStorageDump failed.", "err", err.Error())
	//}
	level, err := db.NewGoLevelDB("state_full", os.TempDir(), 1)
	if err != nil {
		fmt.Println("NewGoLevelDB failed.", "err", err.Error())
		return
	}
	levelWrite, err := db.NewGoLevelDB("state_kv", os.TempDir(), 1)
	if err != nil {
		fmt.Println("NewGoLevelDB failed.", "err", err.Error())
		return
	}
	storageStateDb, err := state.New(storageRoot, state.NewDatabase(level))
	if err != nil {
		fmt.Printf("Error %s\n", err)
		return
	}
	fmt.Println("start dump storage", "storageRoot", storageRoot.String())
	storageMsg := storageStateDb.DumpStorage()
	fmt.Println("dump storage success.", "size", len(storageMsg))

	for key, val := range storageMsg {
		keyByte := common.Hex2Bytes(key)
		valByte := common.Hex2Bytes(val)
		keyByte = append(crypto.Keccak256Hash(addr.Bytes()).Bytes(), keyByte...)
		levelWrite.Set(keyByte, valByte)
	}
	fmt.Println("done")
}

func doStorageDump(addr common.Address, storageRoot common.Hash) error {
	fmt.Println("storageRoot", storageRoot.String())
	level, err := db.NewGoLevelDB("state_full", os.TempDir(), 1)
	if err != nil {
		fmt.Println("NewGoLevelDB failed.", "err", err.Error())
	}
	dstDb := db.NewMemDB()
	sched := NewStateSync(storageRoot, dstDb)
	queue := append([]common.Hash{}, sched.Missing(100)...)

	count := 0
	for len(queue) > 0 {
		results := make([]trie.SyncResult, len(queue))
		for i, hash := range queue {
			data := level.Get(hash[:])
			if len(data) == 0 {
				return errors.New("get stroage msg failed")
			}
			fmt.Println("doStorageDump", "key", hash.Hex(), "data", common.Bytes2Hex(data))
			results[i] = trie.SyncResult{Hash: hash, Data: data}
			count++
		}
		if _, index, err := sched.Process(results); err != nil {
			fmt.Printf("failed to process result #%d: %v\n", index, err)
		}
		if index, err := sched.Commit(dstDb); err != nil {
			fmt.Printf("failed to commit data #%d: %v\n", index, err)
		}
		queue = append(queue[:0], sched.Missing(100)...)
	}
	if count > 0 {
		fmt.Println("doStorageDump exec.", "addr", addr.String(),
			"storageRoot", storageRoot.String(), "count", count)
	}

	return nil
}

type TestStateDump struct {
	statedb *state.StateDB
	ldbFrom *db.GoLevelDB
	ldbDump *db.GoLevelDB
	target  *state.StateDB
}

func TestDumpKv(t *testing.T) {
	stateRoot := common.HexToHash("0x44648cebef840906a0e09686f96263c23c6d84659db702b203fdbc967d0b267b")
	level, err := db.NewGoLevelDB("state_full", os.TempDir(), 1)
	if err != nil {
		fmt.Println("NewGoLevelDB failed.", "err", err.Error())
		return
	}
	leveldump, err := db.NewGoLevelDB("dump_state", os.TempDir(), 1)
	if err != nil {
		fmt.Println("NewGoLevelDB failed.", "err", err.Error())
		return
	}
	accounts, err := accountList(level, stateRoot)
	if err != nil {
		fmt.Println("get account list failed.", "err", err.Error())
		return
	}
	fmt.Println("account counts", len(accounts))

	statedb, err := state.New(stateRoot, state.NewDatabase(level))
	if err != nil {
		fmt.Printf("Error %s\n", err)
		return
	}
	target, err := state.New(common.EmptyHash, state.NewKeyValueDBWithCache(leveldump, 128, false, 0))
	if err != nil {
		fmt.Printf("Error %s\n", err.Error())
		return
	}
	tdk := TestStateDump{
		statedb: statedb,
		ldbFrom: level,
		ldbDump: leveldump,
		target:  target,
	}
	err = tdk.dumpKv(accounts)
	if err != nil {
		fmt.Println("test state dump failed", "err", err.Error())
		return
	}
}

func accountList(ldb *db.GoLevelDB, root common.Hash) (map[common.Address]state.Account, error) {
	allAccounts := make(map[common.Address]state.Account)
	sdb, err := state.New(root, state.NewDatabase(ldb))
	if err != nil {
		fmt.Printf("Error %s\n", err)
		return nil, err
	}
	accountlist := sdb.DumpAccounts()
	fmt.Println("dump account addrs success", "accounts num", len(accountlist))

	for addr, accountBytes := range accountlist {
		oAccount := state.Account{}
		err := ser.DecodeBytes(accountBytes, &oAccount)
		if err != nil {
			fmt.Println("ser DecodeBytes failed.", "err", err.Error())
			return nil, err
		}
		allAccounts[addr] = oAccount
	}
	return allAccounts, nil
}

func (s *TestStateDump) dumpKv(addressList map[common.Address]state.Account) error {
	fmt.Println("start dumpKv...")
	count := 1
	for addr, oldAccount := range addressList {
		if count % 10000 == 0 {
			fmt.Println("dump ing. current index is", count)
		}
		var account state.Account
		account.Root     = oldAccount.Root
		account.Balance  = oldAccount.Balance
		account.CodeHash = oldAccount.CodeHash
		account.Nonce    = oldAccount.Nonce
		var code []byte
		if !bytes.Equal(account.CodeHash, crypto.Keccak256(nil)) {
			code = s.ldbFrom.Get(account.CodeHash)
		}
		err := s.insertKvStateObject(account, addr, code)
		if err != nil {
			fmt.Println("insertStateObject exec failed.", err.Error())
			return err
		}
		count++
	}
	root, err := s.commitKv()
	if err != nil {
		fmt.Println("StateDump commit failed.", "err", err.Error())
		return err
	}
	fmt.Println("dump finish.", "root", root.String())
	return nil
}

func (s *TestStateDump) insertKvStateObject(account state.Account, addr common.Address, code []byte) error {
	if code != nil {
		s.target.SetCode(addr, code)
		codehash := s.target.GetCodeHash(addr)
		if !bytes.Equal(codehash[:], account.CodeHash) {
			fmt.Printf("CodeHash is different  address=%v ,ori hash=%v , new hash=%v", addr.String(),
				common.Bytes2Hex(account.CodeHash), s.target.GetCodeHash(addr).Hex())
			return fmt.Errorf("CodeHash is different  address=%v ,ori hash=%v , new hash=%v",
				addr.String(), common.Bytes2Hex(account.CodeHash), s.target.GetCodeHash(addr).Hex())
		}
	}
	s.target.SetBalance(addr, account.Balance)
	s.target.SetNonce(addr, account.Nonce)

	storageRoot := account.Root
	if storageRoot != common.EmptyHash &&
		storageRoot.String() != "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421" {
		storageStateDb, err := state.New(storageRoot, state.NewDatabase(s.ldbFrom))
		if err != nil {
			fmt.Printf("Error %s\n", err)
			return nil
		}
		fmt.Println("start dump storage", "storageRoot", storageRoot.String())
		storageMsg := storageStateDb.DumpStorage()
		fmt.Println("dump storage success.", "size", len(storageMsg))
		batch := s.ldbDump.NewBatch()
		count := 0
		for key, val := range storageMsg {
			count++
			keyByte := common.Hex2Bytes(key)
			valByte := common.Hex2Bytes(val)
			if addr.Hex() == common.HexToAddress("ddd6e3a99f896cfefff1ff371b237bf5b5a5d776").Hex() {
				fmt.Println("addr", addr.Hex(), "key", key, "val", val)
			}
			keyByte = append(crypto.Keccak256Hash(addr.Bytes()).Bytes(), keyByte...)
			batch.Set(keyByte, valByte)
			if count % 10000 == 0 {
				batch.Commit()
				batch = s.ldbDump.NewBatch()
			}
		}
		batch.Commit()
	}

	return nil
}

func (s *TestStateDump) commitKv() (common.Hash, error) {
	root, err := s.target.Commit(false, 0)
	if err != nil {
		return common.EmptyHash, err
	}
	s.target.Database().TrieDB().Commit(root, false)
	s.target.Reset(root)

	return root, nil
}