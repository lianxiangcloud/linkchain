package txmgr

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/lianxiangcloud/linkchain/libs/common"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
)

var (
	txEntryPrefix = []byte("Tx:")
)

var (
	logger = log.Root()
)

func init() {
	logger.SetHandler(log.StdoutHandler)
}

const (
	prefixMultiSigner = "multisign_"
)

//IBlockStore interface to avoid cycling reference.
type IBlockStore interface {
	LoadBlockCommit(height uint64) *types.Commit
	LoadBlock(height uint64) *types.Block
	GetTxFromBlock(block *types.Block, hash common.Hash) types.Tx
}

//Service manage special Txs.
type Service struct {
	db      dbm.DB
	prefixs []string
	bs      IBlockStore
	//cache

	msignersMap sync.Map //key:SupportType   value:signersInfo
}

// NewCrossState new a Service object with db blockstore.
func NewCrossState(db dbm.DB, bs IBlockStore) *Service {
	s := &Service{
		db: db,
		bs: bs,
		//cache:   make(map[common.Hash][]types.Tx),
		prefixs: make([]string, 0),
	}

	if err := s.loadDB(); err != nil {
		panic(err)
	}
	return s
}

func (s *Service) loadDB() (err error) {
	logger.Info("txmgr loadDB started")

	if err = s.loadMultiSigners(); err != nil {
		return
	}
	logger.Info("txmgr loadMultiSigners finished")

	logger.Info("txmgr loadDB finished")
	return
}

func (s *Service) SetLogger(l log.Logger) {
	logger = l
}

func (s *Service) NewDbBatch() dbm.Batch {
	return s.db.NewBatch()
}

func (s *Service) loadMultiSigners() error {
	prefix := []byte(prefixMultiSigner)
	r := util.BytesPrefix(prefix)
	itr := s.db.Iterator(r.Start, r.Limit)
	defer itr.Close()
	for ; itr.Valid(); itr.Next() {
		key := itr.Key()
		if !bytes.Equal(prefix, key[:len(prefix)]) {
			return fmt.Errorf("invalid iterator key: %s", string(key))
		}
		v := itr.Value()
		singerInfo := &types.SignersInfo{}
		if err := ser.DecodeBytes(v, singerInfo); err != nil {
			return fmt.Errorf("ser.DecodeBytes fail, err(%v) key(%s) val(%s)", err, string(key), string(v))
		}
		logger.Info("loadMultiSigners", "singerInfo.MinPower", singerInfo.MinSignerPower)
		for i := 0; i < len(singerInfo.Signers); i++ {
			logger.Info("Signers", "Addr", singerInfo.Signers[i].Addr, "Power", singerInfo.Signers[i].Power)
		}
		var txType types.SupportType
		switch string(key) {
		case types.DBupdateValidatorsKey:
			txType = types.TxUpdateValidatorsType
		case types.DBcontractCreateKey:
			txType = types.TxContractCreateType
		}
		s.msignersMap.Store(txType, singerInfo)
	}
	return nil
}

func (s *Service) saveMultiSignersInfo(tx *types.MultiSignAccountTx, batch dbm.Batch) error {
	logger.Info("saveMultiSignersInfo", "tx", tx)
	needSaveInfo := &types.SignersInfo{MinSignerPower: tx.MinSignerPower}
	needSaveInfo.Signers = make([]*types.SignerEntry, len(tx.Signers))
	for i := 0; i < len(needSaveInfo.Signers); i++ {
		needSaveInfo.Signers[i] = &types.SignerEntry{Power: tx.Signers[i].Power, Addr: tx.Signers[i].Addr}
	}

	value, err := ser.EncodeToBytes(needSaveInfo)
	if err != nil {
		logger.Error("saveMultiSignersInfo EncodeToBytes failed", "err", err)
		return err
	}
	var key []byte
	switch tx.SupportTxType {
	case types.TxUpdateValidatorsType:
		key = []byte(types.DBupdateValidatorsKey)
	case types.TxContractCreateType:
		key = []byte(types.DBcontractCreateKey)
	default:
		logger.Error("txType is not support", "txType", tx.SupportTxType)
	}
	batch.Set([]byte(key), value)
	s.msignersMap.Store(tx.SupportTxType, needSaveInfo)
	return nil
}

//GetMultiSignersInfo return the SignersInfo of txtype setted in MultiSignAccountTx
func (s *Service) GetMultiSignersInfo(txtype types.SupportType) *types.SignersInfo {
	v, ok := s.msignersMap.Load(txtype)
	if !ok {
		return nil
	}
	return v.(*types.SignersInfo)
}

func calcTxEntryKey(hash common.Hash) []byte {
	return append(txEntryPrefix, hash.Bytes()...)
}

// GetTxEntry return TxEntry from given hash
func (s *Service) GetTxEntry(hash common.Hash) *types.TxEntry {
	var entry = new(types.TxEntry)
	data := s.db.Get(calcTxEntryKey(hash))
	if len(data) == 0 {
		return nil
	}
	if err := ser.DecodeBytes(data, entry); err != nil {
		logger.Error("Invalid TxEntry SER", "hash", hash.Hex(), "err", err)
	}
	return entry
}

//SaveTxEntry save all txEntry of the block
func (s *Service) SaveTxEntry(block *types.Block, txsResult *types.TxsResult) {
	if s == nil {
		logger.Info("txmgr Save: service nil")
		return
	}
	hash := block.Hash()
	batch := s.db.NewBatch()
	defer batch.Commit()
	for i, tx := range block.Data.Txs {
		item := types.TxEntry{
			BlockHash:   hash,
			BlockHeight: block.Height,
			Index:       uint64(i),
		}
		data, err := ser.EncodeToBytes(item)
		if err != nil {
			cmn.PanicSanity(fmt.Sprintf("ser.EncodeToBytes TxEntyr err:%v", err))
		}
		logger.Debug("SaveTxEntry", "tx.Hash", tx.Hash(), "tx", tx)
		batch.Set(calcTxEntryKey(tx.Hash()), data)
	}
}

func (s *Service) DeleteTxEntry(block *types.Block) {
	bat := s.db.NewBatch()
	for _, tx := range block.Data.Txs {
		bat.Delete(calcTxEntryKey(tx.Hash()))
	}
	bat.Write()
}

//AddSpecialTx handles MultiSignAccountTx
func (s *Service) AddSpecialTx(txs []types.Tx) {
	batch := s.db.NewBatch()
	defer batch.Commit()
	for _, tx := range txs {
		switch specialtx := tx.(type) {
		case *types.MultiSignAccountTx:
			s.saveMultiSignersInfo(specialtx, batch)
		default:
		}
	}
}

//Sync flush data to disk.
func (s *Service) Sync() {
	s.db.SetSync(nil, nil)
}
