package db

import (
	"sync"
)

type atomicSetDeleter interface {
	Mutex() *sync.Mutex
	SetNoLock(key, value []byte)
	SetNoLockSync(key, value []byte)
	DeleteNoLock(key []byte)
	DeleteNoLockSync(key []byte)
}

type memBatch struct {
	db  atomicSetDeleter
	ops []operation

	size int
}

type opType int

const (
	opTypeSet    opType = 1
	opTypeDelete opType = 2
)

type operation struct {
	opType
	key   []byte
	value []byte
}

func (mBatch *memBatch) Set(key, value []byte) {
	mBatch.ops = append(mBatch.ops, operation{opTypeSet, key, value})
	mBatch.size += len(value)
}

func (mBatch *memBatch) Delete(key []byte) {
	mBatch.ops = append(mBatch.ops, operation{opTypeDelete, key, nil})
	mBatch.size += 1
}

func (mBatch *memBatch) Write() {
	mBatch.write(false)
}
func (mBatch *memBatch) Commit() error {
	mBatch.write(false)
	return nil
}

func (mBatch *memBatch) WriteSync() {
	mBatch.write(true)
}

func (mBatch *memBatch) write(doSync bool) {
	if mtx := mBatch.db.Mutex(); mtx != nil {
		mtx.Lock()
		defer mtx.Unlock()
	}

	for i, op := range mBatch.ops {
		if doSync && i == (len(mBatch.ops)-1) {
			switch op.opType {
			case opTypeSet:
				mBatch.db.SetNoLockSync(op.key, op.value)
			case opTypeDelete:
				mBatch.db.DeleteNoLockSync(op.key)
			}
			break // we're done.
		}
		switch op.opType {
		case opTypeSet:
			mBatch.db.SetNoLock(op.key, op.value)
		case opTypeDelete:
			mBatch.db.DeleteNoLock(op.key)
		}
	}
}

func (mBatch *memBatch) ValueSize() int {
	return mBatch.size
}

func (mBatch *memBatch) Reset() {
	mBatch.ops = mBatch.ops[:0]
	mBatch.size = 0
}
