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

type subaddrBalance struct {
	Subaddr uint64
	Balance *big.Int
}

type sortableSubaddrs []*subaddrBalance

func (ss sortableSubaddrs) Len() int {
	return len(ss)
}
func (ss sortableSubaddrs) Swap(i, j int) {
	ss[i], ss[j] = ss[j], ss[i]
}
func (ss sortableSubaddrs) Less(i, j int) bool {
	return ss[i].Balance.Cmp(ss[j].Balance) > 0
}

//CreateUTXOTransaction -none
func (wallet *Wallet) CreateUTXOTransaction(from common.Address, nonce uint64, subaddrs []uint64, dests []types.DestEntry,
	tokenID common.Address, refundAddr common.Address, extra []byte) ([]*types.UTXOTransaction, error) {
	if wallet.IsWalletClosed() {
		return nil, wtypes.ErrWalletNotOpen
	}
	if from == common.EmptyAddress {
		wallet.Logger.Debug("CreateUTXOTransaction from is EmptyAddress,use CreateUinTransaction")
		return wallet.CreateUinTransaction(wallet.currAccount.getEthAddress(), "", subaddrs, dests, tokenID, refundAddr, extra)
	}
	if from != wallet.currAccount.getEthAddress() {
		//return nil, fmt.Errorf("Wallet account is not open as %s", from)
		return nil, wtypes.ErrAccountNeedUnlock
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
	ret := wallet.Transfer([]string{fmt.Sprintf("0x%s", raw)})
	err = nil
	if ret[0].ErrCode != 0 {
		//err = fmt.Errorf("ErrCode:%d,ErrMsg:%s", ret[0].ErrCode, ret[0].ErrMsg)
		err = wtypes.ErrSubmitTrans
	}
	return ret[0].Hash, err
}

func (wallet *Wallet) unspentBalancePerSubaddr(tokenID common.Address) map[uint64]*big.Int {
	balancePerSubaddr := make(map[uint64]*big.Int)
	for i := 0; i < len(wallet.currAccount.Transfers); i++ {
		output := wallet.currAccount.Transfers[i]
		// wallet.Logger.Debug("unspentBalancePerSubaddr", "tokenid", output.TokenID, "spent", output.Spent, "frozen", output.Frozen, "amount", output.Amount.String())
		if output.TokenID == tokenID && !output.Spent && !output.Frozen {
			if balance, exist := balancePerSubaddr[output.SubAddrIndex]; exist {
				balancePerSubaddr[output.SubAddrIndex].Add(balance, output.Amount)
			} else {
				balancePerSubaddr[output.SubAddrIndex] = big.NewInt(0).Set(output.Amount)
			}
		}
	}
	return balancePerSubaddr
}

func (wallet *Wallet) unspentIndicePerSubaddr(tokenID common.Address) map[uint64][]uint64 {
	indicePerSubaddr := make(map[uint64][]uint64)
	for i := 0; i < len(wallet.currAccount.Transfers); i++ {
		output := wallet.currAccount.Transfers[i]
		if output.TokenID == tokenID && !output.Spent && !output.Frozen {
			if _, exist := indicePerSubaddr[output.SubAddrIndex]; exist {
				indicePerSubaddr[output.SubAddrIndex] = append(indicePerSubaddr[output.SubAddrIndex], uint64(i))
			} else {
				indicePerSubaddr[output.SubAddrIndex] = []uint64{uint64(i)}
			}
		}
	}
	return indicePerSubaddr
}

//try to find one ouput or two output satisfy balance > needMoney
func (wallet *Wallet) selectPreferIndice(needMoney uint64, subaddrs []uint64, tokenID common.Address) []uint64 {
	subaddrMap := make(map[uint64]bool)
	for _, subaddr := range subaddrs {
		subaddrMap[subaddr] = true
	}
	preferIndice := make([]uint64, 0)
	for i := 0; i < len(wallet.currAccount.Transfers); i++ {
		output := wallet.currAccount.Transfers[i]
		if _, exist := subaddrMap[output.SubAddrIndex]; !exist {
			continue
		}
		if output.TokenID == tokenID && !output.Spent && !output.Frozen && output.Amount.Uint64() >= needMoney {
			preferIndice = append(preferIndice, uint64(i))
			wallet.Logger.Debug("selectPreferIndice", "preferIndice", i, "output.Amount", output.Amount.String(), "needmoney", needMoney)
			return preferIndice
		}
	}
	currentOutputRelatdness := float32(1.0)
	for i := 0; i < len(wallet.currAccount.Transfers); i++ {
		output := wallet.currAccount.Transfers[i]
		if _, exist := subaddrMap[output.SubAddrIndex]; !exist {
			continue
		}
		if output.TokenID == tokenID && !output.Spent && !output.Frozen {
			for j := 0; j < len(wallet.currAccount.Transfers); j++ {
				output2 := wallet.currAccount.Transfers[j]
				if output.SubAddrIndex == output2.SubAddrIndex && output2.TokenID == tokenID &&
					!output2.Spent && !output2.Frozen && output.Amount.Uint64()+output2.Amount.Uint64() >= needMoney {
					relatedness := getOutputRelatedness(output, output2)
					if relatedness < currentOutputRelatdness {
						preferIndice = []uint64{uint64(i), uint64(j)}
						if 0 == relatedness {
							return preferIndice
						}
						currentOutputRelatdness = relatedness
					}
				}
			}
		}
	}
	return preferIndice
}

func (wallet *Wallet) selectBestIndex(indice []uint64, selectedIndice []uint64) uint64 {
	candidates := make([]uint64, 0)
	bestRelatedness := float32(1.0)
	for i := 0; i < len(indice); i++ {
		output := wallet.currAccount.Transfers[indice[i]]
		relatedness := float32(0.0)
		for j := 0; j < len(selectedIndice); j++ {
			output2 := wallet.currAccount.Transfers[selectedIndice[j]]
			r := getOutputRelatedness(output, output2)
			if r > relatedness {
				relatedness = r
				if relatedness == 1.0 {
					break
				}
			}
		}
		if relatedness < bestRelatedness {
			bestRelatedness = relatedness
			candidates = make([]uint64, 0)
		}
		if relatedness == bestRelatedness {
			candidates = append(candidates, uint64(i))
		}
	}
	//find smallest amount
	idx := 0
	for i := 1; i < len(candidates); i++ {
		output := wallet.currAccount.Transfers[indice[candidates[i]]]
		if output.Amount.Cmp(wallet.currAccount.Transfers[indice[candidates[idx]]].Amount) < 0 {
			idx = i
		}
	}
	return indice[candidates[idx]]
}

func (wallet *Wallet) constructSourceEntry(selectIndice []uint64) ([]*types.UTXOSourceEntry, error) {
	if 0 == len(selectIndice) {
		return nil, nil
	}
	str, _ := ser.MarshalJSON(selectIndice)
	wallet.Logger.Debug("constructSourceEntry", "selectIndice", str)
	//TODO other token
	maxIdx := wallet.getGOutIndex(common.EmptyAddress)

	rings := wallet.constructRings(maxIdx, UTXO_DEFAULT_RING_SIZE, selectIndice)
	if rings == nil {
		return wallet.constructSourceEntrySimple(selectIndice)
	}
	for idx, ring := range rings {
		str, _ = ser.MarshalJSON(ring)
		wallet.Logger.Debug("constructSourceEntry", "index", idx, "ring", str)
	}
	return wallet.constructSourceEntryNormal(selectIndice, rings)
}

//TODO check performance
func (wallet *Wallet) constructRings(maxIdx uint64, ringSize int, selectIndice []uint64) map[uint64]ring {
	if uint64(len(selectIndice)*ringSize) > maxIdx {
		return nil
	}
	rings := make(map[uint64]ring)
	excluded := make(map[uint64]bool)
	for _, selectIdx := range selectIndice {
		gIdx := wallet.currAccount.Transfers[selectIdx].GlobalIndex
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
	return rings
}

func (wallet *Wallet) constructSourceEntrySimple(selectIndice []uint64) ([]*types.UTXOSourceEntry, error) {
	wallet.Logger.Debug("constructSourceEntrySimple")
	gIndice := make([]uint64, 0)
	for _, selectIdx := range selectIndice {
		gIdx := wallet.currAccount.Transfers[selectIdx].GlobalIndex
		gIndice = append(gIndice, gIdx)
	}
	ringEntries, err := GetOutputsFromNode(gIndice, common.EmptyAddress)
	if err != nil {
		wallet.Logger.Error("constructSourceEntrySimple GetOutputsFromNode fail", "err", err)
		return nil, err
	}
	sources := make([]*types.UTXOSourceEntry, len(selectIndice))
	for i := 0; i < len(selectIndice); i++ {
		output := wallet.currAccount.Transfers[selectIndice[i]]
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

func (wallet *Wallet) constructSourceEntryNormal(selectIndice []uint64, rings map[uint64]ring) ([]*types.UTXOSourceEntry, error) {
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
		ringEntries, err := GetOutputsFromNode(indice, common.EmptyAddress)
		if err != nil {
			wallet.Logger.Error("constructSourceEntryNormal GetOutputsFromNode fail", "err", err)
			return nil, err
		}
		for i := start; i < end; i++ {
			output := wallet.currAccount.Transfers[selectIndice[i]]
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

func checkDestEntry(dest []*types.UTXODestEntry) (uint64, error) {
	if 0 == len(dest) {
		return 0, wtypes.ErrOutputEmpty
	}
	//rctSig need money to be uint64, we check here
	var totalMoney uint64
	for i := 0; i < len(dest); i++ {
		if dest[i].Amount.Sign() <= 0 {
			return 0, wtypes.ErrOutputMoneyInvalid
		}
		totalMoney += dest[i].Amount.Uint64()
		if totalMoney < dest[i].Amount.Uint64() {
			return 0, wtypes.ErrOutputMoneyOverFlow
		}
	}
	return totalMoney, nil
}

func initSortableSubaddr(subaddrs []uint64, unspentBalancePerSubaddr map[uint64]*big.Int) sortableSubaddrs {
	var ss sortableSubaddrs
	for _, subaddr := range subaddrs {
		if balance, exist := unspentBalancePerSubaddr[subaddr]; exist {
			sb := &subaddrBalance{
				Subaddr: subaddr,
				Balance: balance,
			}
			ss = append(ss, sb)
		}
	}
	return ss
}

// This returns a handwavy estimation of how much two outputs are related
// If they're from the same tx, then they're fully related. From close block
// heights, they're kinda related. The actual values don't matter, just
// their ordering, but it could become more murky if we add scores later.
func getOutputRelatedness(output, output2 *types.UTXOOutputDetail) float32 {
	// expensive test, and same tx will fall onto the same block height below
	if output.TxID == output2.TxID {
		return 1.0
	}
	// same block height -> possibly tx burst, or same tx (since above is disabled)
	dh := output.BlockHeight - output2.BlockHeight
	if output.BlockHeight < output2.BlockHeight {
		dh = output2.BlockHeight - output.BlockHeight
	}
	if dh == 0 {
		return 0.9
	}
	// adjacent blocks -> possibly tx burst
	if dh == 1 {
		return 0.8
	}
	// could extract the payment id, and compare them, but this is a bit expensive too
	// similar block heights
	if dh < 10 {
		return 0.2
	}
	// don't think these are particularly related
	return 0.0
}

func deleteIndexFromIndice(index uint64, indice []uint64) []uint64 {
	var i int
	for i = 0; i < len(indice); i++ {
		if indice[i] == index {
			break
		}
	}
	if i == len(indice) {
		return indice
	}
	if i == len(indice)-1 {
		return indice[:len(indice)-1]
	}
	indice[len(indice)-1], indice[i] = indice[i], indice[len(indice)-1]
	return indice[:len(indice)-1]
}

func estimateFee() uint64 {
	return UTXOTRANSACTION_FEE
}

func estimateTxWeight(inCnt int, outCnt int) uint64 {
	size := 0
	//input
	size += inCnt * (UTXO_DEFAULT_RING_SIZE*8 + 32)
	//output
	size += outCnt * (32 + 32)
	// type
	size++
	// message
	size += 32
	//bulletproof
	logOut := uint(0)
	for (1 << logOut) < outCnt {
		logOut++
	}
	size += (2*(7+int(logOut))+4+5)*32 + 3
	// MGs
	size += inCnt * (64*UTXO_DEFAULT_RING_SIZE + 32)
	// pseudoOuts
	size += 32 * inCnt
	// ecdhInfo
	size += 2 * 32 * outCnt
	// outPk - only commitment is saved
	size += 32 * outCnt
	// txnFee
	size += 4
	return uint64(size)
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
	needMoney, _, err := wallet.checkDest(dests, tokenID, AccountInputMode)
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
	availableMoney, err := GetTokenBalance(from, tokenID)
	if err != nil {
		wallet.Logger.Error("CreateAinTransaction getTokenBalance fail", "from", from, "tokenID", tokenID, "err", err)
		return nil, err
	}
	if availableMoney.Cmp(needMoney) <= 0 {
		return nil, wtypes.ErrBalanceNotEnough
	}
	source := &types.AccountSourceEntry{
		From:   from,
		Nonce:  nonce,
		Amount: big.NewInt(0).Set(needMoney),
	}
	tx, txKey, err := types.NewAinTransaction(source, dests, tokenID, nil)
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
	err = wallet.currAccount.saveTxKeys(ainTx.Hash(), txKey)
	if err != nil {
		wallet.Logger.Error("CreateAinTransaction saveTxKeys fail", "err", err)
		return nil, err
	}

	return ainTx, nil
}

//CreateUinTransaction return a UTXOTransaction for utxo input only
func (wallet *Wallet) CreateUinTransaction(from common.Address, passwd string, subaddrs []uint64, dests []types.DestEntry, tokenID common.Address,
	refundAddr common.Address, extra []byte) ([]*types.UTXOTransaction, error) {
	needMoney, _, err := wallet.checkDest(dests, tokenID, UTXOInputMode)
	if err != nil {
		return nil, err
	}
	wallet.Logger.Debug("CreateUinTransaction", "needMoney", needMoney, "subaddrs", subaddrs)
	acc := accounts.Account{Address: from}
	w, err := wallet.accManager.Find(acc)
	if err != nil {
		wallet.Logger.Error("CreateUinTransaction wallet.accManager.Find(acc)", "from", from, "err", err)
		return nil, wtypes.ErrAccountNotFound
	}
	unspentBalancePerSubaddr := wallet.unspentBalancePerSubaddr(tokenID)
	for addr, balance := range unspentBalancePerSubaddr {
		wallet.Logger.Debug("CreateUinTransaction unspentBalancePerSubaddr", "addr", addr, "balance", balance)
	}
	if 0 == len(subaddrs) {
		subaddrs = make([]uint64, 0)
		for subaddr := range unspentBalancePerSubaddr {
			subaddrs = append(subaddrs, subaddr)
		}
	}
	wallet.Logger.Debug("CreateUinTransaction", "subaddrs", fmt.Sprintf("%v", subaddrs))
	availableMoney := big.NewInt(0)
	for _, subaddr := range subaddrs {
		if balance, exist := unspentBalancePerSubaddr[subaddr]; exist {
			availableMoney.Add(availableMoney, balance)
		}
	}
	wallet.Logger.Debug("CreateUinTransaction", "availableMoney", availableMoney)
	if availableMoney.Cmp(big.NewInt(0).Sub(needMoney, wallet.estimateUtxoTxFee())) < 0 {
		return nil, wtypes.ErrBalanceNotEnough
	}
	unspentIndicePerSubaddr := wallet.unspentIndicePerSubaddr(tokenID)
	for addr, indice := range unspentIndicePerSubaddr {
		wallet.Logger.Debug("CreateUinTransaction unspentIndicePerSubaddr", "addr", addr, "indice", fmt.Sprintf("%v", indice))
	}
	sortableSubaddrs := initSortableSubaddr(subaddrs, unspentBalancePerSubaddr)
	sort.Sort(sortableSubaddrs)
	str, _ := ser.MarshalJSON(sortableSubaddrs)
	wallet.Logger.Debug("CreateUinTransaction", "sortableSubaddrs", str)

	preferIndice := wallet.selectPreferIndice1(needMoney, subaddrs, tokenID)
	if 0 != len(preferIndice) {
		preferSubaddr := wallet.currAccount.Transfers[preferIndice[0]].SubAddrIndex
		for i, sb := range sortableSubaddrs {
			if sb.Subaddr == preferSubaddr {
				sortableSubaddrs[0], sortableSubaddrs[i] = sortableSubaddrs[i], sortableSubaddrs[0]
			}
		}
	}
	wallet.Logger.Debug("CreateUinTransaction", "preferIndice", fmt.Sprintf("%v", preferIndice))
	str, _ = ser.MarshalJSON(sortableSubaddrs)
	wallet.Logger.Debug("CreateUinTransaction", "sortableSubaddrs", str)
	txes, err := wallet.createUinTransaction(w, acc, passwd, preferIndice, sortableSubaddrs, unspentIndicePerSubaddr, dests, tokenID, refundAddr, extra)
	if err != nil {
		wallet.Logger.Error("CreateUinTransaction createUinTransaction", "subaddrs", subaddrs, "dest", dests, "err", err)
		return nil, err
	}
	return txes, nil
}

func (wallet *Wallet) createUinTransaction(w accounts.Wallet, acc accounts.Account, passwd string, preferIndice []uint64, sortableSubaddrs sortableSubaddrs, unspentIndicePerSubaddr map[uint64][]uint64,
	dests []types.DestEntry, tokenID common.Address, refundAddr common.Address, extra []byte) ([]*types.UTXOTransaction, error) {
	var (
		signedTx          types.Tx
		addingFee         = false
		availableFee      = big.NewInt(0)
		needFee           = big.NewInt(0)
		selectedIndice    = make([]uint64, 0)
		paidDests         = make([]types.DestEntry, 0)
		accPaidIdx        = make(map[common.Address]int, 0)
		currUnspentIndice = unspentIndicePerSubaddr[sortableSubaddrs[0].Subaddr]
		txes              = make([]*types.UTXOTransaction, 0)
		chargeAccIdx      = sortableSubaddrs[0].Subaddr
	)
	for 0 != len(dests) || addingFee {
		wallet.Logger.Debug("createUinTransaction start to choose an output")
		wallet.Logger.Debug("createUinTransaction", "preferIndice", fmt.Sprintf("%v", preferIndice))
		str, _ := ser.MarshalJSON(dests)
		wallet.Logger.Debug("createUinTransaction", "dests", str)
		wallet.Logger.Debug("createUinTransaction", "selectedIndice", fmt.Sprintf("%v", selectedIndice))
		str, _ = ser.MarshalJSON(paidDests)
		wallet.Logger.Debug("createUinTransaction", "paidDests", str)
		wallet.Logger.Debug("createUinTransaction", "currUnspentIndice", fmt.Sprintf("%v", currUnspentIndice))
		str, _ = ser.MarshalJSON(txes)
		wallet.Logger.Debug("createUinTransaction", "txes", str)

		if 0 == len(currUnspentIndice) {
			wallet.Logger.Error("createUinTransaction", "len(currUnspentIndice)", len(currUnspentIndice))
			return nil, wtypes.ErrNoMoreOutput
		}
		var index uint64
		if 0 != len(preferIndice) {
			index = preferIndice[0]
			preferIndice = preferIndice[1:]
		} else {
			index = wallet.selectBestIndex(currUnspentIndice, selectedIndice)
		}
		currUnspentIndice = deleteIndexFromIndice(index, currUnspentIndice)
		selectedIndice = append(selectedIndice, index)
		output := wallet.currAccount.Transfers[index]
		availableAmount := big.NewInt(0).Set(output.Amount)
		if addingFee {
			availableFee.Add(availableFee, availableAmount)
		} else {
			for 0 != len(dests) && dests[0].GetAmount().Cmp(availableAmount) <= 0 &&
				estimateTxWeight(len(selectedIndice), len(paidDests)+1) < UTXOTRANSACTION_MAX_SIZE &&
				len(paidDests)+2 <= wtypes.UTXO_DESTS_MAX_NUM {
				if types.TypeUTXODest == dests[0].Type() {
					destEntry := &types.UTXODestEntry{
						Amount:       big.NewInt(0).Set(dests[0].GetAmount()),
						Addr:         dests[0].(*types.UTXODestEntry).Addr,
						IsSubaddress: dests[0].(*types.UTXODestEntry).IsSubaddress,
						Remark:       dests[0].(*types.UTXODestEntry).Remark,
					}
					paidDests = append(paidDests, destEntry)
				} else {
					//merge account dests
					if idx, exist := accPaidIdx[dests[0].(*types.AccountDestEntry).To]; exist {
						paidDests[idx].(*types.AccountDestEntry).Amount.Add(paidDests[idx].(*types.AccountDestEntry).Amount, dests[0].GetAmount())
					} else {
						destEntry := &types.AccountDestEntry{
							Amount: big.NewInt(0).Set(dests[0].GetAmount()),
							To:     dests[0].(*types.AccountDestEntry).To,
						}
						if len(dests[0].(*types.AccountDestEntry).Data) > 0 {
							destEntry.Data = make([]byte, len(dests[0].(*types.AccountDestEntry).Data))
							copy(destEntry.Data, dests[0].(*types.AccountDestEntry).Data)
						}
						paidDests = append(paidDests, destEntry)
						accPaidIdx[dests[0].(*types.AccountDestEntry).To] = len(paidDests) - 1
					}
				}
				availableAmount.Sub(availableAmount, dests[0].GetAmount())
				dests = dests[1:]
			}

			if availableAmount.Sign() > 0 && 0 != len(dests) && dests[0].GetAmount().Cmp(availableAmount) > 0 &&
				estimateTxWeight(len(selectedIndice), len(paidDests)+1) < UTXOTRANSACTION_MAX_SIZE &&
				len(paidDests)+2 <= wtypes.UTXO_DESTS_MAX_NUM {
				if types.TypeUTXODest == dests[0].Type() {
					destEntry := &types.UTXODestEntry{
						Amount:       big.NewInt(0).Set(availableAmount),
						Addr:         dests[0].(*types.UTXODestEntry).Addr,
						IsSubaddress: dests[0].(*types.UTXODestEntry).IsSubaddress,
						Remark:       dests[0].(*types.UTXODestEntry).Remark,
					}
					paidDests = append(paidDests, destEntry)
					dests[0].(*types.UTXODestEntry).Amount.Sub(dests[0].(*types.UTXODestEntry).Amount, availableAmount)
				} else {
					//merge account dests
					if idx, exist := accPaidIdx[dests[0].(*types.AccountDestEntry).To]; exist {
						paidDests[idx].(*types.AccountDestEntry).Amount.Add(paidDests[idx].(*types.AccountDestEntry).Amount, availableAmount)
					} else {
						destEntry := &types.AccountDestEntry{
							Amount: big.NewInt(0).Set(availableAmount),
							To:     dests[0].(*types.AccountDestEntry).To,
						}
						if len(dests[0].(*types.AccountDestEntry).Data) > 0 {
							destEntry.Data = make([]byte, len(dests[0].(*types.AccountDestEntry).Data))
							copy(destEntry.Data, dests[0].(*types.AccountDestEntry).Data)
						}
						paidDests = append(paidDests, destEntry)
						accPaidIdx[dests[0].(*types.AccountDestEntry).To] = len(paidDests) - 1
					}
					dests[0].(*types.AccountDestEntry).Amount.Sub(dests[0].(*types.AccountDestEntry).Amount, availableAmount)
				}
			}
		}
		tryTx := false
		if addingFee {
			noNeedChange := false
			if needFee.Cmp(wallet.estimateUtxoTxFee()) > 0 &&
				len(paidDests) == 1 &&
				types.TypeAcDest == paidDests[0].Type() &&
				0 == availableFee.Cmp(big.NewInt(0).Sub(needFee, wallet.estimateUtxoTxFee())) {
				noNeedChange = true
			}
			if estimateTxWeight(len(selectedIndice), len(paidDests)+1) > UTXOTRANSACTION_FEE_MAX_SIZE {
				return nil, wtypes.ErrTxTooBig
			}
			tryTx = availableFee.Cmp(needFee) >= 0 || noNeedChange
		} else {
			tryTx = (0 == len(dests)) || estimateTxWeight(len(selectedIndice), len(paidDests)+1) > UTXOTRANSACTION_MAX_SIZE || len(paidDests)+1 >= wtypes.UTXO_DESTS_MAX_NUM
			if tryTx && 0 == len(paidDests) {
				return nil, wtypes.ErrTxTooBig
			}
		}

		if tryTx {
			realNeedMoney, hasContract, err := wallet.checkDest(paidDests, tokenID, UTXOInputMode)
			if err != nil {
				return nil, err
			}
			var (
				inAmount   = big.NewInt(0)
				outAmount  = big.NewInt(0)
				needDecFee = false
			)
			for i := 0; i < len(selectedIndice); i++ {
				inAmount.Add(inAmount, wallet.currAccount.Transfers[selectedIndice[i]].Amount)
			}
			for j := 0; j < len(paidDests); j++ {
				outAmount.Add(outAmount, paidDests[j].GetAmount())
			}
			needFee.Sub(realNeedMoney, outAmount)
			if len(paidDests) == 1 &&
				types.TypeAcDest == paidDests[0].Type() &&
				0 == inAmount.Cmp(big.NewInt(0).Sub(realNeedMoney, wallet.estimateUtxoTxFee())) {
				realNeedMoney.Sub(realNeedMoney, wallet.estimateUtxoTxFee())
				needDecFee = true
			}
			if inAmount.Cmp(realNeedMoney) < 0 && !needDecFee {
				if inAmount.Cmp(outAmount) > 0 {
					needFee.Sub(needFee, big.NewInt(0).Sub(inAmount, outAmount))
				}
				addingFee = true
			} else {
				selectSources, err := wallet.constructSourceEntry(selectedIndice)
				if err != nil {
					wallet.Logger.Error("createUinTransaction constructSourceEntry fail", "err", err)
					return nil, err
				}
				wallet.Logger.Debug("createUinTransaction", "selectedIndice", selectedIndice)
				if inAmount.Cmp(realNeedMoney) > 0 {
					changeEntry := &types.UTXODestEntry{
						Amount:   big.NewInt(0).Sub(inAmount, realNeedMoney),
						Addr:     wallet.currAccount.account.Keys[chargeAccIdx].Addr,
						IsChange: true,
					}
					paidDests = append(paidDests, changeEntry)
					wallet.Logger.Debug("createUinTransaction add changeEntry", "chargeAccIdx", chargeAccIdx, "Addr", changeEntry.Addr, "Amount", changeEntry.Amount.String())
				}
				utxoTrans, utxoInEphs, mKeys, txKey, err := types.NewUinTransaction(wallet.currAccount.account.GetKeys(), wallet.currAccount.account.KeyIndex, selectSources, paidDests, tokenID, refundAddr, extra)
				if err != nil {
					wallet.Logger.Error("createUinTransaction NewUinTransaction fail", "err", err)
					return nil, wtypes.ErrNewUinTrans
				}
				if hasContract {
					if 0 == len(passwd) {
						signedTx, err = w.SignTx(acc, utxoTrans, types.SignParam)
						if err != nil {
							wallet.Logger.Error("createUinTransaction SignTx fail", "err", err)
							return nil, wtypes.ErrSignTx
						}
					} else {
						signedTx, err = w.SignTxWithPassphrase(acc, passwd, utxoTrans, types.SignParam)
						if err != nil {
							wallet.Logger.Error("createUinTransaction SignTxWithPassphrase fail", "err", err)
							return nil, wtypes.ErrSignTx
						}
					}
					utxoTrans = signedTx.(*types.UTXOTransaction)
				}
				err = types.UInTransWithRctSig(utxoTrans, selectSources, utxoInEphs, paidDests, mKeys)
				if err != nil {
					wallet.Logger.Error("createUinTransaction UInTransWithRctSig fail", "err", err)
					return nil, wtypes.ErrUinTransWithSign
				}

				// save txkey
				err = wallet.currAccount.saveTxKeys(utxoTrans.Hash(), txKey)
				if err != nil {
					wallet.Logger.Error("createUinTransaction saveTxKeys fail", "err", err)
					return nil, err
				}

				//save trans additional info. such as paid subaddress, outamount
				subAddrs := wallet.getSubaddrs(selectedIndice)
				err = wallet.currAccount.saveAddInfo(utxoTrans.Hash(), &wtypes.UTXOAddInfo{Subaddrs: subAddrs, OutAmount: outAmount})
				if err != nil {
					wallet.Logger.Error("createUinTransaction saveAddInfo fail", "err", err)
					return nil, err
				}

				txes = append(txes, utxoTrans)
				addingFee = false
				availableFee = big.NewInt(0)
				needFee = big.NewInt(0)
				selectedIndice = make([]uint64, 0)
				paidDests = make([]types.DestEntry, 0)
				accPaidIdx = make(map[common.Address]int)
			}
		}
		if 0 != len(dests) || addingFee {
			if 0 == len(currUnspentIndice) && len(sortableSubaddrs) > 1 {
				sortableSubaddrs = sortableSubaddrs[1:]
				currUnspentIndice = unspentIndicePerSubaddr[sortableSubaddrs[0].Subaddr]
			}
		}
	}
	return txes, nil
}

//CreateMinTransaction return a UTXOTransaction for mix input
func (wallet *Wallet) CreateMinTransaction(from common.Address, passwd string, nonce uint64, subaddrs []uint64,
	dests []types.DestEntry, tokenID common.Address, extra []byte) ([]*types.UTXOTransaction, error) {
	_, _, err := wallet.checkDest(dests, tokenID, MixInputMode)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (wallet *Wallet) checkDest(dests []types.DestEntry, tokenID common.Address, mode InputMode) (*big.Int, bool, error) {
	if 0 == len(dests) {
		return big.NewInt(0), false, wtypes.ErrOutputEmpty
	}
	transferMoney := big.NewInt(0)
	needMoney := big.NewInt(0)
	accTransMoney := big.NewInt(0)
	hasContract := false
	for i := 0; i < len(dests); i++ {
		wallet.Logger.Debug("checkDest", "amount", dests[i].GetAmount().String())
		if dests[i].GetAmount().Sign() <= 0 {
			return big.NewInt(0), hasContract, wtypes.ErrOutputMoneyInvalid
		}
		if dests[i].GetAmount().Sign() > 0 && (dests[i].GetAmount().Cmp(big.NewInt(types.UTXO_COMMITMENT_CHANGE_RATE)) < 0 ||
			big.NewInt(0).Mod(dests[i].GetAmount(), big.NewInt(types.UTXO_COMMITMENT_CHANGE_RATE)).Sign() != 0) {
			return big.NewInt(0), hasContract, wtypes.ErrOutputMoneyInvalid
		}
		//estimateGas return fee. if addr is a contract, return value*0.02+contract fee, if addr is a normal, return value*0.02
		if types.TypeAcDest == dests[i].Type() {
			isContract, err := wallet.isContract(dests[i].(*types.AccountDestEntry).To)
			if err != nil {
				return big.NewInt(0), hasContract, err
			}
			if !isContract && dests[i].GetAmount().Sign() == 0 {
				return big.NewInt(0), hasContract, wtypes.ErrOutputMoneyInvalid
			}
			if isContract {
				return big.NewInt(0), hasContract, wtypes.ErrNotSupportContractTx
			}
			// if isContract {
			// 	hasContract = true
			// 	nonce, err := EthGetTransactionCount(wallet.currAccount.getEthAddress())
			// 	if err != nil {
			// 		return big.NewInt(0), hasContract, err
			// 	}
			// 	var kind types.UTXOKind
			// 	switch mode {
			// 	case AccountInputMode:
			// 		kind |= types.Ain
			// 	case UTXOInputMode:
			// 		kind |= types.Uin
			// 	case MixInputMode:
			// 		kind |= types.Ain
			// 		kind |= types.Uin
			// 	}
			// 	kind |= types.Aout
			// 	fee, err := EstimateGas(wallet.currAccount.account.EthAddress, *nonce, dests[i].(*types.AccountDestEntry), kind, tokenID)
			// 	if err != nil {
			// 		return big.NewInt(0), hasContract, err
			// 	}
			// 	// vm run cost gasfee
			// 	if dests[i].GetAmount().Sign() > 0 {
			// 		fee.Sub(fee, wallet.estimateTxFee(dests[i].GetAmount()))
			// 	}
			// 	// needMoney add only vm cost fee
			// 	needMoney.Add(needMoney, fee)
			// }
			accTransMoney.Add(accTransMoney, dests[i].GetAmount())
		}
		transferMoney.Add(transferMoney, dests[i].GetAmount())
	}

	switch mode {
	case AccountInputMode:
		needMoney.Add(needMoney, transferMoney)
		if transferMoney.Sign() > 0 {
			needMoney.Add(needMoney, wallet.estimateTxFee(transferMoney))
		}
	case UTXOInputMode:
		needMoney.Add(needMoney, transferMoney)
		needMoney.Add(needMoney, wallet.estimateUtxoTxFee())
		needMoney.Add(needMoney, wallet.estimateTxFee(accTransMoney))
	case MixInputMode:
		//not support now
		return big.NewInt(0), hasContract, wtypes.ErrMixInputNotSupport
	}
	wallet.Logger.Debug("checkDest", "needMoney", needMoney.String())
	return needMoney, hasContract, nil
}

//limit account fee > 1e11 and tx fee mod 1e11 == 0
func (wallet *Wallet) estimateTxFee(transferMoney *big.Int) *big.Int {
	if transferMoney.Sign() == 0 {
		return big.NewInt(0)
	}
	gasLimit := types.CalNewAmountGas(transferMoney, types.EverLiankeFee)
	return big.NewInt(0).Mul(big.NewInt(0).SetUint64(gasLimit), big.NewInt(types.GasPrice))
}

//limit utxo fee > 1e11 and tx fee mod 1e11 == 0
func (wallet *Wallet) estimateUtxoTxFee() *big.Int {
	return wallet.utxoGas
}

//try to find one ouput or two output satisfy balance > needMoney
func (wallet *Wallet) selectPreferIndice1(needMoney *big.Int, subaddrs []uint64, tokenID common.Address) []uint64 {
	subaddrMap := make(map[uint64]bool)
	for _, subaddr := range subaddrs {
		subaddrMap[subaddr] = true
	}
	preferIndice := make([]uint64, 0)
	for i := 0; i < len(wallet.currAccount.Transfers); i++ {
		output := wallet.currAccount.Transfers[i]
		if _, exist := subaddrMap[output.SubAddrIndex]; !exist {
			continue
		}
		if output.TokenID == tokenID && !output.Spent && !output.Frozen && output.Amount.Cmp(needMoney) >= 0 {
			preferIndice = append(preferIndice, uint64(i))
			return preferIndice
		}
	}
	currentOutputRelatdness := float32(1.0)
	for i := 0; i < len(wallet.currAccount.Transfers); i++ {
		output := wallet.currAccount.Transfers[i]
		if _, exist := subaddrMap[output.SubAddrIndex]; !exist {
			continue
		}
		if output.TokenID == tokenID && !output.Spent && !output.Frozen {
			for j := 0; j < len(wallet.currAccount.Transfers); j++ {
				output2 := wallet.currAccount.Transfers[j]
				if output.SubAddrIndex == output2.SubAddrIndex && output2.TokenID == tokenID &&
					!output2.Spent && !output2.Frozen && big.NewInt(0).Add(output.Amount, output2.Amount).Cmp(needMoney) >= 0 {
					relatedness := getOutputRelatedness(output, output2)
					if relatedness < currentOutputRelatdness {
						preferIndice = []uint64{uint64(i), uint64(j)}
						if 0 == relatedness {
							return preferIndice
						}
						currentOutputRelatdness = relatedness
					}
				}
			}
		}
	}
	return preferIndice
}

func (wallet *Wallet) getSubaddrs(selectedIndice []uint64) []uint64 {
	addrMap := make(map[uint64]bool, 0)
	for _, selectIdx := range selectedIndice {
		subAddr := wallet.currAccount.Transfers[selectIdx].SubAddrIndex
		addrMap[subAddr] = true
	}
	subAddrs := make([]uint64, 0)
	for addr := range addrMap {
		subAddrs = append(subAddrs, addr)
	}
	return subAddrs
}
