package main

import (
	"os"
	"fmt"
	"bytes"
	"bufio"
	"time"
	"math/big"
	"encoding/binary"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/libs/trie"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
)

var (
	emptyRoot = common.HexToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
)

type Dumper interface {
	dump(...common.Address) error
}

type StateDump struct {
	statedb *state.StateDB
	target  *state.StateDB
	destDb  dbm.DB
	db      dbm.DB
}

var (
	stateRoot     = []byte("state_root")
	accountsCount = []byte("accounts_count")
	headerPrefix  = []byte("h")
	numSuffix     = []byte("n")
)

type Bloom [256]byte
type BlockNonce [8]byte

type Header struct {
	ParentHash  common.Hash    `json:"parentHash"       gencodec:"required"`
	UncleHash   common.Hash    `json:"sha3Uncles"       gencodec:"required"`
	Coinbase    common.Address `json:"miner"            gencodec:"required"`
	Root        common.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash      common.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	Bloom       Bloom          `json:"logsBloom"        gencodec:"required"`
	Difficulty  *big.Int       `json:"difficulty"       gencodec:"required"`
	Number      *big.Int       `json:"number"           gencodec:"required"`
	GasLimit    *big.Int       `json:"gasLimit"         gencodec:"required"`
	GasUsed     *big.Int       `json:"gasUsed"          gencodec:"required"`
	Time        *big.Int       `json:"timestamp"        gencodec:"required"`
	Extra       []byte         `json:"extraData"        gencodec:"required"`
	MixDigest   common.Hash    `json:"mixHash"          gencodec:"required"`
	Nonce       BlockNonce     `json:"nonce"            gencodec:"required"`
	OneHashSign []byte         `json:"oneHashSign"      gencodec:"required"`
}

type OldAccount struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash // merkle root of the storage trie
	CodeHash []byte
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func getCanonicalHash(db dbm.DB, number uint64) common.Hash {
	data := db.Get(append(append(headerPrefix, encodeBlockNumber(number)...), numSuffix...))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

func headerKey(hash common.Hash, number uint64) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

func createSymlink(ethermintPath string) error {
	src  := filepath.Join("chaindata")
	dest := filepath.Join(ethermintPath, "chaindata.db")
	err := os.Symlink(src, dest)
	if err != nil {
		fmt.Println("os symlink exec failed.", "src", src, "dest", dest, "err", err.Error())
		return err
	}
	return nil
}

func removeSymlink(ethermintPath string) {
	file := filepath.Join(ethermintPath, "chaindata.db")
	os.Remove(file)
}

func getStateRoot(ethermintPath string, blockHeight uint64) (common.Hash, error) {
	sourceChainDataDB := dbm.NewDB("chaindata", dbm.GoLevelDBBackend, ethermintPath, 0)
	defer sourceChainDataDB.Close()

	hash := getCanonicalHash(sourceChainDataDB, blockHeight)
	if hash == (common.Hash{}) {
		return common.EmptyHash, nil
	}
	data := sourceChainDataDB.Get(headerKey(hash, blockHeight))
	if len(data) == 0 {
		return common.EmptyHash, nil
	}
	header := new(Header)
	if err := ser.Decode(bytes.NewReader(data), header); err != nil {
		fmt.Println("Invalid block header RLP", "hash", hash, "err", err)
		return common.EmptyHash, nil
	}
	hashBytes := header.Root.Bytes()

	return common.BytesToHash(hashBytes), nil
}

func getAccountList(ethermintPath string, root common.Hash) (map[common.Address]OldAccount, error) {
	allAccounts := make(map[common.Address]OldAccount)
	db := dbm.NewDB("chaindata", dbm.GoLevelDBBackend, ethermintPath, 0)
	defer db.Close()
	sdb, err := state.New(root, state.NewDatabase(db))
	if err != nil {
		fmt.Printf("Error %s\n", err)
		return nil, err
	}
	accountlist := sdb.DumpOldAccount()
	fmt.Println("dump account addrs success", "accounts num", len(accountlist))

	for addr, accountBytes := range accountlist {
		oAccount := OldAccount{}
		err := ser.DecodeBytes(accountBytes, &oAccount)
		if err != nil {
			fmt.Println("ser DecodeBytes failed.", "err", err.Error())
			return nil, err
		}
		allAccounts[addr] = oAccount
	}
	return allAccounts, nil
}

func checkAccountListAmount(accountList map[common.Address]OldAccount) error {
	allLianke := new(big.Int).Mul(big.NewInt(15e8), big.NewInt(1e18))
	allAmount := new(big.Int)
	for _, account := range accountList {
		allAmount.Add(allAmount, account.Balance)
	}
	if allAmount.Cmp(allLianke) != 0 {
		fmt.Println("checkAccountListAmount failed.", "allLianke", allLianke.String(), "dumpAllLianke", allAmount.String())
		return errors.New("lianke not eq.")
	}
	return nil
}

func newStateDump(path string, root common.Hash, dumpDir string, isTrie bool) *StateDump {
	db := dbm.NewDB("chaindata", dbm.GoLevelDBBackend, path, 1)
	statedb, err := state.New(root, state.NewDatabase(db))
	if err != nil {
		fmt.Printf("Error %s\n", err)
		return nil
	}

	var dir string
	if isTrie {
		dir = filepath.Join(dumpDir, "db", "full")
	} else {
		dir = filepath.Join(dumpDir, "db", "kv")
	}
	os.MkdirAll(dir, os.ModeDir)
	destDb := dbm.NewDB("state", dbm.GoLevelDBBackend, dir, 1)
	target, err := state.New(common.EmptyHash, state.NewKeyValueDBWithCache(destDb, 128, isTrie, 0))
	if err != nil {
		fmt.Printf("Error %s\n", err.Error())
	}

	dumper := &StateDump{
		statedb: statedb,
		db:      db,
		target:  target,
		destDb:  destDb,
	}
	return dumper
}

func (s *StateDump) dump(addressList map[common.Address]OldAccount) (string, error) {
	fmt.Println("start dump...")
	count := 1
	for addr, oldAccount := range addressList {
		if count % 10000 == 0 {
			fmt.Println("dump ing. current index is", count)
		}
		var account state.Account
		account.Root     = oldAccount.Root
		account.Balance  = oldAccount.Balance
		account.CodeHash = oldAccount.CodeHash
		account.Nonce    = oldAccount.Nonce
		var code []byte
		if !bytes.Equal(account.CodeHash, crypto.Keccak256(nil)) {
			code = s.db.Get(account.CodeHash)
		}
		err := s.insertStateObject(account, addr, code)
		if err != nil {
			fmt.Println("insertStateObject exec failed.", err.Error())
			return "", err
		}
		storageRoot := account.Root
		if storageRoot != common.EmptyHash &&
			storageRoot.Hex() != emptyRoot.Hex() {
			err := s.doStorageDump(addr, storageRoot)
			if err != nil {
				fmt.Println("doStorageDump failed.", "err", err.Error())
				return "", err
			}
		}
		count++
	}
	root, err := s.commit()
	if err != nil {
		fmt.Println("StateDump commit failed.", "err", err.Error())
		return "", err
	}
	fmt.Println("dump finish.", "root", root.Hex())
	return root.Hex(), nil
}

func (s *StateDump) insertStateObject(account state.Account, addr common.Address, code []byte) error {
	if code != nil {
		s.target.SetCode(addr, code)
		codehash := s.target.GetCodeHash(addr)
		if !bytes.Equal(codehash[:], account.CodeHash) {
			fmt.Printf("CodeHash is different  address=%v ,ori hash=%v , new hash=%v", addr.String(),
				common.Bytes2Hex(account.CodeHash), s.target.GetCodeHash(addr).Hex())
			return fmt.Errorf("CodeHash is different  address=%v ,ori hash=%v , new hash=%v",
				addr.String(), common.Bytes2Hex(account.CodeHash), s.target.GetCodeHash(addr).Hex())
		}
	}
	s.target.SetBalance(addr, account.Balance)
	s.target.SetNonce(addr, account.Nonce)
	s.target.SetStorageRoot(addr, account.Root)

	return nil
}

func (s *StateDump) doStorageDump(addr common.Address, storageRoot common.Hash) error {
	fmt.Println("storageRoot", storageRoot.String())
	dstDb := dbm.NewMemDB()
	sched := NewStateSync(storageRoot, dstDb)
	queue := append([]common.Hash{}, sched.Missing(100)...)

	count := 0
	for len(queue) > 0 {
		results := make([]trie.SyncResult, len(queue))
		for i, hash := range queue {
			data := s.db.Get(hash[:])
			if len(data) == 0 {
				return errors.New("get stroage msg failed")
			}
			s.destDb.Set(hash[:], data)
			results[i] = trie.SyncResult{Hash: hash, Data: data}
			count++
		}
		if _, index, err := sched.Process(results); err != nil {
			fmt.Printf("failed to process result #%d: %v\n", index, err)
		}
		if index, err := sched.Commit(dstDb); err != nil {
			fmt.Printf("failed to commit data #%d: %v\n", index, err)
		}
		queue = append(queue[:0], sched.Missing(100)...)
	}
	if count > 0 {
		fmt.Println("doStorageDump exec.", "addr", addr.String(),
			"storageRoot", storageRoot.String(), "count", count)
	}

	return nil
}

// NewStateSync create a new state trie download scheduler.
func NewStateSync(root common.Hash, database trie.DatabaseReader) *trie.Sync {
	var syncer *trie.Sync
	callback := func(leaf []byte, parent common.Hash) error {
		return nil
	}
	syncer = trie.NewSync(root, database, callback)
	return syncer
}

func (s *StateDump) commit() (common.Hash, error) {
	root, err := s.target.Commit(false, 0)
	if err != nil {
		return common.EmptyHash, err
	}
	s.target.Database().TrieDB().Commit(root, false)
	s.target.Reset(root)

	return root, nil
}

func (s *StateDump) dumpKv(addressList map[common.Address]OldAccount) error {
	fmt.Println("start dumpKv...")
	count := 1
	for addr, oldAccount := range addressList {
		if count % 10000 == 0 {
			fmt.Println("dump ing. current index is", count)
		}
		var account state.Account
		account.Root     = oldAccount.Root
		account.Balance  = oldAccount.Balance
		account.CodeHash = oldAccount.CodeHash
		account.Nonce    = oldAccount.Nonce
		var code []byte
		if !bytes.Equal(account.CodeHash, crypto.Keccak256(nil)) {
			code = s.db.Get(account.CodeHash)
		}
		err := s.insertKvStateObject(account, addr, code)
		if err != nil {
			fmt.Println("insertStateObject exec failed.", err.Error())
			return err
		}
		count++
	}
	root, err := s.commitKv()
	if err != nil {
		fmt.Println("StateDump commit failed.", "err", err.Error())
		return err
	}
	fmt.Println("dump finish.", "root", root.String())
	return nil
}

func (s *StateDump) insertKvStateObject(account state.Account, addr common.Address, code []byte) error {
	if code != nil {
		s.target.SetCode(addr, code)
		codehash := s.target.GetCodeHash(addr)
		if !bytes.Equal(codehash[:], account.CodeHash) {
			fmt.Printf("CodeHash is different  address=%v ,ori hash=%v , new hash=%v", addr.String(),
				common.Bytes2Hex(account.CodeHash), s.target.GetCodeHash(addr).Hex())
			return fmt.Errorf("CodeHash is different  address=%v ,ori hash=%v , new hash=%v",
				addr.String(), common.Bytes2Hex(account.CodeHash), s.target.GetCodeHash(addr).Hex())
		}
	}
	s.target.SetBalance(addr, account.Balance)
	s.target.SetNonce(addr, account.Nonce)

	storageRoot := account.Root
	if storageRoot != common.EmptyHash &&
		storageRoot.Hex() != emptyRoot.Hex() {
		storageStateDb, err := state.New(storageRoot, state.NewDatabase(s.db))
		if err != nil {
			fmt.Printf("Error %s\n", err)
			return nil
		}
		fmt.Println("start dump storage", "storageRoot", storageRoot.String())
		storageMsg := storageStateDb.DumpStorage()
		fmt.Println("dump storage success.", "size", len(storageMsg))
		batch := s.destDb.NewBatch()
		count := 0
		for key, val := range storageMsg {
			count++
			keyByte := common.Hex2Bytes(key)
			valByte := common.Hex2Bytes(val)
			keyByte = append(crypto.Keccak256Hash(addr.Bytes()).Bytes(), keyByte...)
			batch.Set(keyByte, valByte)
			if count % 10000 == 0 {
				batch.Commit()
				batch = s.destDb.NewBatch()
			}
		}
		batch.Commit()
	}

	return nil
}

func (s *StateDump) commitKv() (common.Hash, error) {
	root, err := s.target.Commit(false, 0)
	if err != nil {
		return common.EmptyHash, err
	}
	s.target.Database().TrieDB().Commit(root, false)
	s.target.Reset(root)

	return root, nil
}

func (s *StateDump) checkDump(accountList map[common.Address]OldAccount) error {
	for addr, account := range accountList {
		checkBalance := s.target.GetBalance(addr)
		if checkBalance.Cmp(account.Balance) != 0 {
			fmt.Println("checkDump failed.", "addr", addr.Hex(), "srcBalance", account.Balance.String(),
				"checkBalance", checkBalance.String())
			return errors.New("checkDump failed.")
		}
		checkNonce := s.target.GetNonce(addr)
		if checkNonce != account.Nonce {
			fmt.Println("checkDump failed.", "addr", addr.Hex(), "srcNonce", account.Nonce,
				"checkNonce", checkNonce)
			return errors.New("checkDump failed.")
		}
		checkCodeHash := s.target.GetCodeHash(addr)
		if checkCodeHash.String() != common.BytesToHash(account.CodeHash).String() {
			fmt.Println("checkDump failed.", "addr", addr.Hex(), "srcCodeHash", common.BytesToHash(account.CodeHash).String(),
				"checkCodeHash", checkCodeHash.String())
			return errors.New("checkDump failed.")
		}
	}
	return nil
}

func (s *StateDump) saveFromMsgsForCheck(stateRoot common.Hash) error {
	typeStr := "from"
	fname := fmt.Sprintf("acc_%s_%s.log", typeStr, time.Now().Format("20060102-15-04-05"))
	fname = filepath.Join(os.TempDir(), fname)
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	stateDB, err := state.New(stateRoot, state.NewDatabase(s.db))
	if err != nil {
		fmt.Printf("Error %s\n", err)
		return nil
	}
	fmt.Println("start dump", typeStr, time.Now())
	accs := stateDB.JSONDumpOldTrie()
	fmt.Println("start dump", typeStr, time.Now())
	if len(accs) == 0 {
		fmt.Println("accs is empty")
		return nil
	}
	for _, str := range accs {
		if _, err := f.WriteString(str+"\n"); err != nil {
			return errors.New("saveMsgForCheck write string failed")
		}
	}

	return nil
}

func (s *StateDump) saveDumpMsgsForCheck(stateRoot common.Hash, isTrie bool) error {
	typeStr := "tr"
	if !isTrie {
		stateRoot = common.EmptyHash
		typeStr = "kv"
	}
	fname := fmt.Sprintf("acc_%s_%s.log", typeStr, time.Now().Format("20060102-15-04-05"))
	fname = filepath.Join(os.TempDir(), fname)
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	stateDB, err := state.New(stateRoot, state.NewKeyValueDBWithCache(s.destDb, 0, isTrie, 0))
	if err != nil {
		panic(err)
	}
	accs := make([]string, 0)
	fmt.Println("start dump", typeStr, time.Now())
	if isTrie {
		accs = stateDB.JSONDumpTrie()
	} else {
		accs = stateDB.JSONDumpKV()
	}
	fmt.Println("start dump", typeStr, time.Now())
	if len(accs) == 0 {
		fmt.Println("accs is empty")
		return nil
	}
	for _, str := range accs {
		if _, err := f.WriteString(str+"\n"); err != nil {
			return errors.New("saveMsgForCheck write string failed")
		}
	}

	return nil
}

func saveRootAndHeight(dumpPath string, stateRoot string, initBlockHeight string) error {
	stateRootPath := filepath.Join(dumpPath, "db", "full", "stateRoot.txt")
	rootFile, err := os.Create(stateRootPath)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(rootFile)
	defer writer.Flush()
	writer.WriteString(stateRoot)

	heightPath := filepath.Join(dumpPath, "db", "height.txt")
	heightFile, err := os.Create(heightPath)
	if err != nil {
		return err
	}
	writer = bufio.NewWriter(heightFile)
	defer writer.Flush()
	writer.WriteString(initBlockHeight)

	return nil
}