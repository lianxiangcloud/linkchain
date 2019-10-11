package utxo

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"

	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/types"
	lctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"sync"
)

const (
	tokenMaxUtxoOutputSeqKeyPre   = "token_muos_"
	blockTokenInitOutputSeqKeyPre = "btio_"
	kImageVal                     = "k"
	utxoOutputInitSequence uint64 = 1e19
	positionalNotation     int    = 36
)

type UtxoStore struct {
	utxoDB                   dbm.DB
	utxoOutputDB             dbm.DB
	utxoOutputTokenDB        dbm.DB
	maxUtxoOutputSeqTokenMap map[string]int64
	mapMutex                 sync.Mutex
	logger                   log.Logger
	blockHeight              uint64
}

type tokenUtxoSeqs struct {
	Seqs []*tokenUtxoSeq
}

type tokenUtxoSeq struct {
	TokenId string
	Seq     int64
}

func newTokenUtxoSeqs() *tokenUtxoSeqs {
	return &tokenUtxoSeqs{
		Seqs: make([]*tokenUtxoSeq, 0),
	}
}

func (t *tokenUtxoSeqs) addTokenUtxoSeq(tokenId string, seq int64) {
	tus := &tokenUtxoSeq{
		TokenId: tokenId,
		Seq:     seq,
	}
	t.Seqs = append(t.Seqs, tus)
}

func NewUtxoStore(utxoDB dbm.DB, utxoOutputDB dbm.DB, utxoOutputTokenDB dbm.DB) *UtxoStore {
	tokenMaxSeqMap := loadTokenUtxoStoreMaxUtxoOutputSeqMap(utxoDB)
	return &UtxoStore{
		utxoDB:                   utxoDB,
		utxoOutputDB:             utxoOutputDB,
		utxoOutputTokenDB:        utxoOutputTokenDB,
		maxUtxoOutputSeqTokenMap: tokenMaxSeqMap,
	}
}

func loadTokenUtxoStoreMaxUtxoOutputSeqMap(utxoDB dbm.DB) map[string]int64 {
	retMap := make(map[string]int64, 0)
	iter := utxoDB.NewIteratorWithPrefix([]byte(tokenMaxUtxoOutputSeqKeyPre))
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		maxSeq, err := strconv.ParseInt(string(val), positionalNotation, 64)
		if err != nil {
			panic(fmt.Sprintf("strconv.ParseInt failed: val:%s err=%s", string(val), err.Error()))
		}
		addressBytes := key[len(tokenMaxUtxoOutputSeqKeyPre):]
		retMap[string(addressBytes)] = maxSeq
	}
	return retMap
}

func (u *UtxoStore) GetMaxUtxoOutputSeq(tokenId common.Address) int64 {
	u.mapMutex.Lock()
	val, ok := u.maxUtxoOutputSeqTokenMap[tokenId.String()]
	u.mapMutex.Unlock()
	if !ok {
		return -1
	}
	return val
}

// Save current block token init output seq number.
func (u *UtxoStore) saveBlockTokenUtxoOutputSeq(tokenOutputSeqs *tokenUtxoSeqs) error {
	if len(tokenOutputSeqs.Seqs) == 0 {
		return nil
	}
	val, err := ser.EncodeToBytes(tokenOutputSeqs)
	if err != nil {
		u.logger.Error("ser encode to bytes failed.", "err", err.Error())
		return err
	}
	return u.utxoDB.Put(genBlockTokenInitSeq(u.blockHeight), val)
}

func (u *UtxoStore) GetBlockTokenUtxoOutputSeq(blockHeight uint64) map[string]int64 {
	tokenOutputSeqs := newTokenUtxoSeqs()
	val, err := u.utxoDB.Load(genBlockTokenInitSeq(blockHeight))
	if err != nil {
		u.logger.Info("get block Token utxo outputs seq failed.", "blockHeight", blockHeight,
			"err", err.Error())
		return nil
	}
	err = ser.DecodeBytes(val, tokenOutputSeqs)
	if err != nil {
		u.logger.Error("ser decode exec failed", "err", err.Error())
		return nil
	}
	retMap := make(map[string]int64, 0)
	for _, seqObj := range tokenOutputSeqs.Seqs {
		retMap[seqObj.TokenId] = seqObj.Seq
	}
	return retMap
}

func (u *UtxoStore) saveTokenUtxoOutputSeq(tokenSeqMap map[string]int64) error {
	tokenOutputSeqs := newTokenUtxoSeqs()
	u.mapMutex.Lock()
	for tokenId, seq := range tokenSeqMap {
		val := []byte(strconv.FormatInt(seq, positionalNotation))
		err := u.utxoDB.Put(genTokenMaxSeqKey(tokenId), val)
		if err != nil {
			u.mapMutex.Unlock()
			return err
		}
		initBlockSeq, ok := u.maxUtxoOutputSeqTokenMap[tokenId]
		if !ok {
			initBlockSeq = -1
		}
		tokenOutputSeqs.addTokenUtxoSeq(tokenId, initBlockSeq)
		u.maxUtxoOutputSeqTokenMap[tokenId] = seq
	}
	u.mapMutex.Unlock()
	err := u.saveBlockTokenUtxoOutputSeq(tokenOutputSeqs)
	if err != nil {
		u.logger.Error("save block tokend utxo outputs seq failed.", "err", err.Error())
		return err
	}

	return nil
}

func (u *UtxoStore) SetLogger(logger log.Logger) {
	u.logger = logger
}

func (u *UtxoStore) SaveUtxo(kImgs []*lctypes.Key, utxoOutputs []*types.UTXOOutputData, blockHeight uint64) error {
	u.blockHeight = blockHeight
	err := u.SaveKImages(kImgs)
	if err != nil {
		u.logger.Error("SaveKImages failed.", "err", err.Error())
		return err
	}
	err = u.SaveUtxoOutputs(utxoOutputs)
	if err != nil {
		u.logger.Error("SaveUtxoOutputs failed.", "err", err.Error())
	}

	return nil
}

func (u *UtxoStore) HaveTxKeyimgAsSpent(kImg *lctypes.Key) bool {
	val := u.utxoDB.Get(kImg[:])
	if len(val) != 0 {
		return true
	}
	return false
}

func (u *UtxoStore) SaveKImages(kImgs []*lctypes.Key) error {
	batch := u.utxoDB.NewBatch()
	for _, kImg := range kImgs {
		batch.Set(kImg[:], []byte(kImageVal))
	}
	return batch.Commit()
}

func (u *UtxoStore) SaveUtxoOutputs(utxoOutputs []*types.UTXOOutputData) error {
	tmpTokenSeq := make(map[string]int64, 0)
	batch      := u.utxoOutputDB.NewBatch()
	tokenBatch := u.utxoOutputTokenDB.NewBatch()
	for _, utxoOutput := range utxoOutputs {
		if _, ok := tmpTokenSeq[utxoOutput.TokenID.String()]; !ok {
			tmpTokenSeq[utxoOutput.TokenID.String()] = u.GetMaxUtxoOutputSeq(utxoOutput.TokenID)
		}
		nextSeqNum := utxoOutputInitSequence + uint64(tmpTokenSeq[utxoOutput.TokenID.String()]+1)
		nextSeqkey := []byte(strconv.FormatUint(nextSeqNum, positionalNotation))
		val, err := ser.EncodeToBytes(utxoOutput)
		if err != nil {
			u.logger.Error("ser EncodeToBytes exec failed.", "err", err.Error(),
				"key", string(nextSeqkey))
			return err
		}
		if utxoOutput.TokenID == common.EmptyAddress {
			batch.Set(nextSeqkey, val)
		} else {
			nextSeqkey := append(genTokenPreKey(utxoOutput.TokenID), nextSeqkey...)
			tokenBatch.Set(nextSeqkey, val)
		}
		tmpTokenSeq[utxoOutput.TokenID.String()]++
	}
	err := batch.Commit()
	if err != nil {
		u.logger.Error("SaveUtxoOutput db batch commit failed.", "err", err.Error())
		return err
	}
	err = tokenBatch.Commit()
	if err != nil {
		u.logger.Error("SaveUtxoOutput db token batch commit failed.", "err", err.Error())
		return err
	}
	// update maxUtxoOutputSeq
	err = u.saveTokenUtxoOutputSeq(tmpTokenSeq)
	if err != nil {
		u.logger.Error("UtxoStore saveMaxUtxoOutputSeq exec failed.", "err", err.Error())
		return err
	}

	return nil
}

func (u *UtxoStore) GetUtxoOutput(tokenId common.Address, seq uint64) (*types.UTXOOutputData, error) {
	key := []byte(strconv.FormatUint(utxoOutputInitSequence + seq, positionalNotation))
	var val []byte
	if tokenId == common.EmptyAddress {
		valByte, err := u.utxoOutputDB.Load(key)
		if err != nil {
			u.mapMutex.Lock()
			defer u.mapMutex.Unlock()
			u.logger.Error("GetUtxoOutput failed.", "err", err.Error(), "key", string(key),
				"seq", seq, "maxSeq", u.maxUtxoOutputSeqTokenMap[tokenId.String()])
			return nil, err
		}
		val = valByte
	} else {
		key = append(genTokenPreKey(tokenId), key...)
		valByte, err := u.utxoOutputTokenDB.Load(key)
		if err != nil {
			u.logger.Error("GetUtxoOutput failed.", "err", err.Error(), "key", string(key))
			return nil, err
		}
		val = valByte
	}
	if len(val) == 0 {
		u.logger.Error("GetUtxoOutput faield. key not found.", "key", string(key))
		return nil, errors.New("key not found")
	}
	utxoOutput := &types.UTXOOutputData{}
	err := ser.DecodeBytes(val, utxoOutput)
	if err != nil {
		u.logger.Error("ser decode exec failed.", "err", err.Error(), "key", string(key))
		return nil, err
	}
	return utxoOutput, nil
}

func (u *UtxoStore) GetUtxoOutputs(seqs []uint64, tokenId common.Address) ([]*types.UTXOOutputData, error) {
	if len(seqs) == 0 {
		u.logger.Error("GetUtxoOutputs failed. seqs[] length is 0")
		return nil, errors.New("seqs[] length is 0")
	}
	utxoOptputs := make([]*types.UTXOOutputData, 0)
	for _, seq := range seqs {
		utxoOptputData, err := u.GetUtxoOutput(tokenId, seq)
		if err != nil {
			return nil, err
		}
		utxoOptputs = append(utxoOptputs, utxoOptputData)
	}

	return utxoOptputs, nil
}

func (u *UtxoStore) GetRandomUtxoOutputs(counts int, tokenId common.Address) []*types.UTXOOutputData {
	u.mapMutex.Lock()
	if int64(counts) > u.maxUtxoOutputSeqTokenMap[tokenId.String()] + 1 {
		u.logger.Error("GetRandomUtxoOutputs failed. err: counts>maxUtxoOutputSeq",
			"count", counts, "maxUtxoOutputSeq", u.maxUtxoOutputSeqTokenMap[tokenId.String()])
		u.mapMutex.Unlock()
		return nil
	}
	u.mapMutex.Unlock()

	utxoOutputs    := make([]*types.UTXOOutputData, 0)
	utxoOutputsMap := make(map[string]*types.UTXOOutputData, 0)
	for ; len(utxoOutputs) < counts; {
		randKey := u.genRandKey(tokenId.String())
		if utxoOutputsMap[string(randKey)] != nil {
			continue
		}
		var val []byte
		if tokenId == common.EmptyAddress {
			valByte, err := u.utxoOutputDB.Load(randKey)
			if err != nil {
				u.logger.Error("GetRandomUtxoOutputs failed.", "err", err.Error(), "key", string(randKey))
				continue
			}
			val = valByte
		} else {
			fullkey := append(genTokenPreKey(tokenId), randKey...)
			valByte, err := u.utxoOutputTokenDB.Load(fullkey)
			if err != nil {
				u.logger.Error("GetRandomUtxoOutputs failed.", "err", err.Error(), "key", string(fullkey))
				continue
			}
			val = valByte
		}
		if len(val) == 0 {
			panic(fmt.Sprintf("GetRandomUtxoOutputs failed. key:%s not found.", string(randKey)))
		}
		utxoOutput := &types.UTXOOutputData{}
		err := ser.DecodeBytes(val, utxoOutput)
		if err != nil {
			u.logger.Error("GetRandomUtxoOutputs: ser decode exec failed.", "err", err.Error())
			continue
		}
		utxoOutputsMap[string(randKey)] = utxoOutput
		utxoOutputs = append(utxoOutputs, utxoOutput)
	}
	return utxoOutputs
}

func (u *UtxoStore) genRandKey(tokenId string) []byte {
	u.mapMutex.Lock()
	defer u.mapMutex.Unlock()
	randSeq := (common.RandInt64() % (u.maxUtxoOutputSeqTokenMap[tokenId] + 1))
	return []byte(strconv.FormatUint(utxoOutputInitSequence+uint64(randSeq), positionalNotation))
}

func genTokenMaxSeqKey(tokenId string) []byte {
	return []byte(tokenMaxUtxoOutputSeqKeyPre+tokenId)
}

func genTokenPreKey(tokenId common.Address) []byte {
	return []byte(tokenId.String()+":")
}

func genBlockTokenInitSeq(blockHeight uint64) []byte {
	return []byte(fmt.Sprintf("%s%d", blockTokenInitOutputSeqKeyPre, blockHeight))
}
