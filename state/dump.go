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
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/libs/trie"
)

type DumpAccount struct {
	Balance  string            `json:"balance"`
	Nonce    uint64            `json:"nonce"`
	Credits  uint64            `json:"credits"`
	Root     string            `json:"root"`
	CodeHash string            `json:"codeHash"`
	Code     string            `json:"code"`
	Storage  map[string]string `json:"storage"`

	Tokens map[common.Address]*big.Int `json:"tokens"`
}

type Dump struct {
	Root     string                 `json:"root"`
	Accounts map[string]DumpAccount `json:"accounts"`
}

func (s *StateDB) RawDump() Dump {
	dump := Dump{
		Root:     fmt.Sprintf("%x", s.trie.Hash()),
		Accounts: make(map[string]DumpAccount),
	}

	it := trie.NewIterator(s.trie.NodeIterator(nil))
	for it.Next() {
		if len(it.Key) == common.HashLength {
			addr := s.trie.GetKey(it.Key)
			var data Account
			if err := ser.DecodeBytes(it.Value, &data); err != nil {
				panic(err)
			}

			obj := newObject(nil, common.BytesToAddress(addr), data)
			account := DumpAccount{
				Balance:  data.Balance.String(),
				Nonce:    data.Nonce,
				Credits:  data.Credits,
				Root:     common.Bytes2Hex(data.Root[:]),
				CodeHash: common.Bytes2Hex(data.CodeHash),
				Code:     common.Bytes2Hex(obj.Code(s.db)),
				Tokens:   data.Tokens,
				Storage:  make(map[string]string),
			}
			storageIt := trie.NewIterator(obj.getTrie(s.db).NodeIterator(nil))
			for storageIt.Next() {
				account.Storage[common.Bytes2Hex(s.trie.GetKey(storageIt.Key))] = common.Bytes2Hex(storageIt.Value)
			}
			dump.Accounts[common.Bytes2Hex(addr)] = account
		}
	}

	return dump
}

func (s *StateDB) Dump() []byte {
	json, err := json.MarshalIndent(s.RawDump(), "", "    ")
	if err != nil {
		fmt.Println("dump err", err)
	}

	return json
}

type OldAccount struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash // merkle root of the storage trie
	CodeHash []byte
}

func (s *StateDB) DumpOldAccount() map[common.Address][]byte {
	count := 1
	accounts := make(map[common.Address][]byte, 0)
	it := trie.NewIterator(s.trie.NodeIterator(nil))
	for it.Next() {
		if len(it.Key) == common.HashLength {
			addrBytes := s.trie.GetKey(it.Key)
			var data OldAccount
			if err := ser.DecodeBytes(it.Value, &data); err != nil {
				continue
			}
			addr := common.BytesToAddress(addrBytes)
			accounts[addr] = it.Value
			if count%10000 == 0 {
				fmt.Println("DumpAccountAddrs: current count is ", count)
			}
		} else {
			continue
		}
		count++
	}
	return accounts
}

func (s *StateDB) DumpAccounts() map[common.Address][]byte {
	count := 1
	accounts := make(map[common.Address][]byte, 0)
	it := trie.NewIterator(s.trie.NodeIterator(nil))
	for it.Next() {
		if len(it.Key) == common.HashLength {
			addrBytes := s.trie.GetKey(it.Key)
			var data Account
			if err := ser.DecodeBytes(it.Value, &data); err != nil {
				continue
			}
			addr := common.BytesToAddress(addrBytes)
			accounts[addr] = it.Value
			if count%10000 == 0 {
				fmt.Println("DumpAccounts: current count is ", count)
			}
		} else {
			continue
		}
		count++
	}
	return accounts
}

func (s *StateDB) DumpStorage() map[string]string {
	storages := make(map[string]string, 0)
	storageIt := trie.NewIterator(s.trie.NodeIterator(nil))
	for storageIt.Next() {
		storages[common.Bytes2Hex(storageIt.Key)] = common.Bytes2Hex(storageIt.Value)
	}
	return storages
}
