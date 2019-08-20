package db

import (
	"os"
	"fmt"
	"testing"
)

const (
	boltDBName   = "bolt"
	boltDBCounts = uint64(4)
)

var (
	boltDBPath = os.TempDir()
)

func TestCreateBoltDB(t *testing.T) {
	os.MkdirAll(boltDBPath, os.ModePerm)

	bolt, err := NewBoltDB(boltDBName, boltDBPath, boltDBCounts)
	if err != nil {
		t.Fatal("NewBoltDB failed.", "err", err.Error())
	}

	dbFiles := recordFiles(boltDBPath, boltDBName)
	defer removeFiles(dbFiles)
	if len(dbFiles) != int(bolt.dbCounts) {
		t.Fatal("DB counts not eq. bolt.dbCounts is", bolt.dbCounts)
	}

	bolt.Close()
}

func TestBoltDBSingleSetAndGet(t *testing.T) {
	os.MkdirAll(boltDBPath, os.ModePerm)

	bolt, err := NewBoltDB(boltDBName, boltDBPath, boltDBCounts)
	if err != nil {
		fmt.Println("NewBoltDB failed.", "err", err)
		return
	}
	fmt.Println("new bolt db success.")

	dbFiles := recordFiles(boltDBPath, boltDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	fmt.Println("start to set kvs..", "kvs length is", len(kvs))
	for index, kv := range kvs {
		if index % 200 == 0 {
			fmt.Println("set kvs ing.", "current_index", index)
		}
		bolt.Set(kv.key, kv.val)
	}
	fmt.Println("start to get kvs")
	for _, kv := range kvs {
		val := bolt.Get(kv.key)
		if string(val) != string(kv.val) {
			t.Fatal("val not eq.", "key", string(kv.key), "val", string(kv.val), "get_val", string(val))
		}
	}
	bolt.Close()
}

func TestBoltDBSinglePutAndLoad(t *testing.T) {
	os.MkdirAll(boltDBPath, os.ModePerm)

	bolt, err := NewBoltDB(boltDBName, boltDBPath, boltDBCounts)
	if err != nil {
		fmt.Println("NewBoltDB failed.", "err", err)
		return
	}
	fmt.Println("new bolt db success.")

	dbFiles := recordFiles(boltDBPath, boltDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	fmt.Println("start to set kvs..", "kvs length is", len(kvs))
	for index, kv := range kvs {
		if index % 200 == 0 {
			fmt.Println("put kvs ing.", "current_index", index)
		}
		err := bolt.Put(kv.key, kv.val)
		if err != nil {
			t.Fatal(err)
		}
	}
	fmt.Println("start to get kvs")
	for _, kv := range kvs {
		val, err := bolt.Load(kv.key)
		if err != nil {
			t.Fatal(err)
		}
		if string(val) != string(kv.val) {
			t.Fatal("val not eq.", "key", string(kv.key), "val", string(kv.val), "get_val", string(val))
		}
	}

	bolt.Close()
}

func TestBoltDBBatchWrite(t *testing.T) {
	os.MkdirAll(boltDBPath, os.ModePerm)

	bolt, err := NewBoltDB(boltDBName, boltDBPath, boltDBCounts)
	if err != nil {
		fmt.Println("NewBoltDB failed.", "err", err)
		return
	}
	fmt.Println("new bolt db success.")

	dbFiles := recordFiles(boltDBPath, boltDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	batch := bolt.NewBatch()
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
		val, err := bolt.Load(kv.key)
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
	bolt.Close()
}

func TestBoltDBBatchCommit(t *testing.T) {
	os.MkdirAll(boltDBPath, os.ModePerm)

	bolt, err := NewBoltDB(boltDBName, boltDBPath, boltDBCounts)
	if err != nil {
		fmt.Println("NewBoltDB failed.", "err", err)
		return
	}
	fmt.Println("new bolt db success.")

	dbFiles := recordFiles(boltDBPath, boltDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	batch := bolt.NewBatch()
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
		val, err := bolt.Load(kv.key)
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

	bolt.Close()
}

func TestBoltDBIteratorReadAll(t *testing.T) {
	os.MkdirAll(boltDBPath, os.ModePerm)

	bolt, err := NewBoltDB(boltDBName, boltDBPath, boltDBCounts)
	if err != nil {
		fmt.Println("NewBoltDB failed.", "err", err)
		return
	}
	fmt.Println("new bolt db success.")

	dbFiles := recordFiles(boltDBPath, boltDBName)
	defer removeFiles(dbFiles)

	counts := uint32(kvCounts)
	kvs := genTestMsg(counts)
	batch := bolt.NewBatch()
	defer batch.Reset()
	for _, kv := range kvs {
		batch.Set(kv.key, kv.val)
	}
	batch.Write()

	itr := bolt.Iterator(nil, nil)
	checkMap := make(map[string]string, 0)
	for ; itr.Valid(); itr.Next() {
		k := itr.Key()
		v := itr.Value()
		checkMap[string(k)] = string(v)
	}
	itr.Close()
	if uint32(len(checkMap)) != counts {
		t.Fatal("iterator did not read all msg", "counts", counts, "read_counts", len(checkMap))
	}

	reverseItr := bolt.ReverseIterator(nil, nil)
	checkReverseMap := make(map[string]string, 0)
	for ; reverseItr.Valid(); reverseItr.Next() {
		k := reverseItr.Key()
		v := reverseItr.Value()
		checkReverseMap[string(k)] = string(v)
	}
	reverseItr.Close()
	if uint32(len(checkReverseMap)) != counts {
		t.Fatal("reverse iterator did not read all msg", "counts", counts, "read_counts", len(checkReverseMap))
	}

	bolt.Close()
}

func TestBoltDBIteratorSeek(t *testing.T) {
	os.MkdirAll(boltDBPath, os.ModePerm)

	bolt, err := NewBoltDB(boltDBName, boltDBPath, boltDBCounts)
	if err != nil {
		fmt.Println("NewLevelDB failed.", "err", err)
		return
	}
	fmt.Println("new Level db success.")

	dbFiles := recordFiles(boltDBPath, boltDBName)
	defer removeFiles(dbFiles)

	counts := uint32(kvCounts)
	kvs := genTestMsg(counts)
	batch := bolt.NewBatch()
	defer batch.Reset()
	for _, kv := range kvs {
		batch.Set(kv.key, kv.val)
	}
	batch.Write()

	itr := bolt.Iterator(nil, nil)
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

	reverseItr := bolt.ReverseIterator(nil, nil)
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

	bolt.Close()
}
