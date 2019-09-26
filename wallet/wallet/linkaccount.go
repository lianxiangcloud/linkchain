package wallet

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/ringct"
	lkctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/xcrypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
	tctypes "github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/wallet/types"
)

const (
	defaultRefreshBlockInterval = 5 * time.Second
	defaultMaxSubAccount        = uint64(5000)
	// 1 - utxo out，2- utxo in
	rpcUOut uint8 = 0x1
	rpcUIn  uint8 = 0x2

	txAin  uint8 = 0x1
	txAout uint8 = 0x2
	txUin  uint8 = 0x4
	txUout uint8 = 0x8
)

var (
	LinkToken              = common.EmptyAddress
	defaultInitBlockHeight = uint64(0)
)

type transferContainer []*tctypes.UTXOOutputDetail
type balanceMap map[uint64]*big.Int //subaddr index,value balance

// LinkAccount -
type LinkAccount struct {
	// cmn.BaseService
	Logger               log.Logger
	remoteHeight         *big.Int
	localHeight          *big.Int
	lock                 sync.Mutex
	utxoTotalBalance     map[common.Address]*big.Int   //key:tokenid
	AccBalance           map[common.Address]balanceMap //key:tokenid
	gOutIndex            map[common.Address]uint64     //key:tokenid
	txKeys               map[common.Hash]lkctypes.Key  //key:txHash,value tx_key
	mainUTXOAddress      string
	walletOpen           bool
	autoRefresh          bool
	account              *AccountBase
	keyImages            map[lkctypes.Key]uint64
	Transfers            transferContainer
	stop                 chan int
	walletDB             dbm.DB
	refreshBlockInterval time.Duration
	syncQuick            bool
}

// NewLinkAccount return a LinkAccount
func NewLinkAccount(walletDB dbm.DB, logger log.Logger, keystoreFile string, password string) (*LinkAccount, error) {
	la := &LinkAccount{
		remoteHeight:         new(big.Int).SetUint64(defaultInitBlockHeight),
		localHeight:          new(big.Int).SetUint64(defaultInitBlockHeight),
		utxoTotalBalance:     make(map[common.Address]*big.Int),
		AccBalance:           make(map[common.Address]balanceMap),
		gOutIndex:            make(map[common.Address]uint64),
		txKeys:               make(map[common.Hash]lkctypes.Key),
		mainUTXOAddress:      "",
		walletOpen:           false,
		autoRefresh:          false,
		keyImages:            make(map[lkctypes.Key]uint64),
		Transfers:            make(transferContainer, 0),
		stop:                 make(chan int, 1),
		walletDB:             walletDB,
		refreshBlockInterval: defaultRefreshBlockInterval,
	}

	la.account = NewUTXOAccount(keystoreFile, password)

	logModule := fmt.Sprintf("LinkAccount-%s", la.getEthAddress().String())
	// la.BaseService = *cmn.NewBaseService(logger, logModule, la)

	la.Logger = logger.With("module", logModule)

	la.mainUTXOAddress = la.account.GetKeys().Address
	la.setTokenBalanceBySubIndex(LinkToken, 0, big.NewInt(0))

	err := la.loadLocalHeight()
	if err != nil {
		return nil, err
	}

	err = la.loadGOutIndex()
	if err != nil {
		return nil, err
	}

	accSubCnt, err := la.loadAccountSubCnt()
	if err != nil {
		return nil, err
	}
	err = la.account.CreateSubAccountN(accSubCnt)
	if err != nil {
		return nil, err
	}
	err = la.loadTransfers()
	if err != nil {
		return nil, err
	}
	// err = la.loadTxKeys()
	// if err != nil {
	// 	return nil, err
	// }

	la.Logger.Info("NewLinkAccount", "account", la.account.EthAddress)

	la.walletOpen = true
	la.autoRefresh = true

	return la, nil
}

// OnStart starts the Wallet. It implements cmn.Service.
func (la *LinkAccount) OnStart() error {
	la.Logger.Info("starting LinkAccount")

	go la.refreshLoop()
	return nil
}

// OnStop stops the Wallet. It implements cmn.Service.
func (la *LinkAccount) OnStop() {
	la.lock.Lock()
	defer la.lock.Unlock()
	close(la.stop)

	la.walletOpen = false
	la.autoRefresh = false

	la.account.ZeroKey()

	la.Logger.Info("la.account.ZeroKey(), and stopping LinkAccount")
}

func (la *LinkAccount) getEthAddress() common.Address {
	return la.account.EthAddress
}

func (la *LinkAccount) getTokenBalanceBySubIndex(token common.Address, subIdx uint64) *big.Int {
	balanceMap, ok := la.AccBalance[token]
	if ok {
		balance, ok := balanceMap[subIdx]
		if !ok {
			return big.NewInt(0)
		}
		return new(big.Int).Set(balance)
	}
	return big.NewInt(0)
}

func (la *LinkAccount) setTokenBalanceBySubIndex(token common.Address, subIdx uint64, amount *big.Int) {
	_, ok := la.AccBalance[token]
	if !ok {
		la.AccBalance[token] = make(balanceMap)
	}
	la.AccBalance[token][subIdx] = new(big.Int).Set(amount)
}

func (la *LinkAccount) updateBalance(token common.Address, index uint64, bAdd bool, amount *big.Int) error {
	balance := la.getTokenBalanceBySubIndex(token, index)

	totalB, ok := la.utxoTotalBalance[token]
	if !ok {
		totalB = big.NewInt(0)
		la.utxoTotalBalance[token] = totalB
	}

	if bAdd {
		newBalance := new(big.Int).Add(balance, amount)
		la.setTokenBalanceBySubIndex(token, index, newBalance)

		la.utxoTotalBalance[token] = new(big.Int).Add(totalB, amount)
		la.Logger.Info("updateBalance add balance", "index", index, "before", balance, "add amount", amount, "after", newBalance, "utxoTotalBalance", la.utxoTotalBalance)
	} else {
		if balance.Cmp(amount) < 0 {
			la.Logger.Error("updateBalance sub err,balance < amount", "index", index, "balance", balance, "amount", amount)
			return fmt.Errorf("updateBalance sub err,balance < amount")
		}
		newBalance := new(big.Int).Sub(balance, amount)
		la.setTokenBalanceBySubIndex(token, index, newBalance)

		la.utxoTotalBalance[token] = new(big.Int).Sub(totalB, amount)
		la.Logger.Info("updateBalance sub balance", "index", index, "before", balance, "sub amount", amount, "after", newBalance, "utxoTotalBalance", la.utxoTotalBalance)
	}
	return nil
}

func (la *LinkAccount) printBalance() {
	if la.walletOpen {
		la.Logger.Debug("printBalance", "utxoTotalBalance", la.utxoTotalBalance[LinkToken])
	} else {
		la.Logger.Debug("printBalance", "walletOpen", la.walletOpen)
	}
}

func (la *LinkAccount) resetRemoteHeight() error {
	h, err := RefreshMaxBlock()
	if err != nil {
		la.Logger.Error("refreshLoop RefreshMaxBlock", "err", err)
		return err
	}
	if h.Cmp(la.remoteHeight) > 0 {
		la.remoteHeight = h
	}
	return nil
}

func (la *LinkAccount) refreshLoop() {
	refreshMaxBlock := time.NewTimer(la.refreshBlockInterval)

	for {
		select {
		case <-refreshMaxBlock.C:
			err := la.resetRemoteHeight()
			if err != nil {
				refreshMaxBlock.Reset(la.refreshBlockInterval)
				continue
			}

			if la.syncQuick {
				la.RefreshQuick()
			} else {
				la.printBalance()
				la.Refresh()
			}
			refreshMaxBlock.Reset(la.refreshBlockInterval)
		case <-la.stop:
			refreshMaxBlock.Stop()
			la.Logger.Info("refreshLoop", "msg", "la.stop", "EthAddress", la.getEthAddress())
			return
		}
	}
}

// checkBlock check block parent hash with local block hash
func (la *LinkAccount) checkBlock(block *rtypes.RPCBlock) error {
	height := (*big.Int)(block.Height)
	parentHash := block.ParentHash
	parentHeight := new(big.Int).Sub(height, big.NewInt(1))
	localParentHash, err := la.loadBlockHash(parentHeight)
	if err != nil {
		return err
	}
	if *localParentHash != parentHash {
		return types.ErrBlockParentHash
	}
	return nil
}

// Refresh wallet
func (la *LinkAccount) Refresh() {
	for {
		var block *rtypes.RPCBlock
		var err error

		if la.walletOpen && la.autoRefresh && la.localHeight.Cmp(la.remoteHeight) <= 0 {
			la.Logger.Debug("Refresh", "localHeight", la.localHeight, "remoteHeight", la.remoteHeight)
			block, err = GetBlockUTXOsByNumber(la.localHeight)
			if err != nil {
				la.Logger.Error("Refresh getBlockUTXOsByNumber fail", "height", la.localHeight, "err", err)
				return
			}
		}

		la.lock.Lock()

		if la.walletOpen && la.autoRefresh && la.localHeight.Cmp(la.remoteHeight) <= 0 {
			// check block parent hash
			err = la.checkBlock(block)
			if err != nil {
				la.Logger.Error("Refresh CheckBlock fail", "height", la.localHeight, "err", err)
				la.lock.Unlock()
				return
			}

			ids, myTxs, err := la.processBlock(block)
			if err != nil {
				la.Logger.Error("Refresh processBlock fail", "height", la.localHeight, "err", err)
				la.lock.Unlock()
				return
			}

			localBlock := &types.UTXOBlock{
				Height: (*hexutil.Big)(new(big.Int).Set(block.Height.ToInt())),
				Time:   (*hexutil.Big)(new(big.Int).Set(block.Time.ToInt())),
				Txs:    myTxs,
			}

			err = la.save(ids, *block.Hash, localBlock)
			if err != nil {
				la.Logger.Error("Refresh la.save fail", "height", la.localHeight, "err", err)

			}
			la.localHeight.Add(la.localHeight, big.NewInt(1))
			la.lock.Unlock()
		} else {
			la.lock.Unlock()
			return
		}
	}

}

// RefreshQuick wallet
func (la *LinkAccount) RefreshQuick() {
	for {
		var quickBlock *rtypes.QuickRPCBlock
		var err error
		if la.walletOpen && la.autoRefresh {
			la.Logger.Debug("RefreshQuick", "localHeight", la.localHeight, "remoteHeight", la.remoteHeight)

			quickBlock, err = GetBlockUTXO(la.localHeight)
			if err != nil {
				la.Logger.Info("RefreshQuick GetBlockUTXO fail", "height", la.localHeight, "err", err)
				return
			}
		}

		la.lock.Lock()

		if la.walletOpen && la.autoRefresh {
			nextHeight := quickBlock.NextHeight.ToInt()
			remoteHeight := quickBlock.MaxHeight.ToInt()

			if remoteHeight.Cmp(nextHeight) < 0 {
				la.Logger.Info("RefreshQuick remoteHeight.Cmp(nextHeight) < 0, not ready to refresh", "remoteHeight", remoteHeight, "nextHeight", nextHeight)
				la.lock.Unlock()
				return
			}

			if nextHeight.Cmp(la.localHeight) <= 0 {
				nextHeight.Add(la.localHeight, big.NewInt(1))
			}

			if quickBlock.Block == nil {
				// if quickBlock.NextHeight == nil || nextHeight.Sign() <= 0 {
				// 	la.Logger.Info("GetBlockUTXO not available NextHeight", "height", la.localHeight)
				// 	return
				// }
				la.localHeight.Set(nextHeight)
				la.remoteHeight.Set(remoteHeight)
				la.lock.Unlock()
				continue
			}

			ids, myTxs, err := la.processBlock(quickBlock.Block)
			if err != nil {
				la.Logger.Error("RefreshQuick processBlock fail", "height", la.localHeight, "err", err)
				la.lock.Unlock()
				return
			}

			localBlock := &types.UTXOBlock{
				Height: (*hexutil.Big)(new(big.Int).Set(quickBlock.Block.Height.ToInt())),
				Time:   (*hexutil.Big)(new(big.Int).Set(quickBlock.Block.Time.ToInt())),
				Txs:    myTxs,
			}

			err = la.save(ids, *quickBlock.Block.Hash, localBlock)
			if err != nil {
				la.Logger.Error("RefreshQuick la.save fail", "height", la.localHeight, "err", err)
				la.lock.Unlock()
				return
			}

			// if quickBlock.NextHeight == nil || nextHeight.Sign() <= 0 {
			// 	la.Logger.Info("GetBlockUTXO not available NextHeight", "height", la.localHeight)
			// 	return
			// }

			la.localHeight.Set(nextHeight)
			la.remoteHeight.Set(remoteHeight)

			la.lock.Unlock()
		} else {
			la.lock.Unlock()
			return
		}
	}
}

func (la *LinkAccount) processBlock(block *rtypes.RPCBlock) (ids []uint64, myTxs []types.UTXOTransaction, err error) {
	numTxs := len(block.Txs)
	la.Logger.Info("processBlock", "Height", block.Height, "numTxs", numTxs)

	for index := 0; index < numTxs; index++ {
		rpctx := block.Txs[index].(*rtypes.RPCTx)
		switch t := rpctx.Tx.(type) {
		case *tctypes.Transaction:
			// TODO
		case *tctypes.UTXOTransaction:
			tids, myTx, err := la.processNewTransaction(t, block.Height.ToInt().Uint64())
			if err != nil {
				return nil, nil, err
			}
			ids = append(ids, tids...)
			if myTx != nil {
				myTxs = append(myTxs, *myTx)
			}

		default:
			// la.Logger.Warn("processBlock unknown tx type", "type", block.Txs[index].TxType)
		}
	}

	return ids, myTxs, nil
}

func (la *LinkAccount) processNewTransaction(tx *tctypes.UTXOTransaction, height uint64) (tids []uint64, myTx *types.UTXOTransaction, err error) {
	la.Logger.Info("processNewTransaction", "height", height, "txhash", tx.Hash())

	myTx = &types.UTXOTransaction{}
	myTx.Inputs = make([]types.RPCInput, 0)
	myTx.Outputs = make([]types.RPCOutput, 0)
	myTx.TokenID = tx.TokenID
	myTx.Hash = tx.Hash()
	myTx.Fee = (*hexutil.Big)(new(big.Int).Set(tx.Fee))
	myTx.TxFlag = uint8(0)

	// output
	received := big.NewInt(0)
	outputID := -1
	outputCnt := len(tx.Outputs)
	needSaveTx := false
	for i := 0; i < outputCnt; i++ {
		o := tx.Outputs[i]
		received = big.NewInt(0)

		switch ro := o.(type) {
		case *tctypes.UTXOOutput:
			outputID++
			// TODO
			gid := la.increaseGOutIndex(tx.TokenID)

			keyMaps := make(map[lkctypes.KeyDerivation]lkctypes.PublicKey, 0)
			derivationKeys := make([]lkctypes.KeyDerivation, 0)
			derivationKey, err := xcrypto.GenerateKeyDerivation(tx.RKey, la.account.GetKeys().ViewSKey)
			if err != nil {
				la.Logger.Error("GenerateKeyDerivation fail", "rkey", tx.RKey, "err", err)
				continue
			}
			derivationKeys = append(derivationKeys, derivationKey)
			keyMaps[derivationKey] = tx.RKey
			if len(tx.AddKeys) > 0 {
				//we use a addinational key for utxo->account proof, maybe cause err here
				for _, addkey := range tx.AddKeys {
					derivationKey, err = xcrypto.GenerateKeyDerivation(addkey, la.account.GetKeys().ViewSKey)
					if err != nil {
						la.Logger.Info("GenerateKeyDerivation fail", "addkey", addkey, "err", err)
						continue
					}
					derivationKeys = append(derivationKeys, derivationKey)
					keyMaps[derivationKey] = addkey
				}
			}

			recIdx := uint64(outputID)
			realDeriKey, subaddrIndex, err := tctypes.IsOutputBelongToAccount(la.account.GetKeys(), la.account.KeyIndex, ro.OTAddr, derivationKeys, recIdx)
			if err != nil {
				la.Logger.Info("IsOutputBelongToAccount fail", "ro.OTAddr", ro.OTAddr, "derivationKey", derivationKey, "recIdx", recIdx, "err", err)
				continue
			}

			realRKey, exist := keyMaps[realDeriKey]
			if !exist {
				la.Logger.Error("real rkey not found", "real derivation key", realDeriKey)
				continue
			}
			la.Logger.Debug("processNewTransaction", "real derivation key", realDeriKey, "real random key", realRKey)
			secretKey, err := xcrypto.DeriveSecretKey(realDeriKey, outputID, la.account.GetKeys().SpendSKey)
			if err != nil {
				la.Logger.Error("DeriveSecretKey fail", "err", err)
				continue
			}
			sk1 := secretKey
			if subaddrIndex > 0 {
				subaddrSk := xcrypto.GetSubaddressSecretKey(la.account.GetKeys().ViewSKey, uint32(subaddrIndex))
				sk1 = xcrypto.SecretAdd(secretKey, subaddrSk)
			}
			keyImage, err := xcrypto.GenerateKeyImage(lkctypes.PublicKey(ro.OTAddr), sk1)
			if err != nil {
				la.Logger.Error("GenerateKeyImage fail", "otaddr", ro.OTAddr, "err", err)
				continue
			}

			ecdh := &lkctypes.EcdhTuple{
				Mask:   tx.RCTSig.RctSigBase.EcdhInfo[outputID].Mask,
				Amount: tx.RCTSig.RctSigBase.EcdhInfo[outputID].Amount,
			}
			la.Logger.Debug("GenerateKeyDerivation", "derivationKey", realDeriKey, "amount", tx.RCTSig.RctSigBase.EcdhInfo[outputID].Amount)
			scalar, err := xcrypto.DerivationToScalar(realDeriKey, outputID)
			if err != nil {
				la.Logger.Error("DerivationToScalar fail", "derivationKey", realDeriKey, "outputID", outputID, "err", err)
				continue
			}

			ok := xcrypto.EcdhDecode(ecdh, lkctypes.Key(scalar), false)
			if !ok {
				la.Logger.Error("EcdhDecode fail", "err", err)
				continue
			}
			needSaveTx = true

			//check encode amount is valid
			outAmountKeys := []lkctypes.Key{ecdh.Amount}
			outMKeys := lkctypes.KeyV{lkctypes.Key(scalar)}
			_, tCommits, _, err := ringct.ProveRangeBulletproof(outAmountKeys, outMKeys)
			if err != nil {
				la.Logger.Error("ringct.ProveRangeBulletproof fail", "err", err)
				continue
			}
			if len(tCommits) != 1 {
				la.Logger.Error("ringct.ProveRangeBulletproof commits len not expect", "len", len(tCommits))
				continue
			}
			tMask, _ := ringct.Scalarmult8(tCommits[0])
			if !bytes.Equal(tx.RCTSig.OutPk[outputID].Mask[:], tMask[:]) {
				la.Logger.Error("ringct.ProveRangeBulletproof check encode amount invalid")
				continue
			}

			uod := tctypes.UTXOOutputDetail{}
			uod.BlockHeight = height
			uod.TxID = lkctypes.Hash(tx.Hash())
			uod.OutIndex = uint64(outputID)
			uod.GlobalIndex = gid
			uod.Spent = false
			uod.Frozen = false
			uod.SpentHeight = uint64(0)
			uod.KeyImage = lkctypes.Key(keyImage)
			uod.SubAddrIndex = subaddrIndex
			uod.RKey = realRKey
			uod.Mask = ecdh.Mask
			uod.Amount = big.NewInt(0).Mul(tctypes.Hash2BigInt(ecdh.Amount), big.NewInt(tctypes.UTXO_COMMITMENT_CHANGE_RATE))
			la.Logger.Debug("processNewTransaction output", "ro.Amount", ro.Amount.String(), "ecdh.Amount", ecdh.Amount.String(), "outputID", outputID, "scalar", scalar)
			uod.Remark = ro.Remark
			hash := crypto.Sha256(scalar[:])
			for k := 0; k < 32; k++ {
				uod.Remark[k] ^= hash[k]
			}

			la.Transfers = append(la.Transfers, &uod)
			tid := len(la.Transfers) - 1

			la.keyImages[uod.KeyImage] = uint64(tid)
			tids = append(tids, uint64(tid))

			myTx.Outputs = append(myTx.Outputs, types.UTXOOutput{OTAddr: (common.Hash)(ro.OTAddr), GlobalIndex: (hexutil.Uint64)(tid)})
			myTx.TxFlag = myTx.TxFlag | txUout

			la.Logger.Info("processNewTransaction output", "KeyImage", uod.KeyImage.String(), "subaddrIndex", subaddrIndex, "tx.RKey", tx.RKey, "uod.Amount", uod.Amount.String())

			received = new(big.Int).Add(received, uod.Amount)
			la.updateBalance(tx.TokenID, uod.SubAddrIndex, true, uod.Amount)

		case *tctypes.AccountOutput:
			if bytes.Equal(ro.To[:], la.account.EthAddress[:]) {
				needSaveTx = true
				myTx.TxFlag = myTx.TxFlag | txAout
			}
			myTx.Outputs = append(myTx.Outputs, types.AccountOutput{To: ro.To, Amount: (*hexutil.Big)(ro.Amount), Data: (hexutil.Bytes)(ro.Data)})

		default:
		}
	}

	// input
	txMoneySpentInIns := big.NewInt(0)
	for _, i := range tx.Inputs {
		switch ri := i.(type) {
		case *tctypes.UTXOInput:

			gidx := uint64(0)

			keyimage := ri.KeyImage
			iTransfer, ok := la.keyImages[keyimage]
			if ok {
				needSaveTx = true
				myTx.TxFlag = myTx.TxFlag | txUin

				uod := la.Transfers[iTransfer]
				amount := uod.Amount
				uod.Spent = true
				uod.SpentHeight = height

				txMoneySpentInIns = new(big.Int).Add(txMoneySpentInIns, amount)
				tids = append(tids, iTransfer)
				la.updateBalance(tx.TokenID, uod.SubAddrIndex, false, amount)
				la.Logger.Info("processNewTransaction", "utxoTotalBalance", la.utxoTotalBalance, "iTransfer", iTransfer, "amount", amount.String())
				gidx = uod.GlobalIndex
			}
			myTx.Inputs = append(myTx.Inputs, types.UTXOInput{GlobalIndex: hexutil.Uint64(gidx)})
		case *tctypes.AccountInput:
			from, err := tx.From()
			if err != nil {
				la.Logger.Error("get from address fail", "err", err)
				continue
			}
			if bytes.Equal(from[:], la.account.EthAddress[:]) {
				needSaveTx = true
				myTx.TxFlag = myTx.TxFlag | txAin
			}
			myTx.Inputs = append(myTx.Inputs, types.AccountInput{From: from, Nonce: hexutil.Uint64(ri.Nonce), Amount: (*hexutil.Big)(ri.Amount)})

		default:

		}
	}
	if needSaveTx {
		err = la.saveUTXOTx(tx)
		if err != nil {
			la.Logger.Error("processNewTransaction saveUTXOTx fail", "err", err)
		}
		return tids, myTx, nil
	}
	return tids, nil, nil
}

// increaseGOutIndex increase outindex,return curr idx
func (la *LinkAccount) increaseGOutIndex(token common.Address) uint64 {
	_, ok := la.gOutIndex[token]
	if !ok {
		la.gOutIndex[token] = 0
	} else {
		la.gOutIndex[token] = la.gOutIndex[token] + 1
	}
	return la.gOutIndex[token]
}

// GetBalance rpc get balance
func (la *LinkAccount) GetBalance(index uint64, token *common.Address) *big.Int {
	if index >= uint64(len(la.account.Keys)) {
		return big.NewInt(0)
	}
	la.lock.Lock()
	defer la.lock.Unlock()
	return la.getTokenBalanceBySubIndex(*token, index)
}

// GetAddress rpc get address
func (la *LinkAccount) GetAddress(index uint64) (string, error) {
	// if !la.walletOpen {
	// 	return "", types.ErrWalletNotOpen
	// }

	if index >= uint64(len(la.account.Keys)) {
		return "", fmt.Errorf("err: index can not greater than %d", len(la.account.Keys)-1)
	}
	addr := la.account.Keys[index].Address

	return addr, nil
}

// GetHeight rpc get height
func (la *LinkAccount) GetHeight() (localHeight *big.Int, remoteHeight *big.Int) {
	la.lock.Lock()
	defer la.lock.Unlock()

	if la.localHeight.Cmp(new(big.Int).SetUint64(defaultInitBlockHeight)) == 0 {
		return la.localHeight, la.remoteHeight
	}
	return new(big.Int).Sub(la.localHeight, big.NewInt(1)), la.remoteHeight
}

// CreateSubAccount return new sub address and sub index
func (la *LinkAccount) CreateSubAccount(maxSub uint64) error {
	// if !la.walletOpen {
	// 	return types.ErrWalletNotOpen
	// }
	if maxSub > defaultMaxSubAccount {
		return types.ErrSubAccountTooLarge
	}

	subCnt := uint64(len(la.account.Keys) - 1)

	if maxSub > subCnt {
		addCnt := maxSub - subCnt
		for i := uint64(0); i < addCnt; i++ {
			_, idx, err := la.account.CreateSubAccount()
			if err == nil {
				la.setTokenBalanceBySubIndex(LinkToken, idx, big.NewInt(0))
			}
		}
		batch := la.walletDB.NewBatch()
		if la.saveAccountSubCnt(batch) != nil || batch.Commit() != nil {
			return types.ErrSaveAccountSubCnt
		}
		la.Logger.Debug("CreateSubAccount", "account", la.account.String())
	}
	return nil
}

// AutoRefreshBlockchain set autoRefresh
func (la *LinkAccount) AutoRefreshBlockchain(autoRefresh bool) error {
	// if !la.walletOpen {
	// 	return types.ErrWalletNotOpen
	// }

	la.autoRefresh = autoRefresh
	la.Logger.Info("AutoRefreshBlockchain", "la.autoRefresh", la.autoRefresh)
	return nil
}

// GetAccountInfo return eth_account and utxo_accounts
func (la *LinkAccount) GetAccountInfo(tokenID *common.Address) (*types.GetAccountInfoResult, error) {
	// if !la.walletOpen {
	// 	return nil, types.ErrWalletNotOpen
	// }

	var ret types.GetAccountInfoResult
	count := len(la.account.Keys)
	utxo := make([]types.UTXOAccount, count)
	totalBalance := big.NewInt(0)
	for i := uint64(0); i < uint64(count); i++ {
		ba := la.GetBalance(i, tokenID)
		utxo[i] = types.UTXOAccount{Address: la.account.Keys[i].Address, Index: hexutil.Uint64(i), Balance: (*hexutil.Big)(ba)}
		totalBalance.Add(totalBalance, ba)
	}
	ret.UTXOAccounts = utxo
	eth := types.EthAccount{Address: la.account.EthAddress}
	balance, err := GetTokenBalance(la.account.EthAddress, *tokenID)
	if err != nil {
		balance = big.NewInt(0)
		la.Logger.Error("GetAccounts getTokenBalance fail", "err", err)
	}
	nonce, err := EthGetTransactionCount(la.account.EthAddress)
	if err != nil {
		u := uint64(0)
		nonce = &u
		la.Logger.Error("GetAccounts EthGetTransactionCount fail", "err", err)
	}
	totalBalance.Add(totalBalance, balance)
	eth.Balance = (*hexutil.Big)(balance)
	eth.Nonce = hexutil.Uint64(*nonce)
	ret.EthAccount = eth
	ret.TotalBalance = (*hexutil.Big)(totalBalance)
	ret.TokenID = tokenID

	return &ret, nil
}

// RescanBlockchain ,reset wallet block and transfer info
func (la *LinkAccount) RescanBlockchain() error {
	if !la.walletOpen {
		return types.ErrWalletNotOpen
	}
	la.Logger.Debug("RescanBlockchain")

	la.lock.Lock()
	defer la.lock.Unlock()

	accCnt := len(la.AccBalance)
	for i := 0; i < accCnt; i++ {
		la.AccBalance = make(map[common.Address]balanceMap)
	}

	la.localHeight.SetUint64(defaultInitBlockHeight)
	la.remoteHeight.SetUint64(defaultInitBlockHeight)
	la.utxoTotalBalance = make(map[common.Address]*big.Int)
	la.gOutIndex = make(map[common.Address]uint64)
	la.keyImages = make(map[lkctypes.Key]uint64)
	la.Transfers = make(transferContainer, 0)
	return nil
}

// GetGOutIndex return curr idx
func (la *LinkAccount) GetGOutIndex(token common.Address) uint64 {
	la.lock.Lock()
	defer la.lock.Unlock()

	_, ok := la.gOutIndex[token]
	if !ok {
		return 0
	}
	return la.gOutIndex[token]
}

// Status return wallet status
func (la *LinkAccount) Status() *types.StatusResult {
	lh, rh := la.GetHeight()
	ethAddress := common.EmptyAddress
	if la.walletOpen {
		ethAddress = la.account.EthAddress
	}
	chainVersion, err := GetChainVersion()
	if err != nil {
		la.Logger.Error("Status getChainVersion fail", "err", err)
		chainVersion = "0.0.0"
	}
	refreshBlockInterval := la.refreshBlockInterval / time.Second

	return &types.StatusResult{
		LocalHeight:          (*hexutil.Big)(lh),
		RemoteHeight:         (*hexutil.Big)(rh),
		WalletOpen:           la.walletOpen,
		AutoRefresh:          la.autoRefresh,
		WalletVersion:        WalletVersion,
		ChainVersion:         chainVersion,
		EthAddress:           ethAddress,
		RefreshBlockInterval: refreshBlockInterval,
		InitBlockHeight:      (hexutil.Uint64)(defaultInitBlockHeight),
	}
}

// GetTxKey return transaction's tx secKey
func (la *LinkAccount) GetTxKey(hash *common.Hash) (*lkctypes.Key, error) {
	// if !la.walletOpen {
	// 	return nil, types.ErrWalletNotOpen
	// }
	// txKey, ok := la.txKeys[*hash]
	// if ok {
	// 	return &txKey, nil
	// }
	// return nil, types.ErrNotFoundTxKey

	itr := la.walletDB.NewIteratorWithPrefix(hash[:])
	defer itr.Close()

	for ; itr.Valid(); itr.Next() {
		v := itr.Value()
		var key lkctypes.Key
		copy(key[:], v)
		return &key, nil
	}
	return nil, types.ErrNotFoundTxKey
}

// GetMaxOutput return tokenID max output idx
func (la *LinkAccount) GetMaxOutput(tokenID common.Address) (*hexutil.Uint64, error) {
	// if !la.walletOpen {
	// 	return nil, types.ErrWalletNotOpen
	// }
	idx, ok := la.gOutIndex[tokenID]
	if !ok {
		idx = 0
	} else {
		idx++
	}
	return (*hexutil.Uint64)(&idx), nil
}

// GetUTXOTx return UTXOTransaction
func (la *LinkAccount) GetUTXOTx(hash common.Hash) (*tctypes.UTXOTransaction, error) {
	// if !la.walletOpen {
	// 	return nil, types.ErrWalletNotOpen
	// }
	return la.loadUTXOTx(hash)
}

// SetRefreshBlockInterval set refreshBlockInterval
func (la *LinkAccount) SetRefreshBlockInterval(interval time.Duration) {
	la.refreshBlockInterval = interval
	la.Logger.Info("SetRefreshBlockInterval", "refreshBlockInterval", la.refreshBlockInterval)
}

// GetLocalUTXOTxsByHeight return UTXOTransaction
func (la *LinkAccount) GetLocalUTXOTxsByHeight(height *big.Int) (*types.UTXOBlock, error) {
	la.lock.Lock()
	defer la.lock.Unlock()

	block, err := la.loadBlockTxs(height)
	if err == types.ErrBlockNotFound && height.Cmp(la.localHeight) <= 0 {
		err = nil
	}
	return block, err
}

// GetLocalOutputs return UTXOTransaction
func (la *LinkAccount) GetLocalOutputs(ids []hexutil.Uint64) ([]types.UTXOOutputDetail, error) {
	var err error
	outputs := make([]types.UTXOOutputDetail, 0)
	for _, nextid := range ids {
		var o *tctypes.UTXOOutputDetail
		o, err = la.loadOutputDetail(uint64(nextid))
		if err != nil {
			la.Logger.Info("GetLocalOutputs", "id", nextid, "err", err)
			if err == types.ErrOutputNotFound {
				err = nil
				continue
			}
			break
		}
		rpcUTXOOutputDetail := types.UTXOOutputDetail{
			GlobalIndex:  (hexutil.Uint64)(o.GlobalIndex),
			Amount:       (*hexutil.Big)(o.Amount),
			SubAddrIndex: (hexutil.Uint64)(o.SubAddrIndex),
			TokenID:      o.TokenID,
			Remark:       (hexutil.Bytes)(o.Remark[:]),
		}
		outputs = append(outputs, rpcUTXOOutputDetail)
	}
	return outputs, err
}

// SetSyncQuick set la.syncQuick
func (la *LinkAccount) SetSyncQuick(quick bool) {
	la.syncQuick = quick
}

func (la *LinkAccount) GetUTXOAddInfo(hash common.Hash) (*types.UTXOAddInfo, error) {
	return la.loadAddInfo(hash)
}

func (la *LinkAccount) DelUTXOAddInfo(hash common.Hash) error {
	return la.delAddInfo(hash)
}
