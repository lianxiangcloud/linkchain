package db

import (
	"sync"
	"bytes"
	"time"
	"path/filepath"

	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/badger/options"
	"github.com/golang/snappy"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
)

func init() {
	dbCreator := func(name string, dir string, dbCounts uint64) (DB, error) {
		return NewBadgerDB(name, dir, dbCounts)
	}
	registerDBCreator(BadgerBackend, dbCreator, false)
}

var _ DB = (*BadgerDB)(nil)

type BadgerDB struct {
	dbPaths  []string
	dbs      []*badger.DB
	dbCounts uint64
}

func (db *BadgerDB) badgerGc() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		for index := uint64(0); index < db.dbCounts; index++ {
		again:
			err := db.dbs[index].RunValueLogGC(0.01)
			if err == nil {
				goto again
			}
		}
	}
}

func NewBadgerDB(name string, dir string, counts uint64) (*BadgerDB, error) {
	dbCounts := dbCountsPreCheck(counts)
	dbs      := make([]*badger.DB, dbCounts)
	dbPaths  := make([]string, dbCounts)

	for index := uint64(0); index < dbCounts; index++ {
		dbName := genDbName(name, index)
		dbPath := filepath.Join(dir, dbName+".db")
		badgerOpts := badger.DefaultOptions(dbPath)
		badgerOpts.Dir = dbPath
		badgerOpts.ValueDir = dbPath
		badgerOpts.ValueThreshold = 32
		badgerOpts.MaxTableSize = 16 << 20
		badgerOpts.TableLoadingMode = options.MemoryMap
		badgerOpts.ValueLogLoadingMode = options.FileIO
		db, err := badger.Open(badgerOpts)
		if err != nil {
			return nil, err
		}

		dbs[index]     = db
		dbPaths[index] = dbPath
	}

	database := &BadgerDB{
		dbPaths:  dbPaths,
		dbs:      dbs,
		dbCounts: dbCounts,
	}
	go database.badgerGc()

	return database, nil
}

// Implements DB.
func (db *BadgerDB) Get(key []byte) []byte {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)
	var valCopy []byte

	err := db.dbs[index].View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		err = item.Value(func(val []byte) error {
			snappyVal, err := snappy.Decode(nil, val)
			if err != nil {
				return err
			}
			valCopy = append([]byte{}, snappyVal...)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil && err != badger.ErrKeyNotFound {
		panic(err)
	}

	return valCopy
}

func (db *BadgerDB) Load(key []byte) ([]byte, error) {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)
	var valCopy []byte

	err := db.dbs[index].View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		err = item.Value(func(val []byte) error {
			snappyVal, err := snappy.Decode(nil, val)
			if err != nil {
				return err
			}
			valCopy = append([]byte{}, snappyVal...)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})

	return valCopy, err
}

// Implements DB.
func (db *BadgerDB) Has(key []byte) bool {
	return db.Get(key) != nil
}

func (db *BadgerDB) Exist(key []byte) (bool, error) {
	v, err := db.Load(key)
	return v != nil, err
}

// Implements DB.
func (db *BadgerDB) Set(key []byte, val []byte) {
	key = nonNilBytes(key)
	val = nonNilBytes(val)
	index := dbIndex(key, db.dbCounts)

	err := db.dbs[index].Update(func(txn *badger.Txn) error {
		snappyVal := snappy.Encode(nil, val)
		err := txn.Set(key, snappyVal)
		return err
	})
	if err != nil {
		if err != badger.ErrEmptyKey {
			cmn.PanicCrisis(err)
		}
	}
}

func (db *BadgerDB) Put(key []byte, val []byte) error {
	key = nonNilBytes(key)
	val = nonNilBytes(val)
	index := dbIndex(key, db.dbCounts)

	err := db.dbs[index].Update(func(txn *badger.Txn) error {
		snappyVal := snappy.Encode(nil, val)
		err := txn.Set(key, snappyVal)
		return err
	})
	if err != nil {
		if err != badger.ErrEmptyKey {
			return err
		}
	}
	return nil
}

// Implements DB.
func (db *BadgerDB) SetSync(key []byte, val []byte) {
	db.Set(key, val)
}

// Implements DB.
func (db *BadgerDB) Delete(key []byte) {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)

	err := db.dbs[index].Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
	if err != nil {
		cmn.PanicCrisis(err)
	}
}

func (db *BadgerDB) Del(key []byte) error {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)

	err := db.dbs[index].Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
	return err
}

// Implements DB.
func (db *BadgerDB) DeleteSync(key []byte) {
	db.Delete(key)
}

// Implements DB.
func (db *BadgerDB) Close() {
	for index := uint64(0); index < db.dbCounts; index++ {
		db.dbs[index].Close()
	}
}

// Implements DB.
func (db *BadgerDB) Print() {
}

// Implements DB.
func (db *BadgerDB) Stats() map[string]string {
	return make(map[string]string, 0)
}

//----------------------------------------
// Batch

// Implements DB.
func (db *BadgerDB) NewBatch() Batch {
	batchs := make([]*badger.WriteBatch, db.dbCounts)
	for index := uint64(0); index < db.dbCounts; index++ {
		batchs[index] = db.dbs[index].NewWriteBatch()
	}
	return &badgerBatch{
		db,
		batchs,
		0,
	}
}

type badgerBatch struct {
	db     *BadgerDB
	batchs []*badger.WriteBatch
	size   int
}

// Implements Batch.
func (mBatch *badgerBatch) Set(key, val []byte) {
	key = nonNilBytes(key)
	val = nonNilBytes(val)
	index := dbIndex(key, mBatch.db.dbCounts)
	snappyVal := snappy.Encode(nil, val)
	mBatch.batchs[index].Set(key, snappyVal)
	mBatch.size++
}

// Implements Batch.
func (mBatch *badgerBatch) Delete(key []byte) {
	key = nonNilBytes(key)
	index := dbIndex(key, mBatch.db.dbCounts)
	mBatch.batchs[index].Delete(key)
}

// Implements Batch.
func (mBatch *badgerBatch) Write() {
	sw := sync.WaitGroup{}
	for index := uint64(0); index < mBatch.db.dbCounts; index++ {
		sw.Add(1)
		go func(index uint64) {
			defer sw.Done()
			err := mBatch.batchs[index].Flush()
			if err != nil {
				panic(err)
			}
		}(index)
	}
	sw.Wait()
}

func (mBatch *badgerBatch) Commit() error {
	var retErr error
	sw := sync.WaitGroup{}
	for index := uint64(0); index < mBatch.db.dbCounts; index++ {
		sw.Add(1)
		go func(index uint64) {
			defer sw.Done()
			err := mBatch.batchs[index].Flush()
			if err != nil {
				retErr = err
			}
		}(index)
	}
	sw.Wait()
	return retErr
}

// Implements Batch.
func (mBatch *badgerBatch) WriteSync() {
	mBatch.Write()
}

func (mBatch *badgerBatch) ValueSize() int {
	return mBatch.size
}

func (mBatch *badgerBatch) Reset() {
	for index := uint64(0); index < mBatch.db.dbCounts; index++ {
		mBatch.batchs[index].Cancel()
	}
	mBatch.size = 0
}

//----------------------------------------
// Iterator
// NOTE This is almost identical to db/c_level_db.Iterator
// Before creating a third version, refactor.

// Implements DB.
func (db *BadgerDB) Iterator(start, end []byte) Iterator {
	itrs := make([]badger.Iterator, db.dbCounts)
	for index := uint64(0); index < db.dbCounts; index++ {
		txn := db.dbs[index].NewTransaction(true)
		itr := txn.NewIterator(badger.DefaultIteratorOptions)
		itrs[index] = *itr
	}
	return newBadgerIterator(itrs, start, end, false, db.dbCounts)
}

// Implements DB.
func (db *BadgerDB) ReverseIterator(start, end []byte) Iterator {
	itrs := make([]badger.Iterator, db.dbCounts)
	for index := uint64(0); index < db.dbCounts; index++ {
		txn := db.dbs[index].NewTransaction(true)
		iterOption := badger.DefaultIteratorOptions
		iterOption.Reverse = true
		itr := txn.NewIterator(iterOption)
		itrs[index] = *itr
	}
	return newBadgerIterator(itrs, start, end, true, db.dbCounts)
}

func (db *BadgerDB) NewIteratorWithPrefix(prefix []byte) Iterator {
	itrs := make([]badger.Iterator, db.dbCounts)
	for index := uint64(0); index < db.dbCounts; index++ {
		txn := db.dbs[index].NewTransaction(true)
		itr := txn.NewIterator(badger.DefaultIteratorOptions)
		itrs[index] = *itr
	}
	return newBadgerIterator(itrs, prefix, PrefixToEnd(prefix), false, db.dbCounts)
}

func (db *BadgerDB) Dir() string {
	return filepath.Dir(db.dbPaths[0])
}

type badgerIterator struct {
	sources   []badger.Iterator
	lastIndex uint64
	dbIndex   uint64
	dbCounts  uint64
	start     []byte
	end       []byte
	isReverse bool
	isInvalid bool
}

var _ Iterator = (*goLevelDBIterator)(nil)

func newBadgerIterator(sources []badger.Iterator, start, end []byte, isReverse bool, dbCounts uint64) *badgerIterator {
	for index := uint64(0); index < dbCounts; index++ {
		if start == nil {
			sources[index].Rewind()
		} else {
			sources[index].Seek(start)
		}
	}

	return &badgerIterator{
		sources:   sources,
		start:     start,
		end:       end,
		isReverse: isReverse,
		isInvalid: false,
		lastIndex: 0,
		dbIndex:   0,
		dbCounts:  dbCounts,
	}
}

// Implements Iterator.
func (itr *badgerIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

// Implements Iterator.
func (itr *badgerIterator) Valid() bool {

	// Once invalid, forever invalid.
	if itr.isInvalid {
		return false
	}

	// If source is invalid, invalid.
	if !itr.sources[itr.dbIndex].Valid() {
		if itr.dbIndex + 1 == itr.dbCounts {
			itr.isInvalid = true
			return false
		}
		itr.dbIndex++
		return itr.Valid()
	}

	// If key is end or past it, invalid.
	var end = itr.end
	var key = itr.sources[itr.dbIndex].Item().Key()

	if itr.isReverse {
		if end != nil && bytes.Compare(key, end) <= 0 {
			if itr.dbIndex + 1 == itr.dbCounts {
				itr.isInvalid = true
				return false
			}
			itr.dbIndex++
			return itr.Valid()
		}
	} else {
		if end != nil && bytes.Compare(end, key) <= 0 {
			if itr.dbIndex + 1 == itr.dbCounts {
				itr.isInvalid = true
				return false
			}
			itr.dbIndex++
			return itr.Valid()
		}
	}

	// Valid
	return true
}

func (itr *badgerIterator) Seek(key []byte) bool {
	itr.isInvalid = false
	for index := uint64(0); index < itr.dbCounts; index++ {
		if key == nil {
			itr.sources[index].Rewind()
		} else {
			itr.sources[index].Seek(key)
		}
	}
	itr.start = key
	itr.dbIndex = 0
	return itr.Valid()
}

// Implements Iterator.
func (itr *badgerIterator) Key() []byte {
	itr.assertIsValid()
	return cp(itr.sources[itr.dbIndex].Item().Key())
}

// Implements Iterator.
func (itr *badgerIterator) Value() []byte {
	itr.assertIsValid()

	var valCopy []byte
	itr.sources[itr.dbIndex].Item().Value(func(val []byte) error {
		snappyVal, err := snappy.Decode(nil, val)
		if err != nil {
			return err
		}
		valCopy = append([]byte{}, snappyVal...)
		return nil
	})

	return cp(valCopy)
}

// Implements Iterator.
func (itr *badgerIterator) Next() bool {
	itr.assertIsValid()

	tmpDbIndex := itr.dbIndex
	if itr.lastIndex == itr.dbIndex {
		itr.sources[itr.dbIndex].Next()
	}
	itr.lastIndex = itr.dbIndex
	if !itr.Valid() {
		return false
	}
	if tmpDbIndex != itr.dbIndex {
		return itr.Next()
	}
	return true
}

// Implements Iterator.
func (itr *badgerIterator) Close() {
	for index := uint64(0); index < itr.dbCounts; index++ {
		itr.sources[index].Close()
	}
	itr.isInvalid = true
}


func (itr badgerIterator) assertIsValid() {
	if !itr.Valid() {
		panic("badgerIterator is invalid")
	}
}
