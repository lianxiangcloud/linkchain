package wallet

import (
	"encoding/json"
	"fmt"

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
)

func (la *LinkAccount) save(ids []int) error {
	batch := la.walletDB.NewBatch()

	if la.saveLocalHeight(batch) != nil ||
		la.saveGOutIndex(batch) != nil ||
		la.saveAccountSubCnt(batch) != nil ||
		(len(ids) > 0 && la.saveTransfers(batch, ids) != nil) {
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
func (la *LinkAccount) getTransfersKey(idx int) []byte {
	return []byte(fmt.Sprintf("%s_%d", la.addPrefixDBkey(keyTransfers), idx))
}

func (la *LinkAccount) getTransfersCntKey() []byte {
	return []byte(la.addPrefixDBkey(keyTransfersCnt))
}

func (la *LinkAccount) loadTransfers() error {
	var cnt int
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
	for i := 0; i < cnt; i++ {
		k := la.getTransfersKey(i)
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

func (la *LinkAccount) saveTransfers(b dbm.Batch, tids []int) error {
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
