package state

import (
	"bytes"
	"container/heap"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/trie"
	"golang.org/x/crypto/sha3"
)

const (
	test1 = 12345
)

var (
	deleteItem = []byte("")
	deleteVal  = []byte("DD")
	kvHeight   = []byte("kvh")
	hBytes     = make([]byte, 8)
	lenBuf     = make([]byte, 4)
	walFile    = "kvState.wal"
	dbNotFound = fmt.Errorf("leveldb: not found")
)

type wrappedDB struct {
	isTrie bool
	db     dbm.DB
	oldDB  Database
	wal    *os.File
}

func saveHeight(db dbm.DB, height uint64) {
	binary.BigEndian.PutUint64(hBytes, height)
	db.SetSync(kvHeight, hBytes)
}

func loadHeight(db dbm.DB) uint64 {
	h, err := db.Load(kvHeight)
	if err == nil && len(h) >= 8 {
		return binary.BigEndian.Uint64(h)
	}
	return 0
}

func CanRollBackOneBlock(db dbm.DB, height uint64) bool {
	if height > 0 {
		if h := loadHeight(db); h == height {
			return true
		}
		filename := filepath.Join(db.Dir(), walFile)
		if !common.FileExists(filename) {
			// isTrie mode
			return true
		}
	}
	return false
}

func rebuildLastState(db dbm.DB, file *os.File) error {
	_, err := file.Seek(0, 0)
	if err != nil {
		return err
	}
	fi, err := file.Stat()
	if err != nil {
		return err
	}
	size := fi.Size()
	if size < 1 {
		return nil
	}
	buf := make([]byte, size)
	read := int64(0)
	for {
		n, err := file.Read(buf[read:])
		read += int64(n)
		if read == size {
			break
		}
		if err != nil {
			return err
		}
	}

	var key, value []byte
	var n uint32

	bat := db.NewBatch()
	for len(buf) > 0 {
		// TODO buf length must >= 4
		n = binary.BigEndian.Uint32(buf)
		key = buf[4 : n+4]
		buf = buf[n+4:]
		n = binary.BigEndian.Uint32(buf)
		value = buf[4 : n+4]
		buf = buf[n+4:]

		if n > 0 {
			bat.Set(key, value)
		} else {
			bat.Delete(key)
		}
	}
	bat.Write()
	return nil
}

func NewKeyValueDBWithCache(db dbm.DB, cache int, isTrie bool, height uint64) Database {
	var err error
	var wal *os.File
	var oldDB Database = nil
	if isTrie {
		oldDB = NewDatabase(db)
	} else if cache > 0 {
		filename := filepath.Join(db.Dir(), walFile)
		wal, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, os.FileMode(0600))
		if err != nil {
			panic(fmt.Errorf("open %s failed, err: %v", filename, err))
		}
		kvh := loadHeight(db)
		switch kvh {
		case height:
			// last block saved ok
			break
		case height + 1:
			// need go back to last kv state
			err = rebuildLastState(db, wal)
			if err != nil {
				panic(fmt.Errorf("rebuildLastState from %s failed, err: %v", filename, err))
			}
		case 0:
			// init
			break
		default:
			panic(fmt.Errorf("kvStateHeight is %v, blockStoreHeight is %v)", kvh, height))
		}
	}
	return &wrappedDB{
		isTrie: isTrie,
		db:     db,
		oldDB:  oldDB,
		wal:    wal,
	}
}

func (kv *wrappedDB) OpenTrie(root common.Hash) (Trie, error) {
	var oldTrie Trie
	var err error
	if kv.isTrie {
		oldTrie, err = kv.oldDB.OpenTrie(root)
	}
	kh := &kvHeap{}
	heap.Init(kh)
	return &wrappedTrie{
		isTrie:  kv.isTrie,
		oldTrie: oldTrie,
		db:      kv,
		addr:    nil,
		buffer:  make([]byte, 0, 256),
		serial:  kh,
		updates: make(map[string][]byte),
		//cache:   make(map[string][]byte),
	}, err
}

func (kv *wrappedDB) OpenStorageTrie(addrHash, root common.Hash) (Trie, error) {
	var oldTrie Trie
	var err error
	if kv.isTrie {
		oldTrie, err = kv.oldDB.OpenStorageTrie(addrHash, root)
	}
	kh := &kvHeap{}
	heap.Init(kh)
	return &wrappedTrie{
		isTrie:  kv.isTrie,
		oldTrie: oldTrie,
		db:      kv,
		addr:    addrHash[:],
		buffer:  make([]byte, 0, 256),
		serial:  kh,
		updates: make(map[string][]byte),
		//cache:   make(map[string][]byte),
	}, err
}

func (kv *wrappedDB) CopyTrie(t Trie) Trie {
	switch t := t.(type) {
	case *wrappedTrie:
		var oldTrie Trie
		if t.isTrie {
			oldTrie = kv.oldDB.CopyTrie(t.oldTrie)
		}
		kh := &kvHeap{}
		heap.Init(kh)
		return &wrappedTrie{
			isTrie:  kv.isTrie,
			oldTrie: oldTrie,
			db:      kv,
			addr:    t.addr,
			buffer:  make([]byte, 0, 256),
			serial:  kh,
			updates: make(map[string][]byte),
			//cache:   make(map[string][]byte),
		}
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}

func (kv *wrappedDB) ContractCode(addrHash, codeHash common.Hash) ([]byte, error) {
	if kv.isTrie {
		return kv.oldDB.ContractCode(addrHash, codeHash)
	}
	return kv.db.Load(codeHash[:])
}

func (kv *wrappedDB) ContractCodeSize(addrHash, codeHash common.Hash) (int, error) {
	code, err := kv.ContractCode(addrHash, codeHash)
	return len(code), err
}

func (kv *wrappedDB) TrieDB() TrieDB {
	if kv.isTrie {
		return kv.oldDB.TrieDB()
	}
	return kv
}

func (kv *wrappedDB) SaveWAL(height uint64) {
	if kv.wal != nil {
		err := kv.wal.Truncate(0)
		if err != nil {
			panic(err)
		}
		saveHeight(kv.db, height)
	}
}

func (kv *wrappedDB) saveWAL(wb []byte) error {
	if kv.wal != nil {
		for len(wb) > 0 {
			n, err := kv.wal.Write(wb)
			if err != nil {
				return err
			}
			wb = wb[n:]
		}
		err := kv.wal.Sync()
		if err != nil {
			return err
		}
	}
	return nil
}

//******************************************************
func (kv *wrappedDB) DiskDB() trie.DatabaseReader {
	return kv.db
}

func (kv *wrappedDB) InsertBlob(hash common.Hash, blob []byte) {
	kv.db.Put(hash[:], blob)
}

func (kv *wrappedDB) Node(hash common.Hash) ([]byte, error) {
	return nil, fmt.Errorf("unimplemented method (%T).Node", kv)
}

func (kv *wrappedDB) Nodes() []common.Hash {
	return nil
}

func (kv *wrappedDB) Reference(child common.Hash, parent common.Hash) {
	return
}

func (kv *wrappedDB) Commit(node common.Hash, report bool) error {
	return fmt.Errorf("unimplemented method (%T).Commit", kv)
}

//======================================================
type wrappedTrie struct {
	isTrie  bool
	oldTrie Trie
	db      *wrappedDB

	addr    []byte
	buffer  []byte
	serial  *kvHeap
	updates map[string][]byte

	//cache map[string][]byte
	walBz []byte
}

type kvHeap [][]byte

func (kh kvHeap) Len() int {
	return len(kh)
}

func (kh kvHeap) Swap(i, j int) {
	kh[i], kh[j] = kh[j], kh[i]
}

func (kh kvHeap) Less(i, j int) bool {
	switch bytes.Compare(kh[i], kh[j]) {
	case -1:
		return true
	default:
		return false
	}
}

func (kh *kvHeap) Push(h interface{}) {
	*kh = append(*kh, h.([]byte))
}

func (kh *kvHeap) Pop() (x interface{}) {
	n := len(*kh)
	x = (*kh)[n-1]
	*kh = (*kh)[:n-1]
	return x
}

func keyHash(key []byte) []byte {
	sha := sha3.NewLegacyKeccak256()
	sha.Reset()
	sha.Write(key)
	hash := sha.Sum(nil)
	return hash
}

func (kvTrie *wrappedTrie) TryGet(key []byte) ([]byte, error) {
	if kvTrie.isTrie {
		return kvTrie.oldTrie.TryGet(key)
	}
	keyhash := keyHash(key)
	if kvTrie.addr != nil {
		keyhash = append(kvTrie.addr, keyhash...)
	}
	v, _ := kvTrie.db.db.Load(keyhash)
	/*
		if len(v) > 0 {
			kvTrie.cache[string(key)] = v
		}
	*/
	return v, nil // dbNotFound
}

func (kvTrie *wrappedTrie) TryUpdate(key, value []byte) error {
	keyhash := keyHash(key)
	kvTrie.updates[string(keyhash)] = value
	heap.Push(kvTrie.serial, append(keyhash, value...))
	if kvTrie.isTrie {
		return kvTrie.oldTrie.TryUpdate(key, value)
	}
	return nil
}

func (kvTrie *wrappedTrie) TryDelete(key []byte) error {
	keyhash := keyHash(key)
	kvTrie.updates[string(keyhash)] = deleteItem
	heap.Push(kvTrie.serial, append(keyhash, deleteVal...))
	if kvTrie.isTrie {
		return kvTrie.oldTrie.TryDelete(key)
	}
	return nil
}

func (kvTrie *wrappedTrie) Commit(onleaf trie.LeafCallback, height uint64) (common.Hash, error) {
	heap.Init(kvTrie.serial)
	if kvTrie.isTrie {
		return kvTrie.oldTrie.Commit(onleaf, height)
	}
	bat := kvTrie.db.db.NewBatch()
	for k, v := range kvTrie.updates {
		key := []byte(k)
		if kvTrie.addr != nil {
			key = append(kvTrie.addr, key...)
		}

		binary.BigEndian.PutUint32(lenBuf, uint32(len(key)))
		kvTrie.walBz = append(kvTrie.walBz, lenBuf...)
		kvTrie.walBz = append(kvTrie.walBz, key...)
		ov, _ := kvTrie.db.db.Load(key)
		if len(ov) > 0 {
			binary.BigEndian.PutUint32(lenBuf, uint32(len(ov)))
			kvTrie.walBz = append(kvTrie.walBz, lenBuf...)
			kvTrie.walBz = append(kvTrie.walBz, ov...)
		} else {
			binary.BigEndian.PutUint32(lenBuf, 0)
			kvTrie.walBz = append(kvTrie.walBz, lenBuf...)
		}

		if len(v) == 0 {
			bat.Delete(key)
		} else {
			bat.Set(key, v)
		}
	}
	err := kvTrie.db.saveWAL(kvTrie.walBz)
	if err != nil {
		panic(err)
	}
	kvTrie.walBz = kvTrie.walBz[:0]
	//	kvTrie.cache = make(map[string][]byte)
	err = bat.Commit()
	if err != nil {
		return common.Hash{}, err
	}
	kvTrie.updates = make(map[string][]byte)
	return common.Hash{}, nil
}

func (kvTrie *wrappedTrie) Hash() common.Hash {
	if kvTrie.isTrie {
		kvTrie.oldTrie.Hash()
	}
	kvTrie.buffer = kvTrie.buffer[:0]
	for kvTrie.serial.Len() > 0 {
		v := heap.Pop(kvTrie.serial).([]byte)
		kvTrie.buffer = append(kvTrie.buffer, v...)
	}
	return common.BytesToHash(crypto.Keccak256(kvTrie.buffer[:]))
}

func (kvTrie *wrappedTrie) NodeIterator(startKey []byte) trie.NodeIterator {
	if kvTrie.isTrie {
		return kvTrie.oldTrie.NodeIterator(startKey)
	}
	return nil
}

func (kvTrie *wrappedTrie) GetKey(key []byte) []byte {
	if kvTrie.isTrie {
		return kvTrie.oldTrie.GetKey(key)
	}
	return nil
}

func (kvTrie *wrappedTrie) Prove(key []byte, fromLevel uint, proofDb dbm.Putter) error {
	if kvTrie.isTrie {
		return kvTrie.oldTrie.Prove(key, fromLevel, proofDb)
	}
	return fmt.Errorf("unimplemented method (%T).Prove", kvTrie)
}
