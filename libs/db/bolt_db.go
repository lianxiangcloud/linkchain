package db

import (
	"sync"
	"bytes"
	"path/filepath"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"github.com/golang/snappy"

	"github.com/lianxiangcloud/linkchain/libs/log"
)

const (
	defaultBucket     = "default"
	bucketFillPercent = 0.9
	boltMaxBatchSize  = 1e5
)

func init() {
	dbCreator := func(name string, dir string, counts uint64) (DB, error) {
		return NewBoltDB(name, dir, counts)
	}
	registerDBCreator(BoltBackend, dbCreator, false)
}

var _ DB = (*BoltDB)(nil)

type BoltDB struct {
	dbPaths  []string
	dbs      []*bolt.DB
	dbCounts uint64
}

func NewBoltDB(name string, dir string, count uint64) (*BoltDB, error) {
	dbCounts := dbCountsPreCheck(count)
	dbs      := make([]*bolt.DB, dbCounts)
	dbPaths  := make([]string, dbCounts)

	for index := uint64(0); index < dbCounts; index++ {
		dbName := genDbName(name, index)
		dbPath := filepath.Join(dir, dbName + ".db")
		boltOps := &bolt.Options{}
		db, err := bolt.Open(dbPath, 0600, boltOps)
		if err != nil {
			return nil, err
		}

		// Create default bucket
		err = db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte(defaultBucket))
			if err != nil {
				return err
			}
			b.FillPercent = bucketFillPercent
			return nil
		})
		if err != nil {
			return nil, err
		}

		dbs[index]     = db
		dbPaths[index] = dbPath
	}


	database := &BoltDB{
		dbPaths:  dbPaths,
		dbs:      dbs,
		dbCounts: dbCounts,
	}
	return database, nil
}

// Implements DB.
func (db *BoltDB) Get(key []byte) []byte {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)
	var valCopy []byte

	err := db.dbs[index].View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(defaultBucket))
		if b == nil {
			return errors.New("default bucket not exist")
		}
		val := b.Get(key)
		if len(val) == 0 {
			log.Error("BoltDB Get failed. KeyNotFound.", "key", string(key))
			return nil
		}
		snappyVal, err := snappy.Decode(nil, val)
		if err != nil {
			return err
		}
		valCopy = append([]byte{}, snappyVal...)
		return nil
	})
	if err != nil {
		log.Error("BoltDB Get failed.", "err", err.Error(), "key", string(key))
		return nil
	}

	return valCopy
}

func (db *BoltDB) Load(key []byte) ([]byte, error) {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)
	var valCopy []byte

	err := db.dbs[index].View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(defaultBucket))
		if b == nil {
			return errors.New("default bucket not exist")
		}
		val := b.Get(key)
		if len(val) == 0 {
			return errors.New("KeyNotFound")
		}
		snappyVal, err := snappy.Decode(nil, val)
		if err != nil {
			return err
		}
		valCopy = append([]byte{}, snappyVal...)
		return nil
	})

	return valCopy, err
}

func (db *BoltDB) Has(key []byte) bool {
	return db.Get(key) != nil
}

func (db *BoltDB) Exist(key []byte) (bool, error) {
	v, err := db.Load(key)
	return v != nil, err
}

func (db *BoltDB) Set(key []byte, val []byte) {
	key = nonNilBytes(key)
	val = nonNilBytes(val)
	index := dbIndex(key, db.dbCounts)

	err := db.dbs[index].Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(defaultBucket))
		if b == nil {
			return errors.New("default bucket not exist")
		}
		snappyVal := snappy.Encode(nil, val)
		return b.Put(key, snappyVal)
	})
	if err != nil  {
		log.Error("BoltDB Set failed.", "err", err.Error(),
			"key", string(key), "val", string(val))
	}
}

func (db *BoltDB) Put(key[]byte, val[]byte) error {
	key = nonNilBytes(key)
	val = nonNilBytes(val)
	index := dbIndex(key, db.dbCounts)

	err := db.dbs[index].Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(defaultBucket))
		if b == nil {
			return errors.New("default bucket not exist")
		}
		snappyVal := snappy.Encode(nil, val)
		return b.Put(key, snappyVal)
	})

	return err
}

func (db *BoltDB) SetSync(key []byte, val []byte) {
	db.Set(key, val)
}

func (db *BoltDB) Delete(key []byte) {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)
	err := db.dbs[index].Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(defaultBucket))
		if b == nil {
			return errors.New("default bucket not exist")
		}
		return b.Delete(key)
	})
	if err != nil {
		log.Error("BoltDB Delete failed", "err", err.Error(), "key", string(key))
	}
}

func (db *BoltDB) Del(key []byte) error {
	key = nonNilBytes(key)
	index := dbIndex(key, db.dbCounts)
	err := db.dbs[index].Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(defaultBucket))
		if b == nil {
			return errors.New("default bucket not exist")
		}
		return b.Delete(key)
	})
	return err
}

func (db *BoltDB) DeleteSync(key []byte) {
	db.Delete(key)
}

func (db *BoltDB) Close() {
	for index := uint64(0); index < db.dbCounts; index++ {
		db.dbs[index].Close()
	}
}

func (db *BoltDB) Print() {
}

func (db *BoltDB) Stats() map[string]string {
	return make(map[string]string, 0)
}

func (db *BoltDB) Dir() string {
	return filepath.Dir(db.dbPaths[0])
}

type batchMsg struct{
	key     []byte
	val     []byte
	isDel   bool
}

type boltBatch struct {
	dbs      []*bolt.DB
	dbCounts uint64
	batchs   [][]*batchMsg
	size     int
}

func initBatchs(dbCounts uint64) [][]*batchMsg {
	batchs := make([][]*batchMsg, dbCounts)
	for index := uint64(0); index < dbCounts; index++ {
		batch := make([]*batchMsg, 0)
		batchs[index] = batch
	}
	return batchs
}

func (db *BoltDB) NewBatch() Batch {
	batchs := initBatchs(db.dbCounts)
	return &boltBatch {
		db.dbs,
		db.dbCounts,
		batchs,
		0,
	}
}

func (mBatch *boltBatch) Set(key, val []byte) {
	key = nonNilBytes(key)
	val = nonNilBytes(val)
	index := dbIndex(key, mBatch.dbCounts)
	snappyVal := snappy.Encode(nil, val)
	msg := &batchMsg{
		key,
		snappyVal,
		false,
	}
	mBatch.batchs[index] = append(mBatch.batchs[index], msg)
	mBatch.size++

	if mBatch.size == boltMaxBatchSize {
		mBatch.Write()
	}
}

func (mBatch *boltBatch) Delete(key []byte) {
	key = nonNilBytes(key)
	index := dbIndex(key, mBatch.dbCounts)
	msg := &batchMsg{
		key,
		nil,
		true,
	}
	mBatch.batchs[index] = append(mBatch.batchs[index], msg)
	mBatch.size++
}

func (mBatch *boltBatch) Write() {
	sw := sync.WaitGroup{}
	for index := uint64(0); index < mBatch.dbCounts; index++ {
		sw.Add(1)
		go func(index uint64) {
			defer sw.Done()
			if len(mBatch.batchs[index]) == 0 {
				return
			}
			mBatch.dbs[index].Batch(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte(defaultBucket))
				if b == nil {
					return errors.New("default bucket not exist")
				}
				for _, msg := range mBatch.batchs[index] {
					if msg.isDel {
						b.Delete(msg.key)
					} else {
						b.Put(msg.key, msg.val)
					}
				}
				return nil
			})
		}(index)
	}
	sw.Wait()
	mBatch.Reset()
}

func (mBatch *boltBatch) Commit() error {
	var retErr error
	sw := sync.WaitGroup{}
	for index := uint64(0); index < mBatch.dbCounts; index++ {
		sw.Add(1)
		go func(index uint64) {
			defer sw.Done()
			if len(mBatch.batchs[index]) == 0 {
				return
			}
			err := mBatch.dbs[index].Batch(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte(defaultBucket))
				if b == nil {
					return errors.New("default bucket not exist")
				}
				for _, msg := range mBatch.batchs[index] {
					if msg.isDel {
						b.Delete(msg.key)
					} else {
						b.Put(msg.key, msg.val)
					}
				}
				return nil
			})
			if err != nil {
				retErr = err
			}
		}(index)

	}
	sw.Wait()
	mBatch.Reset()
	return retErr
}

func (mBatch *boltBatch) WriteSync() {
	mBatch.Write()
}

func (mBatch *boltBatch) ValueSize() int {
	return mBatch.size
}

func (mBatch *boltBatch) Reset() {
	mBatch.batchs = initBatchs(mBatch.dbCounts)
	mBatch.size = 0
}

func (db *BoltDB) Iterator(start, end []byte) Iterator {
	return newBoltIterator(db.dbs, db.dbCounts, start, end, false)
}

func (db *BoltDB) ReverseIterator(start, end []byte) Iterator {
	return newBoltIterator(db.dbs, db.dbCounts, start, end, true)
}

func (db *BoltDB) NewIteratorWithPrefix(prefix []byte) Iterator {
	return newBoltIterator(db.dbs, db.dbCounts, prefix, PrefixToEnd(prefix), false)
}

type iterKv struct {
	key []byte
	val []byte
}

type boltIterator struct {
	dbs       []*bolt.DB
	dbCounts  uint64
	start     []byte
	end       []byte
	isReverse bool
	isInvalid bool
	kvs       []iterKv
	dbIndex   uint64
}

var _ Iterator = (*boltIterator)(nil)

func newBoltIterator(dbs []*bolt.DB, dbCounts uint64, start, end []byte, isReverse bool) *boltIterator {
	kvs := make([]iterKv, dbCounts)
	for index := uint64(0); index < dbCounts; index++ {
		var key, val []byte
		err := dbs[index].View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(defaultBucket))
			if b == nil {
				return errors.New("default bucket not exist")
			}
			cursor := b.Cursor()
			if isReverse {
				if start == nil {
					key, val = cursor.Last()
				} else {
					key, val = cursor.Seek(start)
					if key != nil {
						if bytes.Compare(start, key) < 0 {
							key, val = cursor.Prev()
						}
					} else {
						key, val = cursor.Last()
					}
				}
			} else {
				if start == nil {
					key, val = cursor.First()
				} else {
					key, val = cursor.Seek(start)
				}
			}
			kvs[index].key = cp(key)
			kvs[index].val = cp(val)

			return nil
		})
		if err != nil {
			log.Error("newBoltIterator failed.", "start", string(start), "end", string(end),
				"isReverse", isReverse, "err", err.Error())
			return nil
		}
	}

	return &boltIterator{
		dbs:       dbs,
		dbCounts:  dbCounts,
		start:     start,
		end:       end,
		isReverse: isReverse,
		isInvalid: false,
		kvs:       kvs,
		dbIndex:   0,
	}
}

func (itr *boltIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

func (itr *boltIterator) Valid() bool {
	if itr.isInvalid {
		return false
	}
	if len(itr.kvs[itr.dbIndex].key) == 0 {
		if itr.dbIndex + 1 == itr.dbCounts {
			itr.isInvalid = true
			return false
		}
		itr.dbIndex++
		return itr.Valid()
	}
	if itr.isReverse {
		if itr.end != nil && bytes.Compare(itr.kvs[itr.dbIndex].key, itr.end) <= 0 {
			if itr.dbIndex + 1 == itr.dbCounts {
				itr.isInvalid = true
				return false
			}
			itr.dbIndex++
			return itr.Valid()
		}
	} else {
		if itr.end != nil && bytes.Compare(itr.end, itr.kvs[itr.dbIndex].key) <= 0 {
			if itr.dbIndex + 1 == itr.dbCounts {
				itr.isInvalid = true
				return false
			}
			itr.dbIndex++
			return itr.Valid()
		}
	}

	return true
}

func (itr *boltIterator) Key() []byte {
	itr.assertIteratorValid()
	return cp(itr.kvs[itr.dbIndex].key)
}

func (itr *boltIterator) Value() []byte {
	itr.assertIteratorValid()
	snappyVal, err := snappy.Decode(nil, itr.kvs[itr.dbIndex].val)
	if err != nil {
		log.Error("boltIterator Value snappy Decode failed.", "err", err.Error())
		return nil
	}

	return cp(snappyVal)
}

func (itr *boltIterator) Next() bool {
	// must check iterator valid.
	itr.assertIteratorValid()

	tmpDbIndex := itr.dbIndex
	var key, val []byte
	err := itr.dbs[itr.dbIndex].View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(defaultBucket))
		if b == nil {
			return errors.New("default bucket not exist")
		}
		cursor := b.Cursor()
		// seek to last seek location
		cursor.Seek(itr.kvs[itr.dbIndex].key)
		if itr.isReverse {
			key, val = cursor.Prev()
		} else {
			key, val = cursor.Next()
		}
		return nil
	})
	if err != nil {
		log.Error("boltIterator Next failed.", "err", err.Error())
		return false
	}
	if !itr.Valid() {
		return false
	}
	if tmpDbIndex != itr.dbIndex {
		return itr.Next()
	}
	itr.kvs[itr.dbIndex].key = cp(key)
	itr.kvs[itr.dbIndex].val = cp(val)

	return true
}

func (itr *boltIterator) Seek(skey []byte) bool {
	itr.isInvalid = false
	kvs := make([]iterKv, itr.dbCounts)

	for index := uint64(0); index < itr.dbCounts; index++ {
		var key, val []byte
		err := itr.dbs[index].View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(defaultBucket))
			if b == nil {
				return errors.New("default bucket not exist")
			}
			cursor := b.Cursor()
			if itr.isReverse {
				if skey == nil {
					key, val = cursor.Last()
				} else {
					key, val = cursor.Seek(skey)
					if key != nil {
						if bytes.Compare(skey, key) < 0 {
							key, val = cursor.Prev()
						}
					} else {
						key, val = cursor.Last()
					}
				}
			} else {
				if skey == nil {
					key, val = cursor.First()
				} else {
					key, val = cursor.Seek(skey)
				}
			}
			return nil
		})
		if err != nil {
			log.Error("newBoltIterator failed.", "start", string(itr.start), "end", string(itr.end),
				"isReverse", itr.isReverse, "err", err.Error())
			return false
		}
		kvs[index].key = cp(key)
		kvs[index].val = cp(val)
	}

	itr.start = skey
	itr.kvs = kvs
	itr.dbIndex = 0
	return itr.Valid()
}

func (itr *boltIterator) Close() {
	itr.kvs = nil
	itr.isInvalid = true
}

func (itr *boltIterator) assertIteratorValid() {
	if !itr.Valid() {
		panic("boltDBIterator is invalid")
	}
}
