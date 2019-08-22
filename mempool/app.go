package mempool

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
)

//ErrTxNotATransaction ...
var ErrTxNotATransaction = fmt.Errorf("tx is not a transaction")

//BasicCheck for check signature in checkTX
const (
	BasicCheck = true
	StateCheck = false
)

type App interface {
	GetNonce(addr common.Address) uint64
	GetBalance(addr common.Address) *big.Int
	CheckTx(tx types.Tx, checkType bool) error
}

type mockApp struct {
	mtx      sync.Mutex
	nonce    map[common.Address]uint64
	nMtx     sync.RWMutex
	balance  map[common.Address]*big.Int
	bMtx     sync.RWMutex
	accounts []*keystore.Key
	mempool  *Mempool
}

func (mApp *mockApp) GetNonce(addr common.Address) uint64 {
	mApp.nMtx.RLock()
	v := mApp.nonce[addr]
	mApp.nMtx.RUnlock()
	return v
}

func (mApp *mockApp) GetBalance(addr common.Address) *big.Int {
	mApp.bMtx.RLock()
	v := mApp.balance[addr]
	mApp.bMtx.RUnlock()
	return v
}

func (mApp *mockApp) CheckTx(tx types.Tx, checkBasic bool) error {
	if !checkBasic {
		mApp.mtx.Lock()
		defer mApp.mtx.Unlock()
		switch eTx := tx.(type) {
		case *types.Transaction:
			from, _ := eTx.From()
			balance := mApp.GetBalance(from)
			nonce := mApp.GetNonce(from)
			cost := eTx.Cost()

			if cost.Cmp(balance) > 0 {
				fmt.Println("insufficcient balance", "from", from.String(), "cost", cost, "balance", balance)
				return types.ErrInsufficientFunds
			}
			if eTx.Nonce() < nonce {
				fmt.Printf("ErrNonceTooLow ,got:%v want:%v from:%v hash:%v\n", eTx.Nonce(), nonce, from.Hex(), tx.Hash().Hex())
				return types.ErrNonceTooLow
			} else if eTx.Nonce() > nonce {
				return types.ErrNonceTooHigh
			}
			mApp.nMtx.Lock()
			mApp.nonce[from]++
			mApp.nMtx.Unlock()

			mApp.bMtx.Lock()
			mApp.balance[from].Sub(mApp.balance[from], cost)
			mApp.balance[*eTx.To()].Add(mApp.balance[*eTx.To()], cost)
			mApp.bMtx.Unlock()
		case *types.MultiSignAccountTx:
			from, _ := eTx.From()
			nonce := mApp.GetNonce(from)

			if eTx.Nonce() < nonce {
				fmt.Printf("ErrNonceTooLow ,eTx:%v got:%v want:%v from:%v hash:%v\n", eTx, eTx.Nonce(), nonce, from.Hex(), tx.Hash().Hex())
				return types.ErrNonceTooLow
			} else if eTx.Nonce() > nonce {
				fmt.Printf("ErrNonceTooHigh ,eTx:%v,got:%v want:%v from:%v hash:%v\n", eTx, eTx.Nonce(), nonce, from.Hex(), tx.Hash().Hex())
				return types.ErrNonceTooHigh
			}
			mApp.nMtx.Lock()
			mApp.nonce[from]++
			mApp.nMtx.Unlock()
		case *types.UTXOTransaction:
			for _, txin := range eTx.Inputs {
				switch input := txin.(type) {
				case *types.UTXOInput:
					if mApp.mempool.KeyImageExists(input.KeyImage) {
						log.Debug("Key image already spent in other txs", "KeyImage", input.KeyImage, "hash", tx.Hash())
						return types.ErrUtxoTxDoubleSpend
					}
					log.Debug("checkState unspent", "KeyImage", input.KeyImage)
					mApp.mempool.KeyImagePush(input.KeyImage)
				case *types.AccountInput:
					//check nonce
					fromAddr, _ := tx.From()
					nonce := mApp.GetNonce(fromAddr)
					if nonce > input.Nonce {
						log.Debug("nonce too low", "got", input.Nonce, "want", nonce, "fromAddr", fromAddr, "txHash", tx.Hash())
						return types.ErrNonceTooLow
					} else if nonce < input.Nonce {
						return types.ErrNonceTooHigh
					}
					balance := mApp.GetBalance(fromAddr)
					//check balance
					if balance.Cmp(input.Amount) < 0 {
						return types.ErrInsufficientFunds
					}
					mApp.nMtx.Lock()
					mApp.nonce[fromAddr]++
					mApp.nMtx.Unlock()

					mApp.bMtx.Lock()
					mApp.balance[fromAddr].Sub(mApp.balance[fromAddr], input.Amount)
					mApp.bMtx.Unlock()
				default:
				}
			}
		}

	} else {
		switch eTx := tx.(type) {
		case *types.UTXOTransaction:
			if (eTx.UTXOKind() & types.AinAout) != types.IllKind {
				//AccountInput or call contract need account signature
				fromAddr, err := tx.From()
				if err != nil {
					log.Debug("CheckTx", "err", err)
					return types.ErrInvalidSig
				}
				eTx.StoreFrom(fromAddr)
			} else {
				eTx.StoreFrom(common.EmptyAddress)
			}
		}
	}
	return nil
}
