package wallet

import (
	"bytes"
	"fmt"
	"math/big"
	"math/rand"
	"sort"

	"github.com/lianxiangcloud/linkchain/libs/common"
	lkctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/types"
	wtypes "github.com/lianxiangcloud/linkchain/wallet/types"
)

const (
	DFS_TOTAL_TRIES         = 1000000
	UTXO_TX_LOW_SIZE_LIMIT  = 1024 * 25
	UTXO_TX_HIGH_SIZE_LIMIT = 1024 * 30
)

type UTXOItem struct {
	subaddr  uint64
	localIdx uint64
	height   uint64
	amount   *big.Int
}

func (item *UTXOItem) String() string {
	return fmt.Sprintf("{subaddr: %d localIdx: %d height: %d amount: %d}\n", item.subaddr, item.localIdx, item.height, item.amount)
}

func descUTXOPoolByAmount(utxoPool []*UTXOItem) {
	sort.Slice(utxoPool, func(i, j int) bool {
		return utxoPool[i].amount.Cmp(utxoPool[j].amount) > 0
	})
}

func descUTXOPoolByHeight(utxoPool []*UTXOItem) {
	sort.Slice(utxoPool, func(i, j int) bool {
		return utxoPool[i].height > utxoPool[j].height
	})
}

func knuthDurstenfeldShuffle(utxoPool []*UTXOItem) {
	if len(utxoPool) <= 2 {
		return
	}
	var r int
	for i := len(utxoPool); i > 0; i-- {
		r = rand.Intn(i)
		utxoPool[r], utxoPool[i-1] = utxoPool[i-1], utxoPool[r]
	}
}

func selectDFS(utxoPool []*UTXOItem, target *big.Int) ([]*UTXOItem, error) {
	var (
		selectedAmount  = big.NewInt(0)
		availableAmount = big.NewInt(0)
		selectedUTXOs   = make([]*UTXOItem, 0)
		selectedState   = make([]bool, len(utxoPool))
		selectedDepth   = 0
		backtrace       = false
	)
	for _, utxo := range utxoPool {
		availableAmount.Add(availableAmount, utxo.amount)
	}
	if availableAmount.Cmp(target) < 0 {
		return nil, wtypes.ErrBalanceNotEnough
	}
	descUTXOPoolByAmount(utxoPool)
	fmt.Printf("target: %d sorted utxoPool: %v\n", target, utxoPool)
	for try := 0; try < DFS_TOTAL_TRIES; try++ {
		if selectedAmount.Cmp(target) == 0 {
			break
		}
		backtrace = false
		if big.NewInt(0).Add(selectedAmount, availableAmount).Cmp(target) < 0 || selectedAmount.Cmp(target) > 0 {
			backtrace = true
		}
		if backtrace { //back and try another branch
			for selectedDepth > 0 && !selectedState[selectedDepth-1] {
				availableAmount.Add(availableAmount, utxoPool[selectedDepth-1].amount)
				fmt.Printf("backtrace utxo, index: %d amount: %d selectAmount: %d availableAmount: %d\n", selectedDepth-1, utxoPool[selectedDepth-1].amount, selectedAmount, availableAmount)
				selectedDepth--
			}
			if selectedDepth == 0 {
				break
			}
			selectedState[selectedDepth-1] = false
			selectedAmount.Sub(selectedAmount, utxoPool[selectedDepth-1].amount)
			fmt.Printf("ignore utxo, index: %d amount: %d selectedAmount: %d availableAmount: %d\n", selectedDepth-1, utxoPool[selectedDepth-1].amount, selectedAmount, availableAmount)
		} else { //continue current branch
			availableAmount.Sub(availableAmount, utxoPool[selectedDepth].amount)
			if selectedDepth > 0 && selectedState[selectedDepth-1] == false &&
				utxoPool[selectedDepth].amount == utxoPool[selectedDepth-1].amount {
				selectedState[selectedDepth] = false
				fmt.Printf("ignore dup utxo, index: %d amount: %d selectedAmount: %d availableAmount: %d\n", selectedDepth, utxoPool[selectedDepth].amount, selectedAmount, availableAmount)
			} else {
				selectedAmount.Add(selectedAmount, utxoPool[selectedDepth].amount)
				selectedState[selectedDepth] = true
				fmt.Printf("select utxo, index: %d amount: %d selectedAmount: %d availableAmount: %d\n", selectedDepth, utxoPool[selectedDepth].amount, selectedAmount, availableAmount)
			}
			selectedDepth++
		}
	}
	//exact match
	if selectedAmount.Cmp(target) == 0 {
		for i := 0; i < selectedDepth; i++ {
			if selectedState[i] {
				selectedUTXOs = append(selectedUTXOs, utxoPool[i])
			}
		}
		fmt.Printf("selectedAmount: %d selectedUtxos: %v\n", selectedAmount, selectedUTXOs)
		return selectedUTXOs, nil
	}
	return nil, wtypes.ErrExactMatchFail
}

func selectSRD(utxoPool []*UTXOItem, target *big.Int) ([]*UTXOItem, *big.Int, error) {
	var (
		selectedAmount  = big.NewInt(0)
		availableAmount = big.NewInt(0)
		selectedUTXOs   = make([]*UTXOItem, 0)
	)
	for _, utxo := range utxoPool {
		availableAmount.Add(availableAmount, utxo.amount)
	}
	if availableAmount.Cmp(target) < 0 {
		return nil, nil, wtypes.ErrBalanceNotEnough
	}
	descUTXOPoolByHeight(utxoPool)
	fmt.Printf("target: %d sorted utxoPool: %v\n", target, utxoPool)
	knuthDurstenfeldShuffle(utxoPool)
	fmt.Printf("shuffle utxoPool: %v\n", utxoPool)
	for _, utxo := range utxoPool {
		selectedUTXOs = append(selectedUTXOs, utxo)
		selectedAmount.Add(selectedAmount, utxo.amount)
		if selectedAmount.Cmp(target) >= 0 {
			break
		}
	}
	fmt.Printf("selectedAmount: %d selectedUTXOs: %v\n", selectedAmount, selectedUTXOs)
	return selectedUTXOs, selectedAmount, nil
}

func coinSelection(utxoPool []*UTXOItem, target *big.Int) ([]*UTXOItem, *big.Int, error) {
	var (
		smallUTXOPool    = make([]*UTXOItem, 0)
		smallTotalAmount = big.NewInt(0)
		totalAmount      = big.NewInt(0)
		minLargeUTXO     *UTXOItem
	)
	for _, utxo := range utxoPool {
		totalAmount.Add(totalAmount, utxo.amount)
		if utxo.amount.Cmp(target) < 0 {
			smallTotalAmount.Add(smallTotalAmount, utxo.amount)
			smallUTXOPool = append(smallUTXOPool, utxo)
		} else if minLargeUTXO == nil || minLargeUTXO.amount.Cmp(utxo.amount) > 0 {
			minLargeUTXO = utxo
		}
	}
	if totalAmount.Cmp(target) < 0 {
		return nil, nil, wtypes.ErrBalanceNotEnough
	}
	if minLargeUTXO != nil && minLargeUTXO.amount.Cmp(target) == 0 {
		return []*UTXOItem{minLargeUTXO}, big.NewInt(0).Set(minLargeUTXO.amount), nil
	}
	if smallTotalAmount.Cmp(target) == 0 {
		return smallUTXOPool, big.NewInt(0).Set(smallTotalAmount), nil
	}
	if smallTotalAmount.Cmp(target) < 0 {
		return []*UTXOItem{minLargeUTXO}, big.NewInt(0).Set(minLargeUTXO.amount), nil
	}
	selectedUTXOs, err := selectDFS(smallUTXOPool, target)
	if err == nil {
		return selectedUTXOs, big.NewInt(0).Set(target), nil
	}
	selectedUTXOs, selectedAmount, _ := selectSRD(smallUTXOPool, target)
	if selectedAmount.Cmp(target) != 0 && minLargeUTXO != nil && selectedAmount.Cmp(minLargeUTXO.amount) < 0 {
		return []*UTXOItem{minLargeUTXO}, big.NewInt(0).Set(minLargeUTXO.amount), nil
	}
	return selectedUTXOs, big.NewInt(0).Set(selectedAmount), nil
}

func updateSubaddrs(subaddrs []uint64, unspentBalancePerSubaddr map[uint64]*big.Int) ([]uint64, *big.Int) {
	var (
		newSubaddrs    = make([]uint64, 0)
		availableMoney = big.NewInt(0)
	)
	if 0 == len(subaddrs) {
		subaddrs = make([]uint64, 0)
		for subaddr := range unspentBalancePerSubaddr {
			subaddrs = append(subaddrs, subaddr)
		}
	}
	for _, subaddr := range subaddrs {
		if balance, exist := unspentBalancePerSubaddr[subaddr]; exist {
			availableMoney.Add(availableMoney, balance)
			newSubaddrs = append(newSubaddrs, subaddr)
		}
	}
	return newSubaddrs, availableMoney
}

func getChangeSubaddr(subaddrs []uint64, unspentBalancePerSubaddr map[uint64]*big.Int) uint64 {
	var (
		changeSubaddr  = uint64(0)
		largestBalance = big.NewInt(0)
	)
	for _, subaddr := range subaddrs {
		if balance, exist := unspentBalancePerSubaddr[subaddr]; exist {
			if largestBalance.Cmp(balance) < 0 {
				changeSubaddr = subaddr
				largestBalance.Set(balance)
			}
		}
	}
	return changeSubaddr
}

//do not care about account output and ser encode now
func estimateTxSize(inCnt int, outCnt int, ringSize int) uint64 {
	size := 0
	//input
	size += inCnt * (ringSize*8 + 32)
	//output
	size += outCnt * (32 + 32)
	//token_id, rkey, addkey
	size += 32 + 32 + outCnt*32

	//---struct RctSigBase
	// type
	size++
	// pseudoOuts
	size += 32 * inCnt
	// ecdhInfo
	size += 3 * 32 * outCnt
	// outPk
	size += 2 * 32 * outCnt
	// txnFee
	size += 8

	//---struct RctSigPrunable
	//bulletproof
	logOut := uint(0)
	for (1 << logOut) < outCnt {
		logOut++
	}
	size += (2*(6+int(logOut)) + 4 + 5) * 32
	if ringSize == UTXO_SIMPLE_RING_SIZE { //Ss
		size += inCnt * (32 + 32)
	} else { // MGs
		size += inCnt * (64*ringSize + 32)
	}
	return uint64(size)
}

//make sure dest.Amount >= paidAmount
func payDest(paidAmount *big.Int, dest types.DestEntry, paidDests []types.DestEntry,
	accPaidIdx map[common.Address]int) ([]types.DestEntry, map[common.Address]int) {
	if types.TypeUTXODest == dest.Type() {
		utxodest := dest.(*types.UTXODestEntry)
		paidDests = append(paidDests, &types.UTXODestEntry{
			Amount:       big.NewInt(0).Set(paidAmount),
			Addr:         utxodest.Addr,
			IsSubaddress: utxodest.IsSubaddress,
			Remark:       utxodest.Remark,
		})
		utxodest.Amount.Sub(utxodest.Amount, paidAmount)
	} else {
		accdest := dest.(*types.AccountDestEntry)
		if idx, exist := accPaidIdx[accdest.To]; exist {
			paidAccDest := paidDests[idx].(*types.AccountDestEntry)
			paidAccDest.Amount.Add(paidAccDest.Amount, paidAmount)
		} else {
			destEntry := &types.AccountDestEntry{
				Amount: big.NewInt(0).Set(paidAmount),
				To:     accdest.To,
			}
			if len(accdest.Data) > 0 {
				destEntry.Data = make([]byte, len(accdest.Data))
				copy(destEntry.Data, accdest.Data)
			}
			paidDests = append(paidDests, destEntry)
			accPaidIdx[accdest.To] = len(paidDests) - 1
		}
		accdest.Amount.Sub(accdest.Amount, paidAmount)
	}
	return paidDests, accPaidIdx
}

func payDests(selectUtxos []*UTXOItem, dests []types.DestEntry) ([]types.DestEntry, error) {
	var (
		paiedDests = make([]types.DestEntry, 0)
		accPaidIdx = make(map[common.Address]int, 0)
		utxoAmount = big.NewInt(0)
	)
	for i := 0; i < len(selectUtxos) && len(dests) > 0; i++ {
		utxoAmount.Set(selectUtxos[i].amount)
		for utxoAmount.Sign() > 0 && len(dests) > 0 {
			paidAmount := big.NewInt(0).Set(dests[0].GetAmount())
			if utxoAmount.Cmp(paidAmount) < 0 {
				paidAmount.Set(utxoAmount)
			}
			paiedDests, accPaidIdx = payDest(paidAmount, dests[0], paiedDests, accPaidIdx)
			utxoAmount.Sub(utxoAmount, paidAmount)
			if dests[0].GetAmount().Sign() == 0 {
				dests = dests[1:]
			}
		}
	}
	if len(dests) > 0 {
		return nil, wtypes.ErrBalanceNotEnough
	}
	return paiedDests, nil
}

func mergeDests(dests []types.DestEntry) ([]types.DestEntry, error) {
	if len(dests) <= wtypes.UTXO_DESTS_MAX_NUM {
		return dests, nil
	}
	var (
		utxoAddrs  = make(map[lkctypes.PublicKey]int, 0)
		newDests   = make([]types.DestEntry, 0)
		mergeDests = make([]types.DestEntry, 0)
		dupCnt     = 0
		nodupCnt   = 0
	)
	for _, dest := range dests { //account dest already merged
		if types.TypeUTXODest == dest.Type() {
			utxodest := dest.(*types.UTXODestEntry)
			if _, exist := utxoAddrs[utxodest.Addr.SpendPublicKey]; exist {
				utxoAddrs[utxodest.Addr.SpendPublicKey]++
			} else {
				utxoAddrs[utxodest.Addr.SpendPublicKey] = 1
			}
		}
	}
	for _, dest := range dests {
		if types.TypeUTXODest == dest.Type() {
			utxodest := dest.(*types.UTXODestEntry)
			if utxoAddrs[utxodest.Addr.SpendPublicKey] > 1 {
				mergeDests = append(mergeDests, dest)
				continue
			}
		}
		newDests = append(newDests, dest)
	}
	for _, cnt := range utxoAddrs {
		if cnt > 1 {
			dupCnt++
		} else {
			nodupCnt++
		}
	}
	sort.Slice(mergeDests, func(i, j int) bool {
		return mergeDests[i].GetAmount().Cmp(mergeDests[j].GetAmount()) > 0
	})
	for len(mergeDests) > dupCnt && len(mergeDests)+nodupCnt > wtypes.UTXO_DESTS_MAX_NUM {
		smallestDest := mergeDests[len(mergeDests)-1].(*types.UTXODestEntry)
		i := len(mergeDests) - 2
		for ; i >= 0; i-- {
			currDest := mergeDests[i].(*types.UTXODestEntry)
			if bytes.Equal(smallestDest.Addr.SpendPublicKey[:], currDest.Addr.SpendPublicKey[:]) {
				break
			}
		}
		if i < 0 {
			newDests = append(newDests, smallestDest)
			mergeDests = mergeDests[:len(mergeDests)-1]
			dupCnt--
			nodupCnt++
			continue
		}
		matchDest := mergeDests[i].(*types.UTXODestEntry)
		matchDest.Amount.Add(matchDest.Amount, smallestDest.Amount)
		mergeDests = mergeDests[:len(mergeDests)-1]
	}
	if len(mergeDests)+nodupCnt > wtypes.UTXO_DESTS_MAX_NUM {
		return nil, wtypes.ErrDestsMergeFail
	}
	for _, dest := range mergeDests {
		newDests = append(newDests, dest)
	}
	return newDests, nil
}

type inOutPacket struct {
	Inputs  []*UTXOItem
	Outputs []types.DestEntry
	Sources []*types.UTXOSourceEntry
}

func (wallet *Wallet) directSelection(utxoPool []*UTXOItem, dests []types.DestEntry, changeSubaddr uint64,
	tokenID common.Address) ([]*inOutPacket, error) {
	var (
		selectedUtxos  = make([]*UTXOItem, 0)
		selectedAmount = big.NewInt(0)
		needAmount     = big.NewInt(0)
		utxoAmount     = big.NewInt(0)
		outKind        = NilOut
		paidDests      = make([]types.DestEntry, 0)
		accPaidIdx     = make(map[common.Address]int, 0)
		packets        = make([]*inOutPacket, 0)
		addingFee      = false
		finish         = false
		checked        = false
		outputCnt      = 0
		err            error
	)
	descUTXOPoolByAmount(utxoPool)
	for i := 0; i < len(utxoPool) && (len(dests) > 0 || addingFee); i++ {
		utxoAmount.Set(utxoPool[i].amount)
		selectedUtxos = append(selectedUtxos, utxoPool[i])
		selectedAmount.Add(selectedAmount, utxoAmount)
		fmt.Printf("select utxo: %s\n", utxoAmount)
		if !addingFee {
			for utxoAmount.Sign() > 0 && len(dests) > 0 {
				outputCnt = len(paidDests)
				if types.TypeUTXODest == dests[0].Type() {
					outputCnt++
				}
				if outputCnt > wtypes.UTXO_DESTS_MAX_NUM {
					outputCnt = wtypes.UTXO_DESTS_MAX_NUM
				}
				if estimateTxSize(len(selectedUtxos), outputCnt, wallet.getRingSize(len(selectedUtxos))) > UTXO_TX_LOW_SIZE_LIMIT {
					addingFee = true
					break
				}
				paidAmount := big.NewInt(0).Set(dests[0].GetAmount())
				if utxoAmount.Cmp(dests[0].GetAmount()) < 0 {
					paidAmount.Set(utxoAmount)
				}
				fmt.Printf("utxoAmount: %s destAmount: %s paidAmount: %s\n", utxoAmount, dests[0].GetAmount(), paidAmount)
				paidDests, accPaidIdx = payDest(paidAmount, dests[0], paidDests, accPaidIdx)
				utxoAmount.Sub(utxoAmount, paidAmount)
				if dests[0].GetAmount().Sign() == 0 {
					dests = dests[1:]
				}
			}
			if !addingFee && len(dests) > 0 {
				continue
			}
		}
		if !checked {
			needAmount, outKind, err = wallet.checkDest(paidDests, tokenID, UTXOInputMode)
			if err != nil {
				return nil, err
			}
			checked = true
		}
		if estimateTxSize(len(selectedUtxos), outputCnt, wallet.getRingSize(len(selectedUtxos))) > UTXO_TX_HIGH_SIZE_LIMIT {
			return nil, wtypes.ErrTxTooBig
		}
		if (outKind&UtxoOut) == NilOut &&
			(selectedAmount.Cmp(needAmount) == 0 ||
				selectedAmount.Cmp(big.NewInt(0).Add(needAmount, wallet.estimateUtxoTxFee())) >= 0) {
			finish = true
		}
		if (outKind&UtxoOut) != NilOut && selectedAmount.Cmp(needAmount) >= 0 {
			finish = true
		}
		if finish {
			packets = append(packets, &inOutPacket{
				Inputs:  selectedUtxos,
				Outputs: paidDests,
			})
			selectedUtxos = make([]*UTXOItem, 0)
			selectedAmount = big.NewInt(0)
			paidDests = make([]types.DestEntry, 0)
			accPaidIdx = make(map[common.Address]int, 0)
			needAmount = big.NewInt(0)
			addingFee = false
			finish = false
			checked = false
		}
	}
	if len(dests) > 0 || addingFee {
		return nil, wtypes.ErrBalanceNotEnough
	}
	for _, packet := range packets {
		packet.Outputs, err = wallet.changeAndMerge(packet.Inputs, packet.Outputs, changeSubaddr, tokenID)
		if err != nil {
			return nil, err
		}
	}
	return packets, nil
}

func (wallet *Wallet) getRingSize(inCnt int) int {
	maxGIdx := wallet.getGOutIndex(common.EmptyAddress)
	if uint64(inCnt*UTXO_DEFAULT_RING_SIZE) > maxGIdx {
		return UTXO_DEFAULT_RING_SIZE
	}
	return UTXO_SIMPLE_RING_SIZE
}

func (wallet *Wallet) checkTxSize(inputCnt int, outKind OutKind) bool {
	if (outKind&UtxoOut) == NilOut &&
		estimateTxSize(inputCnt, 1, wallet.getRingSize(inputCnt)) <= UTXO_TX_HIGH_SIZE_LIMIT {
		return true
	}
	if inputCnt < wtypes.UTXO_DESTS_MAX_NUM &&
		estimateTxSize(inputCnt, inputCnt+1, wallet.getRingSize(inputCnt)) <= UTXO_TX_HIGH_SIZE_LIMIT {
		return true
	}
	if inputCnt >= wtypes.UTXO_DESTS_MAX_NUM &&
		estimateTxSize(inputCnt, wtypes.UTXO_DESTS_MAX_NUM, wallet.getRingSize(inputCnt)) <= UTXO_TX_HIGH_SIZE_LIMIT {
		return true
	}
	return false
}

func (wallet *Wallet) selectionProcess(utxoPool []*UTXOItem, dests []types.DestEntry, changeSubaddr uint64,
	tokenID common.Address) ([]*inOutPacket, error) {
	needMoney, outKind, err := wallet.checkDest(dests, common.EmptyAddress, UTXOInputMode)
	if err != nil {
		return nil, err
	}
	selectedUtxos, selectedAmount, err := coinSelection(utxoPool, needMoney)
	if err != nil {
		return nil, err
	}
	if (outKind&UtxoOut) == NilOut && selectedAmount.Cmp(needMoney) > 0 {
		selectedUtxos, selectedAmount, err = coinSelection(utxoPool, needMoney.Add(needMoney, wallet.estimateUtxoTxFee()))
		if err != nil {
			return nil, err
		}
	}
	if wallet.checkTxSize(len(selectedUtxos), outKind) {
		paidDests, err := payDests(selectedUtxos, dests)
		if err != nil {
			return nil, err
		}
		paidDests, err = wallet.changeAndMerge(selectedUtxos, paidDests, changeSubaddr, tokenID)
		if err == nil {
			return []*inOutPacket{&inOutPacket{
				Inputs:  selectedUtxos,
				Outputs: paidDests,
			}}, nil
		}
	}
	return wallet.directSelection(utxoPool, dests, changeSubaddr, tokenID)
}

func (wallet *Wallet) constructUTXOPool(subaddrs []uint64, unspentIndicePerSubaddr map[uint64][]uint64) []*UTXOItem {
	utxoPool := make([]*UTXOItem, 0)
	for _, subaddr := range subaddrs {
		if unspentIdx, exist := unspentIndicePerSubaddr[subaddr]; exist {
			for _, idx := range unspentIdx {
				utxoPool = append(utxoPool, &UTXOItem{
					subaddr:  subaddr,
					localIdx: idx,
					height:   wallet.currAccount.Transfers[idx].BlockHeight,
					amount:   big.NewInt(0).Set(wallet.currAccount.Transfers[idx].Amount),
				})
			}
		}
	}
	return utxoPool
}

func (wallet *Wallet) constructRingMembers(packets []*inOutPacket) error {
	for _, packet := range packets {
		selectedIdx := make([]uint64, 0)
		for _, item := range packet.Inputs {
			selectedIdx = append(selectedIdx, item.localIdx)
		}
		sources, err := wallet.constructSourceEntry(selectedIdx)
		if err != nil {
			return err
		}
		packet.Sources = sources
	}
	return nil
}

func (wallet *Wallet) changeAndMerge(selectedUtxos []*UTXOItem, dests []types.DestEntry, changeSubaddr uint64,
	tokenID common.Address) ([]types.DestEntry, error) {
	needAmount, outKind, err := wallet.checkDest(dests, tokenID, UTXOInputMode)
	if err != nil {
		return nil, err
	}
	totalAmount := big.NewInt(0)
	for _, utxo := range selectedUtxos {
		totalAmount.Add(totalAmount, utxo.amount)
	}
	if totalAmount.Cmp(needAmount) < 0 {
		return nil, wtypes.ErrBalanceNotEnough
	}
	if (outKind&UtxoOut) == NilOut && totalAmount.Cmp(needAmount) > 0 {
		needAmount.Add(needAmount, wallet.estimateUtxoTxFee())
		if totalAmount.Cmp(needAmount) < 0 {
			return nil, wtypes.ErrBalanceNotEnough
		}
	}
	if totalAmount.Cmp(needAmount) > 0 {
		dests = append(dests, &types.UTXODestEntry{
			Amount:   big.NewInt(0).Sub(totalAmount, needAmount),
			Addr:     wallet.currAccount.account.Keys[changeSubaddr].Addr,
			IsChange: true,
		})
	}
	dests, err = mergeDests(dests)
	if err != nil {
		return nil, err
	}
	return dests, nil
}

func (wallet *Wallet) saveAddInfo(hash common.Hash, packet *inOutPacket, changeSubaddr uint64) error {
	addrmap := make(map[uint64]bool, 0)
	for _, utxo := range packet.Inputs {
		addrmap[utxo.subaddr] = true
	}
	subAddrs := make([]uint64, 0)
	for addr := range addrmap {
		subAddrs = append(subAddrs, addr)
	}
	sort.Slice(subAddrs, func(i, j int) bool {
		return subAddrs[i] < subAddrs[j]
	})
	outAmount := big.NewInt(0)
	for _, dest := range packet.Outputs {
		if utxodest, ok := dest.(*types.UTXODestEntry); ok && utxodest.IsChange {
			continue
		}
		outAmount.Add(outAmount, dest.GetAmount())
	}
	if err := wallet.currAccount.saveAddInfo(hash, &wtypes.UTXOAddInfo{
		Subaddrs:  subAddrs,
		OutAmount: outAmount,
		ChangeIdx: int(changeSubaddr),
	}); err != nil {
		return err
	}
	return nil
}

//CreateUinTransaction return a UTXOTransaction for utxo input only
func (wallet *Wallet) CreateUinTransaction1(subaddrs []uint64, dests []types.DestEntry, tokenID common.Address,
	extra []byte) ([]*types.UTXOTransaction, error) {
	needMoney, _, err := wallet.checkDest(dests, tokenID, UTXOInputMode)
	if err != nil {
		return nil, err
	}
	unspentBalancePerSubaddr := wallet.unspentBalancePerSubaddr(tokenID)
	subaddrs, availableMoney := updateSubaddrs(subaddrs, unspentBalancePerSubaddr)
	wallet.Logger.Debug("CreateUinTransaction", "availableMoney", availableMoney, "needMoney", needMoney)
	if availableMoney.Cmp(needMoney) < 0 {
		return nil, wtypes.ErrBalanceNotEnough
	}
	changeSubaddr := getChangeSubaddr(subaddrs, unspentBalancePerSubaddr)
	unspentIndicePerSubaddr := wallet.unspentIndicePerSubaddr(tokenID)
	utxoPool := wallet.constructUTXOPool(subaddrs, unspentIndicePerSubaddr)
	inOutPackets, err := wallet.selectionProcess(utxoPool, dests, changeSubaddr, tokenID)
	if err != nil {
		return nil, err
	}
	if err = wallet.constructRingMembers(inOutPackets); err != nil {
		return nil, err
	}
	txs := make([]*types.UTXOTransaction, 0)
	for _, packet := range inOutPackets {
		utxoTx, utxoInEphs, mKeys, txKey, err := types.NewUinTransaction(wallet.currAccount.account.GetKeys(),
			wallet.currAccount.account.KeyIndex, packet.Sources, packet.Outputs, tokenID, common.EmptyAddress, extra)
		if err != nil {
			return nil, wtypes.ErrNewUinTrans
		}
		if err = types.UInTransWithRctSig(utxoTx, packet.Sources, utxoInEphs, packet.Outputs, mKeys); err != nil {
			return nil, wtypes.ErrUinTransWithSign
		}
		if utxoTx.Size() > types.MaxPureTransactionSize {
			return nil, wtypes.ErrTxTooBig
		}
		// save txkey
		if err = wallet.currAccount.saveTxKeys(utxoTx.Hash(), txKey); err != nil {
			return nil, err
		}
		//save trans additional info. such as paid subaddress, outamount
		if err = wallet.saveAddInfo(utxoTx.Hash(), packet, changeSubaddr); err != nil {
			return nil, err
		}
		txs = append(txs, utxoTx)
	}
	return txs, nil
}
