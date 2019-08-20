package db

import (
	"fmt"
	"sort"
	"sync"
)

func init() {
	registerDBCreator(MemDBBackend, func(name string, dir string, dbCounts uint64) (DB, error) {
		return NewMemDB(), nil
	}, false)
}

var _ DB = (*MemDB)(nil)

type MemDB struct {
	mtx sync.Mutex
	db  map[string][]byte
}

func NewMemDB() *MemDB {
	database := &MemDB{
		db: make(map[string][]byte),
	}
	return database
}

func (db *MemDB) Dir() string {
	return string("")
}

func (db *MemDB) Len() int {
	return len(db.db)
}

func (db *MemDB) Keys() [][]byte {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	keys := [][]byte{}
	for key := range db.db {
		keys = append(keys, []byte(key))
	}
	return keys
}

// Implements atomicSetDeleter.
func (db *MemDB) Mutex() *sync.Mutex {
	return &(db.mtx)
}

// Implements DB.
func (db *MemDB) Get(key []byte) []byte {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = nonNilBytes(key)

	value := db.db[string(key)]
	return value
}
func (db *MemDB) Load(key []byte) ([]byte, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	value := db.db[string(key)]
	return value, nil
}

// Implements DB.
func (db *MemDB) Has(key []byte) bool {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = nonNilBytes(key)

	_, ok := db.db[string(key)]
	return ok
}
func (db *MemDB) Exist(key []byte) (bool, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	_, ok := db.db[string(key)]
	return ok, nil
}

// Implements DB.
func (db *MemDB) Set(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}
func (db *MemDB) Put(key []byte, value []byte) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
	return nil
}

// Implements DB.
func (db *MemDB) SetSync(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

// Implements atomicSetDeleter.
func (db *MemDB) SetNoLock(key []byte, value []byte) {
	db.SetNoLockSync(key, value)
}

// Implements atomicSetDeleter.
func (db *MemDB) SetNoLockSync(key []byte, value []byte) {
	key = nonNilBytes(key)
	value = nonNilBytes(value)

	db.db[string(key)] = value
}

// Implements DB.
func (db *MemDB) Delete(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}
func (db *MemDB) Del(key []byte) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
	return nil
}

// Implements DB.
func (db *MemDB) DeleteSync(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}

// Implements atomicSetDeleter.
func (db *MemDB) DeleteNoLock(key []byte) {
	db.DeleteNoLockSync(key)
}

// Implements atomicSetDeleter.
func (db *MemDB) DeleteNoLockSync(key []byte) {
	key = nonNilBytes(key)

	delete(db.db, string(key))
}

// Implements DB.
func (db *MemDB) Close() {
	// Close is a noop since for an in-memory
	// database, we don't have a destination
	// to flush contents to nor do we want
	// any data loss on invoking Close()
}

// Implements DB.
func (db *MemDB) Print() {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	for key, value := range db.db {
		fmt.Printf("[%X]:\t[%X]\n", []byte(key), value)
	}
}

// Implements DB.
func (db *MemDB) Stats() map[string]string {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	stats := make(map[string]string)
	stats["database.type"] = "memDB"
	stats["database.size"] = fmt.Sprintf("%d", len(db.db))
	return stats
}

// Implements DB.
func (db *MemDB) NewBatch() Batch {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return &memBatch{db, nil, 0}
}

//----------------------------------------
// Iterator

// Implements DB.
func (db *MemDB) Iterator(start, end []byte) Iterator {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	keys := db.getSortedKeys(start, end, false)
	return newMemDBIterator(db, keys, start, end, false)
}

func (db *MemDB) NewIteratorWithPrefix(prefix []byte) Iterator {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	end := PrefixToEnd(prefix)
	keys := db.getSortedKeys(prefix, end, false)
	return newMemDBIterator(db, keys, prefix, end, false)
}

// Implements DB.
func (db *MemDB) ReverseIterator(start, end []byte) Iterator {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	keys := db.getSortedKeys(start, end, true)
	return newMemDBIterator(db, keys, start, end, true)
}

// We need a copy of all of the keys.
// Not the best, but probably not a bottleneck depending.
type memDBIterator struct {
	db        *MemDB
	cur       int
	keys      []string
	start     []byte
	end       []byte
	isReverse bool
}

var _ Iterator = (*memDBIterator)(nil)

// Keys is expected to be in reverse order for reverse iterators.
func newMemDBIterator(db *MemDB, keys []string, start, end []byte, isReverse bool) *memDBIterator {
	return &memDBIterator{
		db:        db,
		cur:       0,
		keys:      keys,
		start:     start,
		end:       end,
		isReverse: isReverse,
	}
}

// Implements Iterator.
func (itr *memDBIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

// Implements Iterator.
func (itr *memDBIterator) Valid() bool {
	return 0 <= itr.cur && itr.cur < len(itr.keys)
}

// Implements Iterator.
func (itr *memDBIterator) Next() (valid bool) {
	//itr.assertIsValid()
	if valid = itr.Valid(); !valid {
		return
	}
	itr.cur++
	return
}

// Implements Iterator.
func (itr *memDBIterator) Key() []byte {
	itr.assertIsValid()
	return []byte(itr.keys[itr.cur])
}

// Implements Iterator.
func (itr *memDBIterator) Value() []byte {
	itr.assertIsValid()
	key := []byte(itr.keys[itr.cur])
	return itr.db.Get(key)
}

func (itr *memDBIterator) Seek(key []byte) bool {
	itr.db.mtx.Lock()
	defer itr.db.mtx.Unlock()

	keys := itr.db.getSortedKeys(key, itr.end, itr.isReverse)
	itr.keys = keys
	itr.cur = 0
	itr.start = key
	return itr.Valid()
}

// Implements Iterator.
func (itr *memDBIterator) Close() {
	itr.keys = nil
	itr.db = nil
}

func (itr *memDBIterator) assertIsValid() {
	if !itr.Valid() {
		panic("memDBIterator is invalid")
	}
}

//----------------------------------------
// Misc.

func (db *MemDB) getSortedKeys(start, end []byte, reverse bool) []string {
	keys := []string{}
	for key := range db.db {
		inDomain := IsKeyInDomain([]byte(key), start, end, reverse)
		if inDomain {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if reverse {
		nkeys := len(keys)
		for i := 0; i < nkeys/2; i++ {
			temp := keys[i]
			keys[i] = keys[nkeys-i-1]
			keys[nkeys-i-1] = temp
		}
	}
	return keys
}
