package db

import (
	"os"
	"fmt"
	"testing"
	"io/ioutil"
	"strings"
	"path/filepath"
)

const (
	levelDBName   = "level"
	levelDBCounts = uint64(4)
)

var (
	levelDBPath = os.TempDir()
)

func removeFiles(files []string) {
	for _, fileName := range files {
		err := os.RemoveAll(fileName)
		if err != nil {
			fmt.Println("removeFiles exec failed!!!!", "fileName", fileName)
		}
	}
}

func recordFiles(path string, dbName string) []string {
	files, _ := ioutil.ReadDir(path)
	dbFiles := make([]string, 0)
	for _, f := range files {
		if !strings.Contains(f.Name(), dbName) {
			continue
		}
		dbFiles = append(dbFiles, filepath.Join(path, f.Name()))
	}
	return dbFiles
}

func TestCreateLevelDB(t *testing.T) {
	os.MkdirAll(levelDBPath, os.ModePerm)

	level, err := NewGoLevelDB(levelDBName, levelDBPath, levelDBCounts)
	if err != nil {
		t.Fatal("NewGoLevelDB failed.", "err", err.Error())
	}

	dbFiles := recordFiles(levelDBPath, levelDBName)
	defer removeFiles(dbFiles)
	if len(dbFiles) != int(level.dbCounts) {
		t.Fatal("DB counts not eq. level.dbCounts is", level.dbCounts)
	}
	level.Close()
}

func TestLevelDBSingleSetAndGet(t *testing.T) {
	os.MkdirAll(levelDBPath, os.ModePerm)

	level, err := NewGoLevelDB(levelDBName, levelDBPath, levelDBCounts)
	if err != nil {
		fmt.Println("NewGoLevelDB failed.", "err", err)
		return
	}
	fmt.Println("new level db success.")

	dbFiles := recordFiles(levelDBPath, levelDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	fmt.Println("start to set kvs..", "kvs length is", len(kvs))
	for index, kv := range kvs {
		if index % 200 == 0 {
			fmt.Println("set kvs ing.", "current_index", index)
		}
		level.Set(kv.key, kv.val)
	}
	fmt.Println("start to get kvs")
	for _, kv := range kvs {
		val := level.Get(kv.key)
		if string(val) != string(kv.val) {
			t.Fatal("val not eq.", "key", string(kv.key), "val", string(kv.val), "get_val", string(val))
		}
	}
	level.Close()
}

func TestLevelDBSinglePutAndLoad(t *testing.T) {
	os.MkdirAll(levelDBPath, os.ModePerm)

	level, err := NewGoLevelDB(levelDBName, levelDBPath, levelDBCounts)
	if err != nil {
		fmt.Println("NewGoLevelDB failed.", "err", err)
		return
	}
	fmt.Println("new level db success.")

	dbFiles := recordFiles(levelDBPath, levelDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	fmt.Println("start to set kvs..", "kvs length is", len(kvs))
	for index, kv := range kvs {
		if index % 200 == 0 {
			fmt.Println("put kvs ing.", "current_index", index)
		}
		err := level.Put(kv.key, kv.val)
		if err != nil {
			t.Fatal(err)
		}
	}
	fmt.Println("start to get kvs")
	for _, kv := range kvs {
		val, err := level.Load(kv.key)
		if err != nil {
			t.Fatal(err)
		}
		if string(val) != string(kv.val) {
			t.Fatal("val not eq.", "key", string(kv.key), "val", string(kv.val), "get_val", string(val))
		}
	}
	level.Close()
}

func TestLevelDBBatchWrite(t *testing.T) {
	os.MkdirAll(levelDBPath, os.ModePerm)

	level, err := NewGoLevelDB(levelDBName, levelDBPath, levelDBCounts)
	if err != nil {
		fmt.Println("NewGoLevelDB failed.", "err", err)
		return
	}
	fmt.Println("new level db success.")

	dbFiles := recordFiles(levelDBPath, levelDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	batch := level.NewBatch()
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
		val, err := level.Load(kv.key)
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
	level.Close()
}

func TestLevelDBBatchCommit(t *testing.T) {
	os.MkdirAll(levelDBPath, os.ModePerm)

	level, err := NewGoLevelDB(levelDBName, levelDBPath, levelDBCounts)
	if err != nil {
		fmt.Println("NewGoLevelDB failed.", "err", err)
		return
	}
	fmt.Println("new level db success.")

	dbFiles := recordFiles(levelDBPath, levelDBName)
	defer removeFiles(dbFiles)

	kvs := genTestMsg(kvCounts)
	batch := level.NewBatch()
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
		val, err := level.Load(kv.key)
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

	level.Close()
}

func TestLevelDBIteratorReadAll(t *testing.T) {
	os.MkdirAll(levelDBPath, os.ModePerm)

	level, err := NewGoLevelDB(levelDBName, levelDBPath, levelDBCounts)
	if err != nil {
		fmt.Println("NewGoLevelDB failed.", "err", err)
		return
	}
	fmt.Println("new level db success.")

	dbFiles := recordFiles(levelDBPath, levelDBName)
	defer removeFiles(dbFiles)

	counts := uint32(kvCounts)
	kvs := genTestMsg(counts)
	batch := level.NewBatch()
	defer batch.Reset()
	for _, kv := range kvs {
		batch.Set(kv.key, kv.val)
	}
	batch.Write()

	itr := level.Iterator(nil, nil)
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

	reverseItr := level.ReverseIterator(nil, nil)
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

	level.Close()
}

func TestLevelDBIteratorSeek(t *testing.T) {
	os.MkdirAll(levelDBPath, os.ModePerm)

	level, err := NewGoLevelDB(levelDBName, levelDBPath, levelDBCounts)
	if err != nil {
		fmt.Println("NewLevelDB failed.", "err", err)
		return
	}
	fmt.Println("new Level db success.")

	dbFiles := recordFiles(levelDBPath, levelDBName)
	defer removeFiles(dbFiles)

	counts := uint32(kvCounts)
	kvs := genTestMsg(counts)
	batch := level.NewBatch()
	defer batch.Reset()
	for _, kv := range kvs {
		batch.Set(kv.key, kv.val)
	}
	batch.Write()

	itr := level.Iterator(nil, nil)
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

	reverseItr := level.ReverseIterator(nil, nil)
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

	level.Close()
}
