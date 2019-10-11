package wallet

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"sort"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
	wtypes "github.com/lianxiangcloud/linkchain/wallet/types"
)

const (
	UTXOTRANSACTION_MAX_SIZE     = 1024 * 20
	UTXOTRANSACTION_FEE_MAX_SIZE = 1024 * 28
	UTXOTRANSACTION_FEE          = 1e13
	UTXO_SIMPLE_RING_SIZE        = 1
	UTXO_DEFAULT_RING_SIZE       = 11
	UTXO_OUTPUT_QUERY_PAGESIZE   = 2   //2*UTXO_DEFAULT_RING_SIZE
	ACCOUNT_TRANS_FIXED_FEE_RATE = 200 //0.005
)

type InputMode uint8

const (
	AccountInputMode InputMode = 0
	UTXOInputMode    InputMode = 1
	MixInputMode     InputMode = 2
)

type OutKind uint8

const (
	NilOut  OutKind = 0
	AccOut  OutKind = 1
	UtxoOut OutKind = 2
)

//CreateUTXOTransaction -none
func (wallet *Wallet) CreateUTXOTransaction(from common.Address, nonce uint64, subaddrs []uint64, dests []types.DestEntry,
	tokenID common.Address, refundAddr common.Address, extra []byte) ([]*types.UTXOTransaction, error) {
	if wallet.IsWalletClosed() {
		return nil, wtypes.ErrWalletNotOpen
	}
	currAccount, err := wallet.getCurrAccount(from)
	if err != nil {
		return nil, err
	}
	if from == common.EmptyAddress {
		wallet.Logger.Debug("CreateUTXOTransaction from is EmptyAddress,use CreateUinTransaction")
		return wallet.CreateUinTransaction(currAccount.getEthAddress(), subaddrs, dests, tokenID, extra)
	}
	wallet.Logger.Debug("CreateUTXOTransaction from is not EmptyAddress,use CreateAinTransaction", "from", from, "nonce", nonce, "tokenID", tokenID)
	tx, err := wallet.CreateAinTransaction(from, "", nonce, dests, tokenID, extra)
	if err != nil {
		return nil, err
	}
	return []*types.UTXOTransaction{tx}, nil
}

//SubmitUTXOTransactions -none
func (wallet *Wallet) SubmitUTXOTransactions(txes []*types.UTXOTransaction) ([]common.Hash, error) {
	if 0 == len(txes) {
		return nil, nil
	}
	hashes := make([]common.Hash, 0)
	for i := 0; i < len(txes); i++ {
		hash, err := wallet.SubmitUTXOTransaction(txes[i])
		if err != nil {
			return hashes, err
		}
		hashes = append(hashes, hash)
	}
	return hashes, nil
}

type submitRawTxResp struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Result  string      `json:"result"`
	Error   interface{} `json:"error,omitempty"`
}

//SubmitUTXOTransaction -none
func (wallet *Wallet) SubmitUTXOTransaction(tx *types.UTXOTransaction) (common.Hash, error) {
	if tx == nil {
		return common.Hash{}, nil
	}
	rawTx, err := ser.EncodeToBytes(tx)
	if err != nil {
		return common.Hash{}, wtypes.ErrInnerServer
	}
	raw := hex.EncodeToString(rawTx)
	ret := wallet.api.Transfer([]string{fmt.Sprintf("0x%s", raw)})
	err = nil
	if ret[0].ErrCode != 0 {
		//err = fmt.Errorf("ErrCode:%d,ErrMsg:%s", ret[0].ErrCode, ret[0].ErrMsg)
		err = wtypes.ErrSubmitTrans
	}
	return ret[0].Hash, err
}

func (wallet *Wallet) unspentBalancePerSubaddr(from common.Address, tokenID common.Address) (map[uint64]*big.Int, error) {
	currAccount, err := wallet.getCurrAccount(from)
	if err != nil {
		return nil, err
	}
	balancePerSubaddr := make(map[uint64]*big.Int)
	for i := 0; i < len(currAccount.Transfers); i++ {
		output := currAccount.Transfers[i]
		// wallet.Logger.Debug("unspentBalancePerSubaddr", "tokenid", output.TokenID, "spent", output.Spent, "frozen", output.Frozen, "amount", output.Amount.String())
		if output.TokenID == tokenID && !output.Spent && !output.Frozen {
			if balance, exist := balancePerSubaddr[output.SubAddrIndex]; exist {
				balancePerSubaddr[output.SubAddrIndex].Add(balance, output.Amount)
			} else {
				balancePerSubaddr[output.SubAddrIndex] = big.NewInt(0).Set(output.Amount)
			}
		}
	}
	return balancePerSubaddr, nil
}

func (wallet *Wallet) unspentIndicePerSubaddr(from common.Address, tokenID common.Address) (map[uint64][]uint64, error) {
	currAccount, err := wallet.getCurrAccount(from)
	if err != nil {
		return nil, err
	}
	indicePerSubaddr := make(map[uint64][]uint64)
	for i := 0; i < len(currAccount.Transfers); i++ {
		output := currAccount.Transfers[i]
		if output.TokenID == tokenID && !output.Spent && !output.Frozen {
			if _, exist := indicePerSubaddr[output.SubAddrIndex]; exist {
				indicePerSubaddr[output.SubAddrIndex] = append(indicePerSubaddr[output.SubAddrIndex], uint64(i))
			} else {
				indicePerSubaddr[output.SubAddrIndex] = []uint64{uint64(i)}
			}
		}
	}
	return indicePerSubaddr, nil
}

func (wallet *Wallet) constructSourceEntry(from common.Address, selectIndice []uint64, tokenID common.Address) ([]*types.UTXOSourceEntry, error) {
	if 0 == len(selectIndice) {
		return nil, nil
	}
	str, _ := ser.MarshalJSON(selectIndice)
	wallet.Logger.Debug("constructSourceEntry", "selectIndice", str)

	maxIdx := wallet.getGOutIndex(tokenID)

	rings, err := wallet.constructRings(from, maxIdx, UTXO_DEFAULT_RING_SIZE, selectIndice)
	if err != nil {
		return nil, err
	}
	if rings == nil {
		return wallet.constructSourceEntrySimple(from, selectIndice, tokenID)
	}
	for idx, ring := range rings {
		str, _ = ser.MarshalJSON(ring)
		wallet.Logger.Debug("constructSourceEntry", "index", idx, "ring", str)
	}
	return wallet.constructSourceEntryNormal(from, selectIndice, rings, tokenID)
}

//TODO check performance
func (wallet *Wallet) constructRings(from common.Address, maxIdx uint64, ringSize int,
	selectIndice []uint64) (map[uint64]ring, error) {
	if uint64(len(selectIndice)*ringSize) > maxIdx {
		return nil, nil
	}
	currAccount, err := wallet.getCurrAccount(from)
	if err != nil {
		return nil, err
	}
	rings := make(map[uint64]ring)
	excluded := make(map[uint64]bool)
	for _, selectIdx := range selectIndice {
		gIdx := currAccount.Transfers[selectIdx].GlobalIndex
		rings[selectIdx] = ring{gIdx}
		excluded[gIdx] = true
		for {
			if ringSize == len(rings[selectIdx]) {
				break
			}
			ridx := uint64(rand.Int63n(int64(maxIdx + 1)))
			if _, exist := excluded[ridx]; exist {
				continue
			}
			excluded[ridx] = true
			rings[selectIdx] = append(rings[selectIdx], ridx)
		}
		sort.Sort(rings[selectIdx])
	}
	return rings, nil
}

func (wallet *Wallet) constructSourceEntrySimple(from common.Address, selectIndice []uint64, tokenID common.Address) ([]*types.UTXOSourceEntry, error) {
	wallet.Logger.Debug("constructSourceEntrySimple")
	currAccount, err := wallet.getCurrAccount(from)
	if err != nil {
		return nil, err
	}
	gIndice := make([]uint64, 0)
	for _, selectIdx := range selectIndice {
		gIdx := currAccount.Transfers[selectIdx].GlobalIndex
		gIndice = append(gIndice, gIdx)
	}
	ringEntries, err := wallet.api.GetOutputsFromNode(gIndice, tokenID)
	if err != nil {
		wallet.Logger.Error("constructSourceEntrySimple GetOutputsFromNode fail", "err", err)
		return nil, err
	}
	sources := make([]*types.UTXOSourceEntry, len(selectIndice))
	for i := 0; i < len(selectIndice); i++ {
		output := currAccount.Transfers[selectIndice[i]]
		sourceEntry := &types.UTXOSourceEntry{
			Ring:      make([]types.UTXORingEntry, 0),
			RingIndex: 0,
			RKey:      output.RKey,
			OutIndex:  output.OutIndex,
			Amount:    big.NewInt(0).Set(output.Amount),
			Mask:      output.Mask,
		}
		sourceEntry.Ring = append(sourceEntry.Ring, *ringEntries[i])
		sources[i] = sourceEntry
	}
	return sources, nil
}

func (wallet *Wallet) constructSourceEntryNormal(from common.Address, selectIndice []uint64,
	rings map[uint64]ring, tokenID common.Address) ([]*types.UTXOSourceEntry, error) {
	currAccount, err := wallet.getCurrAccount(from)
	if err != nil {
		return nil, err
	}
	var (
		start   = 0
		end     = 0
		sources = make([]*types.UTXOSourceEntry, len(selectIndice))
	)
	wallet.Logger.Debug("constructSourceEntryNormal")
	for {
		start = end
		end += UTXO_OUTPUT_QUERY_PAGESIZE
		if start >= len(selectIndice) {
			break
		}
		if end >= len(selectIndice) {
			end = len(selectIndice)
		}
		indice := make([]uint64, 0)
		for i := start; i < end; i++ {
			r := rings[selectIndice[i]]
			for j := 0; j < len(r); j++ {
				indice = append(indice, r[j])
			}
		}
		ringEntries, err := wallet.api.GetOutputsFromNode(indice, tokenID)
		if err != nil {
			wallet.Logger.Error("constructSourceEntryNormal GetOutputsFromNode fail", "err", err)
			return nil, err
		}
		for i := start; i < end; i++ {
			output := currAccount.Transfers[selectIndice[i]]
			r := rings[selectIndice[i]]
			sigleRingEntries := ringEntries[:UTXO_DEFAULT_RING_SIZE]
			ringEntries = ringEntries[UTXO_DEFAULT_RING_SIZE:]
			sourceEntry := &types.UTXOSourceEntry{
				Ring:      make([]types.UTXORingEntry, 0),
				RingIndex: 0,
				RKey:      output.RKey,
				OutIndex:  output.OutIndex,
				Amount:    big.NewInt(0).Set(output.Amount),
				Mask:      output.Mask,
			}
			for j := 0; j < len(sigleRingEntries); j++ {
				if sigleRingEntries[j].Index != r[j] {
					return nil, wtypes.ErrOutputQueryNotMatch
				}
				if sigleRingEntries[j].Index == output.GlobalIndex {
					sourceEntry.RingIndex = uint64(j)
				}
				sourceEntry.Ring = append(sourceEntry.Ring, *sigleRingEntries[j])
			}
			sources[i] = sourceEntry
		}
	}
	return sources, nil
}

type ring []uint64

func (r ring) Len() int {
	return len(r)
}
func (r ring) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}
func (r ring) Less(i, j int) bool {
	return r[i] < r[j]
}

//CreateAinTransaction return a UTXOTransaction for account input only
func (wallet *Wallet) CreateAinTransaction(from common.Address, passwd string, nonce uint64, dests []types.DestEntry,
	tokenID common.Address, extra []byte) (*types.UTXOTransaction, error) {
	needMoney, outKind, err := wallet.checkDest(dests, tokenID, AccountInputMode)
	if err != nil {
		wallet.Logger.Error("CreateAinTransaction checkDest fail", "err", err)
		return nil, err
	}
	wallet.Logger.Debug("CreateAinTransaction", "needMoney", needMoney.String())
	acc := accounts.Account{Address: from}
	w, err := wallet.accManager.Find(acc)
	if err != nil {
		wallet.Logger.Error("CreateAinTransaction wallet.accManager.Find fail", "acc", from, "err", err)
		return nil, wtypes.ErrAccountNotFound
	}
	availableMoney, err := wallet.api.GetTokenBalance(from, tokenID)
	if err != nil {
		wallet.Logger.Error("CreateAinTransaction getTokenBalance fail", "from", from, "tokenID", tokenID, "err", err)
		return nil, err
	}
	if availableMoney.Cmp(needMoney) < 0 {
		return nil, wtypes.ErrBalanceNotEnough
	}
	// check token fee available
	totalFee := big.NewInt(0)
	if !common.IsLKC(tokenID) {
		totalFee = wallet.calTokenFee(AccountInputMode, outKind)
		availableLkcMoney, err := wallet.api.GetTokenBalance(from, common.EmptyAddress)
		if err != nil {
			wallet.Logger.Error("CreateAinTransaction getTokenBalance fail", "from", from, "tokenID", tokenID, "err", err)
			return nil, err
		}
		if availableLkcMoney.Cmp(totalFee) <= 0 {
			return nil, wtypes.ErrTokenFeeNotEnough
		}
	}
	source := &types.AccountSourceEntry{
		From:   from,
		Nonce:  nonce,
		Amount: big.NewInt(0).Set(needMoney),
	}
	tx, txKey, err := types.NewAinTokenTransaction(source, dests, tokenID, totalFee, nil)
	if err != nil {
		return nil, wtypes.ErrNewAinTrans
	}

	var signedTx types.Tx
	if 0 == len(passwd) {
		signedTx, err = w.SignTx(acc, tx, types.SignParam)
		if err != nil {
			return nil, wtypes.ErrSignTx
		}
	} else {
		signedTx, err = w.SignTxWithPassphrase(acc, passwd, tx, types.SignParam)
		if err != nil {
			return nil, wtypes.ErrSignTx
		}
	}

	ainTx := signedTx.(*types.UTXOTransaction)

	// save txkey
	currAccount, err := wallet.getCurrAccount(from)
	if err != nil {
		return nil, err
	}
	err = currAccount.saveTxKeys(ainTx.Hash(), txKey)
	if err != nil {
		wallet.Logger.Error("CreateAinTransaction saveTxKeys fail", "err", err)
		return nil, err
	}

	return ainTx, nil
}

func (wallet *Wallet) checkDest(dests []types.DestEntry, tokenID common.Address, mode InputMode) (*big.Int, OutKind, error) {
	if 0 == len(dests) {
		return nil, NilOut, wtypes.ErrOutputEmpty
	}
	var (
		accTransMoney = big.NewInt(0)
		transferMoney = big.NewInt(0)
		needMoney     = big.NewInt(0)
		outKind       = NilOut
	)
	for i := 0; i < len(dests); i++ {
		if dests[i].GetAmount().Sign() <= 0 || dests[i].GetAmount().Cmp(big.NewInt(types.GetUtxoCommitmentChangeRate(tokenID))) < 0 ||
			big.NewInt(0).Mod(dests[i].GetAmount(), big.NewInt(types.GetUtxoCommitmentChangeRate(tokenID))).Sign() != 0 {
			return nil, NilOut, wtypes.ErrOutputMoneyInvalid
		}
		if types.TypeAcDest == dests[i].Type() {
			isContract, err := wallet.api.IsContract(dests[i].(*types.AccountDestEntry).To)
			if err != nil {
				return nil, NilOut, err
			}
			if isContract {
				return nil, NilOut, wtypes.ErrNotSupportContractTx
			}
			accTransMoney.Add(accTransMoney, dests[i].GetAmount())
		} else {
			outKind |= UtxoOut
		}
		transferMoney.Add(transferMoney, dests[i].GetAmount())
	}
	needMoney.Add(needMoney, transferMoney)
	if common.IsLKC(tokenID) {
		switch mode {
		case AccountInputMode:
			if transferMoney.Sign() > 0 {
				needMoney.Add(needMoney, wallet.estimateTxFee(transferMoney))
			}
		case UTXOInputMode:
			if accTransMoney.Sign() > 0 {
				needMoney.Add(needMoney, wallet.estimateTxFee(accTransMoney))
			}
			if (outKind & UtxoOut) != NilOut { //if only one account item, not add estimateUtxoTxFee
				needMoney.Add(needMoney, wallet.estimateUtxoTxFee())
			}
		case MixInputMode:
			//not support now
			return nil, NilOut, wtypes.ErrMixInputNotSupport
		}
	}
	return needMoney, outKind, nil
}

func (wallet *Wallet) calTokenFee(mode InputMode, outKind OutKind) *big.Int {
	fee := big.NewInt(0)

	switch mode {
	case AccountInputMode:
		fee.Add(fee, wallet.estimateTxFee(big.NewInt(0)))
	case UTXOInputMode:
		if (outKind & UtxoOut) != AccOut {
			fee.Add(fee, wallet.estimateTxFee(big.NewInt(0)))
		}
		if (outKind & UtxoOut) != NilOut { //if only one account item, not add estimateUtxoTxFee
			fee.Add(fee, wallet.estimateUtxoTxFee())
		}
	case MixInputMode:
		//not support now
	}
	return fee
}

//estimateTxFee envelopes types.CalNewAmountGas()
//limit account fee > 1e11 and tx fee mod 1e11 == 0
func (wallet *Wallet) estimateTxFee(transferMoney *big.Int) *big.Int {
	gasLimit := types.CalNewAmountGas(transferMoney, types.EverLiankeFee)
	return big.NewInt(0).Mul(big.NewInt(0).SetUint64(gasLimit), big.NewInt(types.GasPrice))
}

//estimateUtxoTxFee envelopes getter of wallet.utxoGas
//limit utxo fee > 1e11 and tx fee mod 1e11 == 0
func (wallet *Wallet) estimateUtxoTxFee() *big.Int {
	return wallet.utxoGas
}
