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
	"bytes"
	"fmt"
	"io"
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
)

var emptyCodeHash = crypto.Keccak256(nil)

type Code []byte

func (c Code) String() string {
	return string(c) //strings.Join(Disassemble(self), " ")
}

type Storage map[common.Hash][]byte

func (s Storage) String() (str string) {
	for key, value := range s {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}

	return
}

func (s Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range s {
		cpy[key] = value
	}

	return cpy
}

// stateObject represents an Ethereum account which is being modified.
//
// The usage pattern is as follows:
// First you need to obtain a state object.
// Account values can be accessed and modified through the object.
// Finally, call CommitTrie to write the modified storage trie into a database.
type stateObject struct {
	address  common.Address
	addrHash common.Hash // hash of ethereum address of the account
	data     Account
	db       *StateDB

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// Write caches.
	trie Trie // storage trie, which becomes non-nil on first access
	code Code // contract bytecode, which gets set when code is loaded

	originStorage Storage // Storage cache of original entries to dedup rewrites
	dirtyStorage  Storage // Storage entries that need to be flushed to disk

	// Cache flags.
	// When an object is marked suicided it will be delete from the trie
	// during the "update" phase of the state transition.
	dirtyCode bool // true if the code was updated
	suicided  bool
	deleted   bool
}

// empty returns whether the account is considered empty.
func (c *stateObject) empty() bool {
	return c.data.Nonce == 0 && c.data.Balance.Sign() == 0 && bytes.Equal(c.data.CodeHash, emptyCodeHash)
}

// Account is the Ethereum consensus representation of accounts.
// These objects are stored in the main account trie.
type Account struct {
	Nonce    uint64
	Credits  uint64
	Balance  *big.Int
	Tokens   map[common.Address]*big.Int
	Root     common.Hash // merkle(or kv) root of the storage trie
	CodeHash []byte
}

// newObject creates a state object.
func newObject(db *StateDB, address common.Address, data Account) *stateObject {
	if data.Balance == nil {
		data.Balance = new(big.Int)
	}
	if data.Tokens == nil {
		data.Tokens = make(map[common.Address]*big.Int)
	}
	if data.CodeHash == nil {
		data.CodeHash = emptyCodeHash
	}
	return &stateObject{
		db:            db,
		address:       address,
		addrHash:      crypto.Keccak256Hash(address[:]),
		data:          data,
		originStorage: make(Storage),
		dirtyStorage:  make(Storage),
	}
}

// EncodeSER implements ser.Encoder.
func (c *stateObject) EncodeSER(w io.Writer) error {
	return ser.Encode(w, c.data)
}

// setError remembers the first non-nil error it is called with.
func (c *stateObject) setError(err error) {
	if c.dbErr == nil {
		c.dbErr = err
	}
}

func (c *stateObject) markSuicided() {
	c.suicided = true
}

func (c *stateObject) touch() {
	c.db.journal.append(touchChange{
		account: &c.address,
	})
	if c.address == ripemd {
		// Explicitly put it in the dirty-cache, which is otherwise generated from
		// flattened journals.
		c.db.journal.dirty(c.address)
	}
}

func (c *stateObject) getTrie(db Database) Trie {
	if c.trie == nil {
		var err error
		c.trie, err = db.OpenStorageTrie(c.addrHash, c.data.Root)
		if err != nil {
			c.trie, _ = db.OpenStorageTrie(c.addrHash, common.EmptyHash)
			c.setError(fmt.Errorf("can't create storage trie: %v", err))
		}
	}
	return c.trie
}

// GetState returns a value in account storage.
func (c *stateObject) GetState(db Database, key common.Hash) []byte {
	value, dirty := c.dirtyStorage[key]
	if dirty {
		// If we have a dirty value for this state entry, return it
		return value
	}

	// Otherwise return the entry's original value
	return c.GetCommittedState(db, key)
}

// GetCommittedState retrieves a value from the committed account storage trie.
func (c *stateObject) GetCommittedState(db Database, key common.Hash) []byte {
	// If we have the original value cached, return that
	value, cached := c.originStorage[key]
	if cached {
		return value
	}
	// Otherwise load the value from the database
	enc, err := c.getTrie(db).TryGet(key[:])
	if err != nil {
		c.setError(err)
		return nil
	}
	if len(enc) > 0 {
		_, content, _, err := ser.Split(enc)
		if err != nil {
			c.setError(err)
		}
		value = content
	}
	c.originStorage[key] = value
	return value
}

// SetState updates a value in account storage.
func (c *stateObject) SetState(db Database, key common.Hash, value []byte) {
	// If the new value is the same as old, don't set
	prev := c.GetState(db, key)
	if bytes.Equal(prev, value) {
		return
	}
	// New value is different, update and journal the change
	c.db.journal.append(storageChange{
		account:  &c.address,
		key:      key,
		prevalue: prev,
	})
	c.setState(key, value)
}

func (c *stateObject) setState(key common.Hash, value []byte) {
	c.dirtyStorage[key] = value
}

func (c *stateObject) setStorageRoot(root common.Hash) {
	c.data.Root = root
}

// updateTrie writes cached storage modifications into the object's storage trie.
func (c *stateObject) updateTrie(db Database) Trie {
	tr := c.getTrie(db)
	for key, value := range c.dirtyStorage {
		delete(c.dirtyStorage, key)

		// Skip noop changes, persist actual changes
		if bytes.Equal(value, c.originStorage[key]) {
			continue
		}
		c.originStorage[key] = value

		if len(value) == 0 {
			c.setError(tr.TryDelete(key[:]))
			continue
		}
		// Encoding []byte cannot fail, ok to ignore the error.
		v, _ := ser.EncodeToBytes(bytes.TrimLeft(value[:], "\x00"))
		c.setError(tr.TryUpdate(key[:], v))
	}
	return tr
}

// UpdateRoot sets the trie root to the current root hash of
func (c *stateObject) updateRoot(db Database) {
	c.updateTrie(db)
	c.data.Root = c.trie.Hash()
}

// CommitTrie the storage trie of the object to db.
// This updates the trie root.
func (c *stateObject) CommitTrie(db Database, height uint64) error {
	c.updateTrie(db)
	if c.dbErr != nil {
		return c.dbErr
	}
	root, err := c.trie.Commit(nil, height)
	if err == nil {
		c.data.Root = root
	}
	return err
}

// AddBalance removes amount from c's balance.
// It is used to add funds to the destination account of a transfer.
func (c *stateObject) AddBalance(amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Sign() == 0 {
		if c.empty() {
			c.touch()
		}

		return
	}
	c.SetBalance(new(big.Int).Add(c.Balance(), amount))
}

// SubBalance removes amount from c's balance.
// It is used to remove funds from the origin account of a transfer.
func (c *stateObject) SubBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	c.SetBalance(new(big.Int).Sub(c.Balance(), amount))
}

func (c *stateObject) SetBalance(amount *big.Int) {
	c.SetCredits(c.Credits() + 1)
	c.db.journal.append(balanceChange{
		account: &c.address,
		prev:    new(big.Int).Set(c.data.Balance),
	})
	c.setBalance(amount)
}

func (c *stateObject) setBalance(amount *big.Int) {
	c.data.Balance = amount
}

func (c *stateObject) AddTokenBalance(token common.Address, amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Sign() == 0 {
		if c.empty() {
			c.touch()
		}

		return
	}
	c.SetTokenBalance(token, new(big.Int).Add(c.TokenBalance(token), amount))
}

func (c *stateObject) SubTokenBalance(token common.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	c.SetTokenBalance(token, new(big.Int).Sub(c.TokenBalance(token), amount))
}

func (c *stateObject) SetTokenBalance(token common.Address, amount *big.Int) {
	if token == common.EmptyAddress {
		c.SetBalance(amount)
		return
	}
	if _, ok := c.data.Tokens[token]; !ok {
		c.data.Tokens[token] = common.Big0
	}
	c.SetCredits(c.Credits() + 1)
	c.db.journal.append(tokenBalanceChange{
		account: &c.address,
		token:   &token,
		prev:    new(big.Int).Set(c.data.Tokens[token]),
	})
	c.setTokenBalance(token, amount)
}

func (c *stateObject) setTokenBalance(token common.Address, amount *big.Int) {
	c.data.Tokens[token] = amount
}

// Return the gas back to the origin. Used by the Virtual machine or Closures
func (c *stateObject) ReturnGas(gas *big.Int) {}

func (c *stateObject) deepCopy(db *StateDB) *stateObject {
	stateObject := newObject(db, c.address, c.data)
	if c.trie != nil {
		stateObject.trie = db.db.CopyTrie(c.trie)
	}

	stateObject.code = c.code
	stateObject.dirtyStorage = c.dirtyStorage.Copy()
	stateObject.originStorage = c.originStorage.Copy()
	stateObject.suicided = c.suicided
	stateObject.dirtyCode = c.dirtyCode
	stateObject.deleted = c.deleted
	return stateObject
}

//
// Attribute accessors
//

// Returns the address of the contract/account
func (c *stateObject) Address() common.Address {
	return c.address
}

// Code returns the contract code associated with this object, if any.
func (c *stateObject) Code(db Database) []byte {
	if c.code != nil {
		return c.code
	}
	if bytes.Equal(c.CodeHash(), emptyCodeHash) {
		return nil
	}
	code, err := db.ContractCode(c.addrHash, common.BytesToHash(c.CodeHash()))
	if err != nil {
		c.setError(fmt.Errorf("can't load code hash %x: %v", c.CodeHash(), err))
	}
	c.code = code
	return code
}

func (c *stateObject) SetCode(codeHash common.Hash, code []byte) {
	prevcode := c.Code(c.db.db)
	c.db.journal.append(codeChange{
		account:  &c.address,
		prevhash: c.CodeHash(),
		prevcode: prevcode,
	})
	c.setCode(codeHash, code)
}

func (c *stateObject) setCode(codeHash common.Hash, code []byte) {
	c.code = code
	c.data.CodeHash = codeHash[:]
	c.dirtyCode = true
}

func (c *stateObject) SetNonce(nonce uint64) {
	c.db.journal.append(nonceChange{
		account: &c.address,
		prev:    c.data.Nonce,
	})
	c.setNonce(nonce)
}

func (c *stateObject) setNonce(nonce uint64) {
	c.data.Nonce = nonce
}

func (c *stateObject) SetCredits(credits uint64) {
	c.db.journal.append(creditsChange{
		account: &c.address,
		prev:    c.data.Credits,
	})
	c.setCredits(credits)
}

func (c *stateObject) setCredits(credits uint64) {
	c.data.Credits = credits
}

func (c *stateObject) CodeHash() []byte {
	return c.data.CodeHash
}

func (c *stateObject) Balance() *big.Int {
	return c.data.Balance
}

func (c *stateObject) TokenBalance(token common.Address) *big.Int {
	if token == common.EmptyAddress {
		return c.data.Balance
	}
	if balance, ok := c.data.Tokens[token]; ok {
		return balance
	}
	return common.Big0
}

func (c *stateObject) TokenBalances() types.TokenValues {
	tv := make(types.TokenValues, 0, len(c.data.Tokens)+1)
	if c.data.Balance.Sign() > 0 {
		tv = append(tv, types.TokenValue{
			TokenAddr: common.EmptyAddress,
			Value:     big.NewInt(0).Set(c.data.Balance),
		})
	}
	for addr, val := range c.data.Tokens {
		if val.Sign() > 0 {
			tv = append(tv, types.TokenValue{
				TokenAddr: addr,
				Value:     big.NewInt(0).Set(val),
			})
		}
	}
	return tv
}

func (c *stateObject) Nonce() uint64 {
	return c.data.Nonce
}

func (c *stateObject) Credits() uint64 {
	return c.data.Credits
}

func (c *stateObject) StorageRoot() common.Hash {
	return c.data.Root
}

// Never called, but must be present to allow stateObject to be used
// as a vm.Account interface that also satisfies the vm.ContractRef
// interface. Interfaces are awesome.
func (c *stateObject) Value() *big.Int {
	panic("Value on stateObject should never be called")
}
