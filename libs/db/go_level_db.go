package db

import (
	"sync"
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
)

func init() {
	dbCreator := func(name string, dir string, counts uint64) (DB, error) {
		return NewGoLevelDB(name, dir, counts)
	}
	registerDBCreator(LevelDBBackend, dbCreator, false)
	registerDBCreator(GoLevelDBBackend, dbCreator, false)
}

var _ DB = (*GoLevelDB)(nil)

type GoLevelDB struct {
	dbPaths  []string
	dbs      []*leveldb.DB
	dbCounts uint64
}

func NewGoLevelDB(name string, dir string, counts uint64) (*GoLevelDB, error) {
	dbCounts := dbCountsPreCheck(counts)
	dbs      := make([]*leveldb.DB, dbCounts)
	dbPaths  := make([]string, dbCounts)

	for index := uint64(0); index < dbCounts; index++ {
		dbName := genDbName(name, index)
		dbPath := filepath.Join(dir, dbName+".db")
		db, err := leveldb.OpenFile(dbPath, nil)
		if err != nil {
			es := fmt.Sprintf("OpenFile:%s", err)
			db, err = leveldb.RecoverFile(dbPath, nil)
			if err != nil {
				return nil, fmt.Errorf("%s. RecoverFile:%s", es, err)
			}
		}
		dbs[index]     = db
		dbPaths[index] = dbPath
	}

	database := &GoLevelDB{
		dbPaths:  dbPaths,
		dbs:      dbs,
		dbCounts: dbCounts,
	}
	return database, nil
}

func (db *GoLevelDB) Dir() string {
	return filepath.Dir(db.dbPaths[0])
}

// Implements DB.
func (db *GoLevelDB) Get(key []byte) []byte {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)
	res, err := db.dbs[index].Get(key, nil)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil
		}
		panic(err)
	}
	return res
}
func (db *GoLevelDB) Load(key []byte) ([]byte, error) {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)
	return db.dbs[index].Get(key, nil)
}

// Implements DB.
func (db *GoLevelDB) Has(key []byte) bool {
	return db.Get(key) != nil
}
func (db *GoLevelDB) Exist(key []byte) (bool, error) {
	v, err := db.Load(key)
	return v != nil, err
}

// Implements DB.
func (db *GoLevelDB) Set(key []byte, value []byte) {
	key = nonNilBytes(key)
	value = nonNilBytes(value)
	index := dbIndex(key, db.dbCounts)
	err := db.dbs[index].Put(key, value, nil)
	if err != nil {
		cmn.PanicCrisis(err)
	}
}
func (db *GoLevelDB) Put(key []byte, value []byte) error {
	key = nonNilBytes(key)
	value = nonNilBytes(value)
	index := dbIndex(key, db.dbCounts)
	return db.dbs[index].Put(key, value, nil)
}

// Implements DB.
func (db *GoLevelDB) SetSync(key []byte, value []byte) {
	key = nonNilBytes(key)
	value = nonNilBytes(value)
	index := dbIndex(key, db.dbCounts)
	err := db.dbs[index].Put(key, value, &opt.WriteOptions{Sync: true})
	if err != nil {
		cmn.PanicCrisis(err)
	}
}

// Implements DB.
func (db *GoLevelDB) Delete(key []byte) {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)
	err := db.dbs[index].Delete(key, nil)
	if err != nil {
		cmn.PanicCrisis(err)
	}
}
func (db *GoLevelDB) Del(key []byte) error {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)
	return db.dbs[index].Delete(key, nil)
}

// Implements DB.
func (db *GoLevelDB) DeleteSync(key []byte) {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)
	err := db.dbs[index].Delete(key, &opt.WriteOptions{Sync: true})
	if err != nil {
		cmn.PanicCrisis(err)
	}
}

// Implements DB.
func (db *GoLevelDB) Close() {
	for index := uint64(0); index < db.dbCounts; index++ {
		db.dbs[index].Close()
	}
}

// Implements DB.
func (db *GoLevelDB) Print() {}

// Implements DB.
func (db *GoLevelDB) Stats() map[string]string {
	keys := []string{
		"leveldb.num-files-at-level{n}",
		"leveldb.stats",
		"leveldb.sstables",
		"leveldb.blockpool",
		"leveldb.cachedblock",
		"leveldb.openedtables",
		"leveldb.alivesnaps",
		"leveldb.aliveiters",
	}

	stats := make(map[string]string)
	for index := uint64(0); index < db.dbCounts; index++ {
		for _, key := range keys {
			str, err := db.dbs[index].GetProperty(key)
			if err == nil {
				fullKey := genStatsKey(key, index)
				stats[fullKey] = str
			}
		}
	}

	return stats
}

//----------------------------------------
// Batch

// Implements DB.
func (db *GoLevelDB) NewBatch() Batch {
	batchs := make([]*leveldb.Batch, db.dbCounts)
	for index := uint64(0); index < db.dbCounts; index++ {
		batch := new(leveldb.Batch)
		batchs[index] = batch
	}
	return &goLevelDBBatch{db, batchs, db.dbCounts, 0}
}

type goLevelDBBatch struct {
	db       *GoLevelDB
	batchs   []*leveldb.Batch
	dbCounts uint64
	size     int
}

// Implements Batch.
func (mBatch *goLevelDBBatch) Set(key, value []byte) {
	key   = nonNilBytes(key)
	value = nonNilBytes(value)
	index := dbIndex(key, mBatch.dbCounts)
	mBatch.batchs[index].Put(key, value)
}

// Implements Batch.
func (mBatch *goLevelDBBatch) Delete(key []byte) {
	key = nonNilBytes(key)
	index := dbIndex(key, mBatch.dbCounts)
	mBatch.batchs[index].Delete(key)
}

// Implements Batch.
func (mBatch *goLevelDBBatch) Write() {
	sw := sync.WaitGroup{}
	for index := uint64(0); index < mBatch.dbCounts; index++ {
		sw.Add(1)
		go func(index uint64) {
			defer sw.Done()
			err := mBatch.db.dbs[index].Write(mBatch.batchs[index], &opt.WriteOptions{Sync: false})
			if err != nil {
				panic(err)
			}
		}(index)
	}
	sw.Wait()
}
func (mBatch *goLevelDBBatch) Commit() error {
	var retErr error
	sw := sync.WaitGroup{}
	for index := uint64(0); index < mBatch.dbCounts; index++ {
		sw.Add(1)
		go func(index uint64) {
			defer sw.Done()
			err := mBatch.db.dbs[index].Write(mBatch.batchs[index], &opt.WriteOptions{Sync: false})
			if err != nil {
				retErr = err
			}
		}(index)
	}
	sw.Wait()
	return retErr
}

// Implements Batch.
func (mBatch *goLevelDBBatch) WriteSync() {
	sw := sync.WaitGroup{}
	for index := uint64(0); index < mBatch.dbCounts; index++ {
		sw.Add(1)
		go func(index uint64) {
			defer sw.Done()
			err := mBatch.db.dbs[index].Write(mBatch.batchs[index], &opt.WriteOptions{Sync: true})
			if err != nil {
				panic(err)
			}
		}(index)
	}
	sw.Wait()
}

func (mBatch *goLevelDBBatch) ValueSize() int {
	return mBatch.size
}

func (mBatch *goLevelDBBatch) Reset() {
	for index := uint64(0); index < mBatch.dbCounts; index++ {
		mBatch.batchs[index].Reset()
	}
	mBatch.size = 0
}

//----------------------------------------
// Iterator
// NOTE This is almost identical to db/c_level_db.Iterator
// Before creating a third version, refactor.

// Implements DB.
func (db *GoLevelDB) Iterator(start, end []byte) Iterator {
	itrs := make([]iterator.Iterator, db.dbCounts)
	for index := uint64(0); index < db.dbCounts; index++ {
		itr := db.dbs[index].NewIterator(nil, nil)
		itrs[index] = itr
	}
	return newGoLevelDBIterator(itrs, start, end, false, db.dbCounts)
}

func (db *GoLevelDB) NewIteratorWithPrefix(prefix []byte) Iterator {
	itrs := make([]iterator.Iterator, db.dbCounts)
	for index := uint64(0); index < db.dbCounts; index++ {
		itr := db.dbs[index].NewIterator(nil, nil)
		itrs[index] = itr
	}
	return newGoLevelDBIterator(itrs, prefix, PrefixToEnd(prefix), false, db.dbCounts)
}

// Implements DB.
func (db *GoLevelDB) ReverseIterator(start, end []byte) Iterator {
	itrs := make([]iterator.Iterator, db.dbCounts)
	for index := uint64(0); index < db.dbCounts; index++ {
		itr := db.dbs[index].NewIterator(nil, nil)
		itrs[index] = itr
	}
	return newGoLevelDBIterator(itrs, start, end, true, db.dbCounts)
}

type goLevelDBIterator struct {
	sources   []iterator.Iterator
	lastIndex uint64
	dbIndex   uint64
	dbCounts  uint64
	start     []byte
	end       []byte
	isReverse bool
	isInvalid bool
}

var _ Iterator = (*goLevelDBIterator)(nil)

func newGoLevelDBIterator(sources []iterator.Iterator, start, end []byte, isReverse bool, dbCounts uint64) *goLevelDBIterator {
	for index := uint64(0); index < dbCounts; index++ {
		if isReverse {
			if start == nil {
				sources[index].Last()
			} else {
				valid := sources[index].Seek(start)
				if valid {
					soakey := sources[index].Key() // start or after key
					if bytes.Compare(start, soakey) < 0 {
						sources[index].Prev()
					}
				} else {
					sources[index].Last()
				}
			}
		} else {
			if start == nil {
				sources[index].First()
			} else {
				sources[index].Seek(start)
			}
		}
	}

	return &goLevelDBIterator{
		sources:   sources,
		lastIndex: 0,
		dbIndex:   0,
		dbCounts:  dbCounts,
		start:     start,
		end:       end,
		isReverse: isReverse,
		isInvalid: false,
	}
}

// Implements Iterator.
func (itr *goLevelDBIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

// Implements Iterator.
func (itr *goLevelDBIterator) Valid() bool {

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
	var key = itr.sources[itr.dbIndex].Key()

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

// Implements Iterator.
func (itr *goLevelDBIterator) Key() []byte {
	// Key returns a copy of the current key.
	// See https://github.com/syndtr/goleveldb/blob/52c212e6c196a1404ea59592d3f1c227c9f034b2/leveldb/iterator/iter.go#L88
	itr.assertIsValid()
	return cp(itr.sources[itr.dbIndex].Key())
}

// Implements Iterator.
func (itr *goLevelDBIterator) Value() []byte {
	// Value returns a copy of the current value.
	// See https://github.com/syndtr/goleveldb/blob/52c212e6c196a1404ea59592d3f1c227c9f034b2/leveldb/iterator/iter.go#L88
	itr.assertIsValid()
	return cp(itr.sources[itr.dbIndex].Value())
}

// Implements Iterator.
func (itr *goLevelDBIterator) Next() bool {
	itr.assertIsValid()

	tmpDbIndex := itr.dbIndex
	if itr.lastIndex == itr.dbIndex {
		if itr.isReverse {
			itr.sources[itr.dbIndex].Prev()
		} else {
			itr.sources[itr.dbIndex].Next()
		}
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

func (itr *goLevelDBIterator) Seek(key []byte) bool {
	itr.isInvalid = false

	for index := uint64(0); index < itr.dbCounts; index++ {
		if itr.isReverse {
			if key == nil {
				itr.sources[index].Last()
			} else {
				valid := itr.sources[index].Seek(key)
				if valid {
					soakey := itr.sources[index].Key() // start or after key
					if bytes.Compare(key, soakey) < 0 {
						itr.sources[index].Prev()
					}
				} else {
					itr.sources[index].Last()
				}
			}
		} else {
			if key == nil {
				itr.sources[index].First()
			} else {
				itr.sources[index].Seek(key)
			}
		}
	}

	itr.start = key
	itr.dbIndex = 0
	return itr.Valid()
}

// Implements Iterator.
func (itr *goLevelDBIterator) Close() {
	for index := uint64(0); index < itr.dbCounts; index++ {
		itr.sources[index].Release()
	}
	itr.isInvalid = true
}


func (itr goLevelDBIterator) assertIsValid() {
	if !itr.Valid() {
		panic("goLevelDBIterator is invalid")
	}
}
