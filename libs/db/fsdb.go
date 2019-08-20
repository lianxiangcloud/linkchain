package db

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/pkg/errors"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
)

const (
	keyPerm = os.FileMode(0600)
	dirPerm = os.FileMode(0700)
)

func init() {
	registerDBCreator(FSDBBackend, func(name string, dir string, counts uint64) (DB, error) {
		dbPath := filepath.Join(dir, name+".db")
		return NewFSDB(dbPath), nil
	}, false)
}

var _ DB = (*FSDB)(nil)

// It's slow.
type FSDB struct {
	mtx sync.Mutex
	dir string
}

func NewFSDB(dir string) *FSDB {
	err := os.MkdirAll(dir, dirPerm)
	if err != nil {
		panic(errors.Wrap(err, "Creating FSDB dir "+dir))
	}
	database := &FSDB{
		dir: dir,
	}
	return database
}

func (db *FSDB) Dir() string {
	return filepath.Dir(db.dir)
}

func (db *FSDB) Get(key []byte) []byte {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = escapeKey(key)

	path := db.nameToPath(key)
	value, err := read(path)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		panic(errors.Wrapf(err, "Getting key %s (0x%X)", string(key), key))
	}
	return value
}
func (db *FSDB) Load(key []byte) ([]byte, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = escapeKey(key)

	path := db.nameToPath(key)
	value, err := read(path)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, errors.Wrapf(err, "Getting key %s (0x%X)", string(key), key)
	}
	return value, nil
}

func (db *FSDB) Has(key []byte) bool {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = escapeKey(key)

	path := db.nameToPath(key)
	return cmn.FileExists(path)
}
func (db *FSDB) Exist(key []byte) (bool, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = escapeKey(key)

	path := db.nameToPath(key)
	return cmn.FileExists(path), nil
}

func (db *FSDB) Set(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}
func (db *FSDB) Put(key []byte, value []byte) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.SetNoLockNoPanic(key, value)
}

func (db *FSDB) SetSync(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

// NOTE: Implements atomicSetDeleter.
func (db *FSDB) SetNoLock(key []byte, value []byte) {
	key = escapeKey(key)
	value = nonNilBytes(value)
	path := db.nameToPath(key)
	err := write(path, value)
	if err != nil {
		panic(errors.Wrapf(err, "Setting key %s (0x%X)", string(key), key))
	}
}
func (db *FSDB) SetNoLockNoPanic(key []byte, value []byte) error {
	key = escapeKey(key)
	value = nonNilBytes(value)
	path := db.nameToPath(key)
	return write(path, value)
}

func (db *FSDB) Delete(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}
func (db *FSDB) Del(key []byte) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.DeleteNoLockNoPanic(key)
}

func (db *FSDB) DeleteSync(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}

// NOTE: Implements atomicSetDeleter.
func (db *FSDB) DeleteNoLock(key []byte) {
	key = escapeKey(key)
	path := db.nameToPath(key)
	err := remove(path)
	if os.IsNotExist(err) {
		return
	} else if err != nil {
		panic(errors.Wrapf(err, "Removing key %s (0x%X)", string(key), key))
	}
}
func (db *FSDB) DeleteNoLockNoPanic(key []byte) error {
	key = escapeKey(key)
	path := db.nameToPath(key)
	err := remove(path)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "Removing key %s (0x%X)", string(key), key)
	}
	return nil
}

func (db *FSDB) Close() {
	// Nothing to do.
}

func (db *FSDB) Print() {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	panic("FSDB.Print not yet implemented")
}

func (db *FSDB) Stats() map[string]string {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	panic("FSDB.Stats not yet implemented")
}

func (db *FSDB) NewBatch() Batch {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	// Not sure we would ever want to try...
	// It doesn't seem easy for general filesystems.
	panic("FSDB.NewBatch not yet implemented")
}

func (db *FSDB) Mutex() *sync.Mutex {
	return &(db.mtx)
}

func (db *FSDB) Iterator(start, end []byte) Iterator {
	return db.MakeIterator(start, end, false)
}

func (db *FSDB) NewIteratorWithPrefix(prefix []byte) Iterator {
	return db.MakeIterator(prefix, PrefixToEnd(prefix), false)
}

func (db *FSDB) MakeIterator(start, end []byte, isReversed bool) Iterator {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	// We need a copy of all of the keys.
	// Not the best, but probably not a bottleneck depending.
	keys, err := list(db.dir, start, end, isReversed)
	if err != nil {
		panic(errors.Wrapf(err, "Listing keys in %s", db.dir))
	}
	if isReversed {
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	} else {
		sort.Strings(keys)
	}
	return newFSDBIterator(db, keys, start, end, isReversed)
}

func (db *FSDB) ReverseIterator(start, end []byte) Iterator {
	return db.MakeIterator(start, end, true)
}

func (db *FSDB) nameToPath(name []byte) string {
	n := url.PathEscape(string(name))
	return filepath.Join(db.dir, n)
}

type fsDBIterator struct {
	db        *FSDB
	cur       int
	keys      []string
	start     []byte
	end       []byte
	isReverse bool
}

var _ Iterator = (*fsDBIterator)(nil)

// Keys is expected to be in reverse order for reverse iterators.
func newFSDBIterator(db *FSDB, keys []string, start, end []byte, isReverse bool) *fsDBIterator {
	return &fsDBIterator{
		db:        db,
		cur:       0,
		keys:      keys,
		start:     start,
		end:       end,
		isReverse: isReverse,
	}
}

// Implements Iterator.
func (itr *fsDBIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

// Implements Iterator.
func (itr *fsDBIterator) Valid() bool {
	return 0 <= itr.cur && itr.cur < len(itr.keys)
}

// Implements Iterator.
func (itr *fsDBIterator) Next() (valid bool) {
	//itr.assertIsValid()
	if valid = itr.Valid(); !valid {
		return
	}
	itr.cur++
	return
}

// Implements Iterator.
func (itr *fsDBIterator) Key() []byte {
	itr.assertIsValid()
	return []byte(itr.keys[itr.cur])
}

// Implements Iterator.
func (itr *fsDBIterator) Value() []byte {
	itr.assertIsValid()
	key := []byte(itr.keys[itr.cur])
	return itr.db.Get(key)
}

func (itr *fsDBIterator) Seek(key []byte) bool {
	itr.db.mtx.Lock()
	defer itr.db.mtx.Unlock()

	keys, err := list(itr.db.dir, key, itr.end, itr.isReverse)
	if err != nil {
		panic(errors.Wrapf(err, "Listing keys in %s", itr.db.dir))
	}
	if itr.isReverse {
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	} else {
		sort.Strings(keys)
	}

	itr.keys = keys
	itr.cur = 0
	itr.start = key
	return itr.Valid()
}

// Implements Iterator.
func (itr *fsDBIterator) Close() {
	itr.keys = nil
	itr.db = nil
}

func (itr *fsDBIterator) assertIsValid() {
	if !itr.Valid() {
		panic("fsDBIterator is invalid")
	}
}

// Read some bytes to a file.
// CONTRACT: returns os errors directly without wrapping.
func read(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	d, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// Write some bytes from a file.
// CONTRACT: returns os errors directly without wrapping.
func write(path string, d []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, keyPerm)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(d)
	if err != nil {
		return err
	}
	err = f.Sync()
	return err
}

// Remove a file.
// CONTRACT: returns os errors directly without wrapping.
func remove(path string) error {
	return os.Remove(path)
}

// List keys in a directory, stripping of escape sequences and dir portions.
// CONTRACT: returns os errors directly without wrapping.
func list(dirPath string, start, end []byte, isReversed bool) ([]string, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	names, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, name := range names {
		n, err := url.PathUnescape(name)
		if err != nil {
			return nil, fmt.Errorf("Failed to unescape %s while listing", name)
		}
		key := unescapeKey([]byte(n))
		if IsKeyInDomain(key, start, end, isReversed) {
			keys = append(keys, string(key))
		}
	}
	return keys, nil
}

// To support empty or nil keys, while the file system doesn't allow empty
// filenames.
func escapeKey(key []byte) []byte {
	return []byte("k_" + string(key))
}
func unescapeKey(escKey []byte) []byte {
	if len(escKey) < 2 {
		panic(fmt.Sprintf("Invalid esc key: %x", escKey))
	}
	if string(escKey[:2]) != "k_" {
		panic(fmt.Sprintf("Invalid esc key: %x", escKey))
	}
	return escKey[2:]
}
