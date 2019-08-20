package db

import (
	"os"
	"fmt"
	"testing"
)

const (
	badgerDBName   = "badger"
	badgerDBCounts = uint64(4)
	kvCounts       = 200
)

var (
	badgerDBPath = os.TempDir()
)

func TestCreateBadgerDB(t *testing.T) {
	os.MkdirAll(badgerDBPath, os.ModePerm)

	bdb, err := NewBadgerDB(badgerDBName, badgerDBPath, badgerDBCounts)
	if err != nil {
		t.Fatal("NewBadgerDB failed.", "err", err.Error())
	}

	dbFiles := recordFiles(badgerDBPath, badgerDBName)
	for _, fileName := range dbFiles {
		fmt.Println("fileName:", fileName)
	}
	defer removeFiles(dbFiles)
	if len(dbFiles) != int(bdb.dbCounts) {
		t.Fatal("DB counts not eq. bdb.dbCounts is", bdb.dbCounts)
	}

	bdb.Close()
}

func TestBadgerSingleSetAndGet(t *testing.T) {
	os.MkdirAll(badgerDBPath, os.ModePerm)

	badger, err := NewBadgerDB(badgerDBName, badgerDBPath, badgerDBCounts)
	if err != nil {
		fmt.Println("NewBadgerDB failed.", "err", err)
		return
	}
	fmt.Println("new Badger db success.")

	dbFiles := recordFiles(badgerDBPath, badgerDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	fmt.Println("start to set kvs..", "kvs length is", len(kvs))
	for index, kv := range kvs {
		if index % 200 == 0 {
			fmt.Println("set kvs ing.", "current_index", index)
		}
		badger.Set(kv.key, kv.val)
	}
	fmt.Println("start to get kvs")
	for _, kv := range kvs {
		val := badger.Get(kv.key)
		if string(val) != string(kv.val) {
			t.Fatal("val not eq.", "key", string(kv.key), "val", string(kv.val), "get_val", string(val))
		}
	}

	badger.Close()
}

func TestBadgerSinglePutAndLoad(t *testing.T) {
	os.MkdirAll(badgerDBPath, os.ModePerm)

	badger, err := NewBadgerDB(badgerDBName, badgerDBPath, badgerDBCounts)
	if err != nil {
		fmt.Println("NewBadgerDB failed.", "err", err)
		return
	}
	fmt.Println("new Badger db success.")

	dbFiles := recordFiles(badgerDBPath, badgerDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	fmt.Println("start to set kvs..", "kvs length is", len(kvs))
	for index, kv := range kvs {
		if index % 200 == 0 {
			fmt.Println("put kvs ing.", "current_index", index)
		}
		err := badger.Put(kv.key, kv.val)
		if err != nil {
			t.Fatal(err)
		}
	}
	fmt.Println("start to get kvs")
	for _, kv := range kvs {
		val, err := badger.Load(kv.key)
		if err != nil {
			t.Fatal(err)
		}
		if string(val) != string(kv.val) {
			t.Fatal("val not eq.", "key", string(kv.key), "val", string(kv.val), "get_val", string(val))
		}
	}

	badger.Close()
}

func TestBadgerBatchWrite(t *testing.T) {
	os.MkdirAll(badgerDBPath, os.ModePerm)

	badger, err := NewBadgerDB(badgerDBName, badgerDBPath, badgerDBCounts)
	if err != nil {
		fmt.Println("NewBadgerDB failed.", "err", err)
		return
	}
	fmt.Println("new Badger db success.")

	dbFiles := recordFiles(badgerDBPath, badgerDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	batch := badger.NewBatch()
	defer batch.Reset()
	for index, kv := range kvs {
		if index % 200 == 0 {
			fmt.Println("put kvs ing.", "current_index", index)
		}
		batch.Set(kv.key, kv.val)
	}
	batch.Write()

	fmt.Println("start to get kvs")
	for index, kv := range kvs {
		val, err := badger.Load(kv.key)
		if err != nil {
			t.Fatal(err)
		}
		if index % kvCounts == 0 {
			fmt.Println("Load kvs ing.", "current_index", index)
		}
		if string(val) != string(kv.val) {
			t.Fatal("val not eq.", "key", string(kv.key), "val", string(kv.val), "get_val", string(val))
		}
	}

	badger.Close()
}

func TestBadgerBatchCommit(t *testing.T) {
	os.MkdirAll(badgerDBPath, os.ModePerm)

	badger, err := NewBadgerDB(badgerDBName, badgerDBPath, badgerDBCounts)
	if err != nil {
		fmt.Println("NewBadgerDB failed.", "err", err)
		return
	}
	fmt.Println("new Badger db success.")

	dbFiles := recordFiles(badgerDBPath, badgerDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	batch := badger.NewBatch()
	defer batch.Reset()
	for index, kv := range kvs {
		if index % 200 == 0 {
			fmt.Println("put kvs ing.", "current_index", index)
		}
		batch.Set(kv.key, kv.val)
	}
	err = batch.Commit()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("start to get kvs")
	for index, kv := range kvs {
		val, err := badger.Load(kv.key)
		if err != nil {
			t.Fatal(err)
		}
		if index % 200 == 0 {
			fmt.Println("Load kvs ing.", "current_index", index)
		}
		if string(val) != string(kv.val) {
			t.Fatal("val not eq.", "key", string(kv.key), "val", string(kv.val), "get_val", string(val))
		}
	}
	badger.Close()
}

func TestBadgerIteratorReadAll(t *testing.T) {
	os.MkdirAll(badgerDBPath, os.ModePerm)

	badger, err := NewBadgerDB(badgerDBName, badgerDBPath, badgerDBCounts)
	if err != nil {
		fmt.Println("NewBadgerDB failed.", "err", err)
		return
	}
	fmt.Println("new Badger db success.")

	dbFiles := recordFiles(badgerDBPath, badgerDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	batch := badger.NewBatch()
	defer batch.Reset()
	for _, kv := range kvs {
		batch.Set(kv.key, kv.val)
	}
	batch.Write()

	itr := badger.Iterator(nil, nil)
	checkMap := make(map[string]string, 0)
	for ; itr.Valid(); itr.Next() {
		k := itr.Key()
		v := itr.Value()
		checkMap[string(k)] = string(v)
	}
	itr.Close()
	if uint32(len(checkMap)) != kvCounts {
		t.Fatal("iterator did not read all msg", "counts", kvCounts, "read_counts", len(checkMap))
	}

	reverseItr := badger.ReverseIterator(nil, nil)
	checkReverseMap := make(map[string]string, 0)
	for ; reverseItr.Valid(); reverseItr.Next() {
		k := reverseItr.Key()
		v := reverseItr.Value()
		checkReverseMap[string(k)] = string(v)
	}
	reverseItr.Close()
	if uint32(len(checkReverseMap)) != kvCounts {
		t.Fatal("reverse iterator did not read all msg", "counts", kvCounts, "read_counts", len(checkReverseMap))
	}

	badger.Close()
}

func TestBadgerIteratorSeek(t *testing.T) {
	os.MkdirAll(badgerDBPath, os.ModePerm)

	badger, err := NewBadgerDB(badgerDBName, badgerDBPath, badgerDBCounts)
	if err != nil {
		fmt.Println("NewBadgerDB failed.", "err", err)
		return
	}
	fmt.Println("new Badger db success.")

	dbFiles := recordFiles(badgerDBPath, badgerDBName)
	defer removeFiles(dbFiles)

	counts := uint32(kvCounts)
	kvs := genTestMsg(counts)
	batch := badger.NewBatch()
	defer batch.Reset()
	for _, kv := range kvs {
		batch.Set(kv.key, kv.val)
	}
	batch.Write()

	itr := badger.Iterator(nil, nil)
	checkMap := make(map[string]string, 0)
	for ; itr.Valid(); itr.Next() {
		k := itr.Key()
		v := itr.Value()
		checkMap[string(k)] = string(v)
	}
	if uint32(len(checkMap)) != counts {
		t.Fatal("iterator did not read all msg", "counts", counts, "read_counts", len(checkMap))
	}
	itr.Seek(nil)
	checkSeekMap := make(map[string]string, 0)
	for ; itr.Valid(); itr.Next() {
		k := itr.Key()
		v := itr.Value()
		checkSeekMap[string(k)] = string(v)
	}
	itr.Close()
	if uint32(len(checkMap)) != counts {
		t.Fatal("iterator seek did not read all msg", "counts", counts, "read_counts", len(checkSeekMap))
	}

	reverseItr := badger.ReverseIterator(nil, nil)
	checkReverseMap := make(map[string]string, 0)
	for ; reverseItr.Valid(); reverseItr.Next() {
		k := reverseItr.Key()
		v := reverseItr.Value()
		checkReverseMap[string(k)] = string(v)
	}
	if uint32(len(checkReverseMap)) != counts {
		t.Fatal("reverse iterator did not read all msg", "counts", counts, "read_counts", len(checkReverseMap))
	}
	reverseItr.Seek(nil)
	checkSeekReverseMap := make(map[string]string, 0)
	for ; reverseItr.Valid(); reverseItr.Next() {
		k := reverseItr.Key()
		v := reverseItr.Value()
		checkSeekReverseMap[string(k)] = string(v)
	}
	reverseItr.Close()
	if uint32(len(checkSeekReverseMap)) != counts {
		t.Fatal("reverse iterator seek did not read all msg", "counts", counts, "read_counts", len(checkSeekReverseMap))
	}
	badger.Close()
}
