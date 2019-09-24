// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package state

import (
	"fmt"
	"sort"
	"encoding/json"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/libs/trie"
	"github.com/lianxiangcloud/linkchain/types"
)

type accountJSON struct {
	Balance     string      `json:"balance"`
	Nonce       uint64      `json:"nonce"`
	Root        string      `json:"-"`
	CodeHash    string      `json:"codeHash"`
	StorageLen  int         `json:"storageLen"`
	StorageHash common.Hash `json:"storageHash"`
}

func (s *StateDB) JSONDumpKV() []string {
	wdb, ok := s.db.(*wrappedDB)
	if !ok {
		panic(fmt.Errorf("db is not a kv db"))
	}
	if wdb.isTrie {
		return s.JSONDumpTrie()
	}

	accSlice := make(sort.StringSlice, 0)

	it := wdb.db.NewIteratorWithPrefix(nil)
	count := 0
	errCount := 0
	for ; it.Valid(); it.Next() {
		if len(it.Key()) != common.HashLength {
			continue
		}

		addr, val := it.Key(), it.Value()
		var data Account
		if err := ser.DecodeBytes(val, &data); err != nil {
			errCount++
			fmt.Printf("%d:%x\n", errCount, addr)
			continue
		}

		account := accountJSON{
			Balance:  data.Balance.String(),
			Nonce:    data.Nonce,
			Root:     common.Bytes2Hex(data.Root[:]),
			CodeHash: common.Bytes2Hex(data.CodeHash),
		}

		storageKVs := make(sort.StringSlice, 0)
		storageIt := wdb.db.NewIteratorWithPrefix(addr)
		storageKVCount := 0
		for ; storageIt.Valid(); storageIt.Next() {
			skey := storageIt.Key()
			sval := storageIt.Value()
			if len(skey) == common.HashLength {
				continue
			}
			storageKVCount++
			storageKVs = append(storageKVs, fmt.Sprintf("%x:%x", skey[32:], sval))
		}
		storageKVs.Sort()
		account.StorageLen = storageKVCount
		account.StorageHash = types.RlpHash(storageKVs)
		bs, err := json.Marshal(account)
		if err != nil {
			panic(err)
		}
		accSlice = append(accSlice, fmt.Sprintf("0x%x-%s", addr, bs))

		count++
	}

	accSlice.Sort()
	return accSlice
}

func (s *StateDB) JSONDumpTrie() []string {
	wdb, ok := s.db.(*wrappedDB)
	if !ok {
		panic(fmt.Errorf("db is not a kv db"))
	}
	if !wdb.isTrie {
		return s.JSONDumpKV()
	}

	accSlice := make(sort.StringSlice, 0)
	it := trie.NewIterator(s.trie.NodeIterator(nil))
	count := 0
	for it.Next() {
		key, val := it.Key, it.Value
		if len(key) != common.HashLength {
			continue
		}

		addr := s.trie.GetKey(key)
		var data Account
		if err := ser.DecodeBytes(val, &data); err != nil {
			panic(err)
		}

		obj := newObject(nil, common.BytesToAddress(addr), data)
		account := accountJSON{
			Balance:  data.Balance.String(),
			Nonce:    data.Nonce,
			Root:     common.Bytes2Hex(data.Root[:]),
			CodeHash: common.Bytes2Hex(data.CodeHash),
		}
		storageKVs := make(sort.StringSlice, 0)
		storageIt := trie.NewIterator(obj.getTrie(s.db).NodeIterator(nil))
		storageKVCount := 0
		for storageIt.Next() {
			storageKVCount++
			storageKVs = append(storageKVs, fmt.Sprintf("%x:%x", storageIt.Key, storageIt.Value))
		}
		storageKVs.Sort()
		account.StorageLen = storageKVCount
		account.StorageHash = types.RlpHash(storageKVs)
		bs, err := json.Marshal(account)
		if err != nil {
			panic(err)
		}
		accSlice = append(accSlice, fmt.Sprintf("%s-%s", crypto.Keccak256Hash(obj.address.Bytes()).Hex(), bs))

		count++
	}
	accSlice.Sort()
	return accSlice
}

func (s *StateDB) JSONDumpOldTrie() []string {
	accSlice := make(sort.StringSlice, 0)
	it := trie.NewIterator(s.trie.NodeIterator(nil))
	count := 0
	for it.Next() {
		if len(it.Key) == common.HashLength {
			addrBytes := s.trie.GetKey(it.Key)
			var data OldAccount
			if err := ser.DecodeBytes(it.Value, &data); err != nil {
				continue
			}
			account := accountJSON{
				Balance:  data.Balance.String(),
				Nonce:    data.Nonce,
				Root:     common.Bytes2Hex(data.Root[:]),
				CodeHash: common.Bytes2Hex(data.CodeHash),
			}
			storageKVs := make(sort.StringSlice, 0)
			stateDB, err := New(data.Root, s.db)
			storageIt := trie.NewIterator(stateDB.trie.NodeIterator(nil))
			storageKVCount := 0
			for storageIt.Next() {
				storageKVCount++
				storageKVs = append(storageKVs, fmt.Sprintf("%x:%x", storageIt.Key, storageIt.Value))
			}
			storageKVs.Sort()
			account.StorageLen = storageKVCount
			account.StorageHash = types.RlpHash(storageKVs)
			bs, err := json.Marshal(account)
			if err != nil {
				panic(err)
			}
			accSlice = append(accSlice, fmt.Sprintf("%s-%s", crypto.Keccak256Hash(addrBytes).Hex(), bs))

			count++
		} else {
			continue
		}
	}

	accSlice.Sort()
	return accSlice
}