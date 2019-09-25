package wallet

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/common"
	lkctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	tctypes "github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/wallet/types"
)

const (
	keyLocalHeight      = "localHeight"
	keyUtxoTotalBalance = "utxoTotalBalance"
	keyGOutIndex        = "gOutIndex"
	keyAccBalance       = "AccBalance"
	keyTransfers        = "transfers"
	keyTransfersCnt     = "transfersCnt"
	keyAccountSubCnt    = "accountSubCnt"
	keyTxKeys           = "txKeys"
	keyUTXOTx           = "utxoTx"
	keyBlockHash        = "blockHash"
	keyBlockTxs         = "blockTxs"
	keyUTXOAddInfo      = "utxoAddInfo"
)

func (la *LinkAccount) save(ids []uint64, blockHash common.Hash, localBlock *types.UTXOBlock) error {
	batch := la.walletDB.NewBatch()

	if la.saveLocalHeight(batch) != nil ||
		la.saveGOutIndex(batch) != nil ||
		la.saveAccountSubCnt(batch) != nil ||
		(len(ids) > 0 && la.saveTransfers(batch, ids) != nil) ||
		la.saveBlockHash(batch, la.localHeight, blockHash) != nil ||
		la.saveBlockTxs(batch, localBlock) != nil {
		la.Logger.Error("Refresh batchSave fail", "height", la.localHeight)
		return fmt.Errorf("save fail")
	}
	return batch.Commit()
}

func (la *LinkAccount) addPrefixDBkey(key string) string {
	prefix := la.mainUTXOAddress
	return fmt.Sprintf("%s_%s", prefix, key)
}

// localHeight
func (la *LinkAccount) getLocalHeightKey() []byte {
	return []byte(la.addPrefixDBkey(keyLocalHeight))
}

func (la *LinkAccount) loadLocalHeight() error {
	key := la.getLocalHeightKey()

	val := la.walletDB.Get(key[:])
	if len(val) != 0 {
		if err := ser.DecodeBytes(val, &la.localHeight); err != nil {
			la.Logger.Error("loadLocalHeight DecodeBytes fail", "val", string(val), "err", err)
			return err
		}
		// set next height
		la.localHeight.Add(la.localHeight, big.NewInt(1))
	}
	la.Logger.Debug("loadLocalHeight", "la.localHeight", la.localHeight)
	return nil
}

func (la *LinkAccount) saveLocalHeight(b dbm.Batch) error {
	key := la.getLocalHeightKey()
	val, err := ser.EncodeToBytes(la.localHeight)
	if err != nil {
		la.Logger.Error("saveLocalHeight EncodeToBytes fail", "err", err)
		return err
	}
	b.Set(key, val)
	return nil
}

// gOutIndex
func (la *LinkAccount) getGOutIndexKey() []byte {
	return []byte(la.addPrefixDBkey(keyGOutIndex))
}

func (la *LinkAccount) loadGOutIndex() error {
	key := la.getGOutIndexKey()

	val := la.walletDB.Get(key[:])
	if len(val) != 0 {
		if err := json.Unmarshal(val, &la.gOutIndex); err != nil {
			la.Logger.Error("loadGOutIndex DecodeBytes fail", "val", string(val), "err", err)
			return err
		}
	}
	return nil
}

func (la *LinkAccount) saveGOutIndex(b dbm.Batch) error {
	key := la.getGOutIndexKey()
	// val, err := ser.EncodeToBytes(la.gOutIndex)
	// val, err := ser.MarshalJSON(la.gOutIndex)
	val, err := json.Marshal(la.gOutIndex)
	if err != nil {
		la.Logger.Error("saveGOutIndex EncodeToBytes fail", "err", err)
		return err
	}
	b.Set(key, val)
	return nil
}

// transfers
func (la *LinkAccount) getTransfersKey(idx uint64) []byte {
	return []byte(fmt.Sprintf("%s_%d", la.addPrefixDBkey(keyTransfers), idx))
}

func (la *LinkAccount) getTransfersCntKey() []byte {
	return []byte(la.addPrefixDBkey(keyTransfersCnt))
}

func (la *LinkAccount) loadTransfers() error {
	var cnt uint64
	key := la.getTransfersCntKey()
	val := la.walletDB.Get(key[:])
	if len(val) != 0 {
		if err := ser.UnmarshalJSON(val, &cnt); err != nil {
			la.Logger.Error("loadTransfers DecodeBytes fail", "val", string(val), "err", err)
			return err
		}
	}
	la.Transfers = make(transferContainer, cnt)
	// accCnt := len(la.account.Keys)
	// la.AccBalance = make(map[common.Address]balanceVec)

	// for i := 0; i < accCnt; i++ {
	// 	la.AccBalance[i] = big.NewInt(0)
	// }
	for i := uint64(0); i < cnt; i++ {
		k := la.getTransfersKey(uint64(i))
		v := la.walletDB.Get(k[:])
		if len(v) != 0 {
			var tx tctypes.UTXOOutputDetail
			if err := ser.UnmarshalJSON(v, &tx); err != nil {
				la.Logger.Error("loadTransfers DecodeBytes fail", "val", string(val), "err", err)
				return err
			}
			la.Transfers[i] = &tx

			if !tx.Spent && !tx.Frozen {
				la.updateBalance(tx.TokenID, tx.SubAddrIndex, true, tx.Amount)
			}

			la.keyImages[tx.KeyImage] = i
		}
	}
	return nil
}

func (la *LinkAccount) saveTransfers(b dbm.Batch, tids []uint64) error {
	key := la.getTransfersCntKey()
	cnt := len(la.Transfers)
	val, err := ser.MarshalJSON(cnt)
	if err != nil {
		la.Logger.Error("saveTransfers EncodeToBytes fail", "err", err)
		return err
	}
	b.Set(key, val)

	for _, i := range tids {
		k := la.getTransfersKey(i)
		val, err := ser.MarshalJSON(la.Transfers[i])
		if err != nil {
			la.Logger.Error("saveTransfers EncodeToBytes fail", "err", err)
			return err
		}
		b.Set(k, val)
	}
	return nil
}

func (la *LinkAccount) loadOutputDetail(id uint64) (*tctypes.UTXOOutputDetail, error) {
	k := la.getTransfersKey(id)
	v := la.walletDB.Get(k[:])
	if len(v) != 0 {
		var tx tctypes.UTXOOutputDetail
		if err := ser.UnmarshalJSON(v, &tx); err != nil {
			la.Logger.Error("loadTransfers DecodeBytes fail", "val", string(v), "err", err)
			return nil, err
		}
		return &tx, nil
	}
	return nil, types.ErrOutputNotFound
}

// keyAccountSubCnt
func (la *LinkAccount) getAccountSubCntKey() []byte {
	return []byte(la.addPrefixDBkey(keyAccountSubCnt))
}

func (la *LinkAccount) loadAccountSubCnt() (int, error) {
	key := la.getAccountSubCntKey()

	val := la.walletDB.Get(key[:])
	if len(val) != 0 {
		var cnt int
		if err := ser.UnmarshalJSON(val, &cnt); err != nil {
			la.Logger.Error("loadGOutIndex DecodeBytes fail", "val", string(val), "err", err)
			return cnt, err
		}
		return cnt, nil
	}
	return 0, nil
}

func (la *LinkAccount) saveAccountSubCnt(b dbm.Batch) error {
	key := la.getAccountSubCntKey()
	if la.account == nil || la.account.Keys == nil {
		return nil
	}
	accountSubCnt := len(la.account.Keys)
	val, err := ser.MarshalJSON(accountSubCnt)
	if err != nil {
		la.Logger.Error("saveAccountSubCnt EncodeToBytes fail", "err", err)
		return err
	}
	b.Set(key, val)
	return nil
}

// keyTxKeys
func (la *LinkAccount) getTxKeysKey() []byte {
	return []byte(la.addPrefixDBkey(keyTxKeys))
}

/*
func (la *LinkAccount) loadTxKeys() error {
	key := la.getTxKeysKey()

	val := la.walletDB.Get(key[:])
	if len(val) != 0 {
		if err := json.Unmarshal(val, &la.txKeys); err != nil {
			la.Logger.Error("loadTxKeys DecodeBytes fail", "val", string(val), "err", err)
			return err
		}
		return nil
	}
	return nil
}
*/

func (la *LinkAccount) saveTxKeys(hash common.Hash, txKey *lkctypes.Key) error {
	/*
		la.lock.Lock()
		defer la.lock.Unlock()

		la.txKeys[hash] = *txKey
		la.Logger.Debug("saveTxKeys", "hash", hash, "txKey", *txKey)

		key := la.getTxKeysKey()
		if la.txKeys == nil || len(la.txKeys) == 0 {
			return nil
		}
		val, err := json.Marshal(la.txKeys)
		if err != nil {
			la.Logger.Error("saveTxKeys EncodeToBytes fail", "err", err)
			return err
		}
		batch := la.walletDB.NewBatch()
		batch.Set(key, val)
		return batch.Commit()
	*/

	key := make([]byte, 0, 128)
	key = append(key, hash[:]...)
	key = append(key, byte('_'))
	key = append(key, la.getTxKeysKey()...)
	return la.walletDB.Put(key, txKey[:])
}

// UTXOTransaction
func (la *LinkAccount) getUTXOTxKey(hash common.Hash) []byte {
	return []byte(fmt.Sprintf("%s_%s", la.addPrefixDBkey(keyUTXOTx), hash.String()))
}

func (la *LinkAccount) loadUTXOTx(hash common.Hash) (*tctypes.UTXOTransaction, error) {
	key := la.getUTXOTxKey(hash)
	val := la.walletDB.Get(key[:])
	if len(val) == 0 {
		return nil, types.ErrTxNotFound
	}
	var utxoTx tctypes.UTXOTransaction
	if err := ser.DecodeBytes(val, &utxoTx); err != nil {
		la.Logger.Error("loadUTXOTx DecodeBytes fail", "val", string(val), "err", err)
		return nil, err
	}
	return &utxoTx, nil
}
func (la *LinkAccount) saveUTXOTx(utxoTx *tctypes.UTXOTransaction) error {
	la.Logger.Debug("saveUTXOTx", "hash", utxoTx.Hash(), "utxoTx", utxoTx)
	key := la.getUTXOTxKey(utxoTx.Hash())
	val, err := ser.EncodeToBytes(utxoTx)
	if err != nil {
		la.Logger.Error("saveTxKeys EncodeToBytes fail", "err", err)
		return err
	}
	batch := la.walletDB.NewBatch()
	batch.Set(key, val)
	return batch.Commit()
}

// block hash
func (la *LinkAccount) getBlockHashKey(height *big.Int) []byte {
	return []byte(fmt.Sprintf("%s_%s", la.addPrefixDBkey(keyBlockHash), height.String()))
}

func (la *LinkAccount) loadBlockHash(height *big.Int) (*common.Hash, error) {
	if height.Cmp(new(big.Int).SetUint64(defaultInitBlockHeight)) < 0 {
		return &common.EmptyHash, nil
	}
	key := la.getBlockHashKey(height)
	val := la.walletDB.Get(key[:])
	if len(val) == 0 {
		return nil, types.ErrBlockNotFound
	}
	var hash common.Hash
	if err := ser.DecodeBytes(val, &hash); err != nil {
		la.Logger.Error("loadBlockHash DecodeBytes fail", "val", string(val), "height", height.String(), "err", err)
		return nil, err
	}
	return &hash, nil
}
func (la *LinkAccount) saveBlockHash(b dbm.Batch, height *big.Int, hash common.Hash) error {
	key := la.getBlockHashKey(height)
	val, err := ser.EncodeToBytes(hash)
	if err != nil {
		la.Logger.Error("saveBlockHash EncodeToBytes fail", "height", height.String(), "hash", hash, "err", err)
		return err
	}
	la.Logger.Debug("saveBlockHash", "height", height.String(), "hash", hash)
	b.Set(key, val)
	return nil
}

// blockTx
func (la *LinkAccount) getBlockTxsKey(height *big.Int) []byte {
	return []byte(fmt.Sprintf("%s_%s", la.addPrefixDBkey(keyBlockTxs), height.String()))
}

func (la *LinkAccount) loadBlockTxs(height *big.Int) (*types.UTXOBlock, error) {
	key := la.getBlockTxsKey(height)
	val := la.walletDB.Get(key[:])
	if len(val) == 0 {
		return nil, types.ErrBlockNotFound
	}
	var block types.UTXOBlock
	// if err := ser.DecodeBytes(val, &block); err != nil {
	if err := json.Unmarshal(val, &block); err != nil {
		la.Logger.Error("loadBlockTxs DecodeBytes fail", "val", string(val), "err", err)
		return nil, err
	}
	//skip self account-> other utxo and other utxo-> self account
	//because these trans already show in account trans list
	txs := make([]types.UTXOTransaction, 0)
	for _, trans := range block.Txs {
		var flag uint8
		if (trans.TxFlag & txUout) == txUout {
			flag = flag | rpcUOut
		}
		if (trans.TxFlag & txUin) == txUin {
			flag = flag | rpcUIn
		}
		if flag > 0 {
			trans.TxFlag = flag
			txs = append(txs, trans)
		}
	}
	block.Txs = txs
	return &block, nil
}
func (la *LinkAccount) saveBlockTxs(b dbm.Batch, localBlock *types.UTXOBlock) error {
	key := la.getBlockTxsKey(localBlock.Height.ToInt())
	val, err := json.Marshal(localBlock)
	if err != nil {
		la.Logger.Error("saveBlockTxs EncodeToBytes fail", "err", err)
		return err
	}
	b.Set(key, val)
	return nil
}

func (la *LinkAccount) getAddInfoKey(hash common.Hash) []byte {
	return []byte(fmt.Sprintf("%s_%s", la.addPrefixDBkey(keyUTXOAddInfo), hash.String()))
}

func (la *LinkAccount) loadAddInfo(hash common.Hash) (*types.UTXOAddInfo, error) {
	key := la.getAddInfoKey(hash)
	val := la.walletDB.Get(key[:])
	if len(val) == 0 {
		return nil, types.ErrAddInfoNotFound
	}
	var addInfo types.UTXOAddInfo
	if err := json.Unmarshal(val, &addInfo); err != nil {
		la.Logger.Error("loadAddInfo json.Unmarshal fail", "val", string(val), "err", err)
		return nil, err
	}
	return &addInfo, nil
}

func (la *LinkAccount) saveAddInfo(hash common.Hash, addInfo *types.UTXOAddInfo) error {
	key := la.getAddInfoKey(hash)
	val, err := json.Marshal(addInfo)
	if err != nil {
		la.Logger.Error("saveAddInfo json.Marshal fail", "err", err)
		return err
	}
	batch := la.walletDB.NewBatch()
	batch.Set(key, val)
	return batch.Commit()
}

func (la *LinkAccount) delAddInfo(hash common.Hash) error {
	key := la.getAddInfoKey(hash)
	return la.walletDB.Del(key)
}
