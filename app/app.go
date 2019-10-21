package app

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"runtime"
	"sync"

	"github.com/lianxiangcloud/linkchain/blockchain"
	"github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	lctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p"
	"github.com/lianxiangcloud/linkchain/libs/txmgr"
	"github.com/lianxiangcloud/linkchain/metrics"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/utxo"
	"github.com/lianxiangcloud/linkchain/vm/evm"
	"github.com/lianxiangcloud/linkchain/vm/wasm"
	"github.com/xunleichain/tc-wasm/vm"
)

const (
	// maxTransactionSize is 32KB in order to prevent DOS attacks
	// maxTransactionSize is 256KB for wasm
	maxTransactionSize = 256 * 1024
)

var _ types.TxCensor = &LinkApplication{}

// Processor is an interface for processing blocks using a given initial state.
//
// Process takes the block to be processed and the statedb upon which the
// initial state is based. It should return the receipts generated, amount
// of gas used in the process and return an error if any of the internal rules
// failed.
type Processor interface {
	Process(block *types.Block, statedb *state.StateDB, cfg evm.Config) (types.Receipts, []*types.Log, uint64, []types.Tx, []*types.UTXOOutputData, []*lctypes.Key, error)
}

type ProcessResult struct {
	logs      []*types.Log
	receipts  *types.Receipts
	txsResult types.TxsResult
	tmpState  *state.StateDB
	height    uint64
	isOk      bool
}

type PoceedHandle func(wasm *wasm.WASM, coinbase common.Address, amount *big.Int, logger log.Logger) error
type AwardHandle func(wasm *wasm.WASM, logger log.Logger) error

func (p *ProcessResult) GetReceipts() *types.Receipts {
	return p.receipts
}

func (p *ProcessResult) GetTxsResult() *types.TxsResult {
	return &p.txsResult
}

type LinkApplication struct {
	logger             log.Logger
	mempool            types.Mempool
	processor          Processor // block processor interface
	vmConfig           evm.Config
	blockChain         *blockchain.BlockStore
	balanceRecordStore *blockchain.BalanceRecordStore
	utxoStore          *utxo.UtxoStore
	currentBlock       *types.Block
	storeState         *state.StateDB
	checkTxState       *state.StateDB
	stateLock          sync.Mutex
	crossState         txmgr.CrossState
	eventbus           *types.EventBus
	nodeType           string
	coinbase           common.Address

	lastValChanegHeight uint64
	lastVals            []*types.Validator
	lastCoe             *types.Coefficient
	lastTxsResult       types.TxsResult
	conManager          *p2p.ConManager
	processLock         sync.Mutex
	processMap          map[common.Hash]*ProcessResult
	poceedHandle        PoceedHandle
	awardHandle         AwardHandle
}

func NewLinkApplication(db dbm.DB, bc *blockchain.BlockStore, utxoStore *utxo.UtxoStore,
	txService txmgr.CrossState, eventbus *types.EventBus, isTrie bool, brs *blockchain.BalanceRecordStore, poceedHandle PoceedHandle, awardHandle AwardHandle) (*LinkApplication, error) {
	currentBlock := bc.LoadBlock(bc.Height())
	if currentBlock == nil {
		return nil, types.ErrUnknownBlock
	}

	txsResult, err := bc.LoadTxsResult(bc.Height())
	if err != nil {
		return nil, err
	}

	storeState, err := state.New(txsResult.TrieRoot, state.NewKeyValueDBWithCache(db, 128, isTrie, bc.Height()))
	if err != nil {
		return nil, err
	}

	app := &LinkApplication{
		logger:             log.NewNopLogger(),
		vmConfig:           evm.Config{EnablePreimageRecording: false},
		blockChain:         bc,
		balanceRecordStore: brs,
		utxoStore:          utxoStore,
		currentBlock:       currentBlock,
		storeState:         storeState,
		checkTxState:       storeState.Copy(),
		crossState:         txService,
		eventbus:           eventbus,

		lastTxsResult: *txsResult,
		processMap:    make(map[common.Hash]*ProcessResult, 4),
		poceedHandle:  poceedHandle,
		awardHandle:   awardHandle,
	}
	app.processor = NewStateProcessor(bc, app)
	app.lastCoe = GetCoefficient(app.storeState, app.logger)
	return app, nil
}

func (app *LinkApplication) GetLastChangedVals() (height uint64, vals []*types.Validator) {
	app.LockState()
	defer app.UnlockState()
	return app.lastValChanegHeight, app.lastVals
}

func (app *LinkApplication) SetLastChangedVals(height uint64, vals []*types.Validator) {
	app.LockState()
	defer app.UnlockState()
	app.lastValChanegHeight, app.lastVals = height, vals
}

func (app *LinkApplication) SetLogger(l log.Logger) {
	app.logger = l
}

func (app *LinkApplication) SetMempool(mempool types.Mempool) {
	app.mempool = mempool
}

func (app *LinkApplication) SetConm(conM *p2p.ConManager) {
	app.conManager = conM
}

func (app *LinkApplication) TxMgr() types.TxMgr {
	return app.crossState
}
func (app *LinkApplication) State() types.State {
	return app.checkTxState
}
func (app *LinkApplication) Block() *types.Block {
	return app.currentBlock
}
func (app *LinkApplication) LockState() {
	app.stateLock.Lock()
}

func (app *LinkApplication) UnlockState() {
	app.stateLock.Unlock()
}

func (app *LinkApplication) BlockChain() types.BlockChain {
	return app.blockChain
}
func (app *LinkApplication) UTXOStore() types.UTXOStore {
	return app.utxoStore
}
func (app *LinkApplication) Mempool() types.Mempool {
	return app.mempool
}

func (app *LinkApplication) IsWasmContract(data []byte) bool {
	return wasm.IsWasmContract(data)
}
func (app *LinkApplication) GetUTXOGas() uint64 {
	return app.lastCoe.UTXOFee.Uint64()
}

func (app *LinkApplication) CreateBlock(height uint64, maxTxs int, gasLimit uint64, timeUnix uint64) *types.Block {
	app.logger.Info("CreateBlock: start", "height", height, "maxTxs", maxTxs)
	if height != app.currentBlock.Height+1 {
		app.logger.Error("CreateBlock: mismatched height", "lastBlockHeight", app.currentBlock.Height, "height", height)
		return nil
	}

	txs := app.mempool.Reap(maxTxs)
	numTxs := uint64(len(txs))

	block := &types.Block{
		Header: &types.Header{
			Height:      height,
			Time:        timeUnix,
			NumTxs:      numTxs,
			TotalTxs:    app.currentBlock.TotalTxs + numTxs,
			ParentHash:  app.currentBlock.Hash(),
			StateHash:   app.lastTxsResult.StateHash,
			ReceiptHash: app.lastTxsResult.ReceiptHash,
			GasLimit:    gasLimit,
			GasUsed:     app.lastTxsResult.GasUsed,
		},
		Data: &types.Data{
			Txs: txs,
		},
	}
	block.DataHash = block.Data.Hash()

	app.logger.Info("CreateBlock: done", "height", height, "dataHash", block.DataHash, "NumTxs", len(txs))
	return block
}

func (app *LinkApplication) PreRunBlock(block *types.Block) {
	app.logger.Info("PreRunBlock: begin", "height", block.Height, "NumTxs", block.NumTxs)
	processResult := ProcessResult{
		tmpState: app.storeState.Copy(),
		height:   block.Height,
		isOk:     false,
	}

	app.processBlock(block, &processResult, true)
	if !processResult.isOk {
		app.logger.Error("PreRunBlock: processBlock fail, should not happen!!!")
		panic("PreRunBlock: processBlock fail, should not happen!!!")
	}

	block.Header.StateHash = processResult.txsResult.StateHash
	block.Header.ReceiptHash = processResult.txsResult.ReceiptHash
	block.Header.GasUsed = processResult.txsResult.GasUsed
	app.logger.Info("PreRunBlock: done", "height", block.Height, "NumTxs", block.NumTxs)
}

func (app *LinkApplication) CheckBlock(block *types.Block) bool {
	blockHash := block.Hash()
	dataHash := block.Data.Hash()
	app.logger.Debug("CheckBlock: start", "height", block.Height, "blockHash", blockHash, "dataHash", dataHash)

	// 1. prev check
	if block.Height != app.currentBlock.Height+1 {
		app.logger.Error("CheckBlock: mismatched height", "lastBlockHeight", app.currentBlock.Height, "height", block.Height)
		return false
	}

	if int(block.Time()) <= int(app.currentBlock.Time())-300 {
		app.logger.Error("CheckBlock: The new block time is 300s earlier than the last block time", "last Block time", app.currentBlock.Time(), "new block time", block.Time())
		app.logger.Report("CheckBlock", "logID", types.LogIdBlockTimeError, "height", block.Height, "last Block time", app.currentBlock.Time(), "current block time", block.Time())
		return false
	}

	if block.DataHash != dataHash {
		app.logger.Error("CheckBlock: mismatched dataHash", "want", dataHash, "got", block.DataHash)
		return false
	}

	parentBlockHash := app.currentBlock.Hash()
	if block.Header.ParentHash != parentBlockHash {
		app.logger.Error("CheckBlock: mismatched parentHash", "want", parentBlockHash, "got", block.Header.ParentHash)
		return false
	}

	if err := app.verifySpecTxSign(block); err != nil {
		app.logger.Error("CheckBlock: verify signature failed:", "err", err)
		return false
	}

	// 2. check process result
	app.processLock.Lock()
	processResult := app.processMap[blockHash]
	if processResult == nil {
		processResult = &ProcessResult{
			tmpState: app.storeState.Copy(),
			height:   block.Height,
			isOk:     false,
		}

		app.processBlock(block, processResult, false)
		app.processMap[blockHash] = processResult
	}
	app.processLock.Unlock()

	if !processResult.isOk {
		app.logger.Warn("CheckBlock: process result is fail")
		return false
	}

	// 3. post check
	if block.Header.GasUsed != processResult.txsResult.GasUsed {
		app.logger.Error("CheckBlock: mismatched gasUsed", "want", processResult.txsResult.GasUsed, "got", block.Header.GasUsed, "block", block.String())
		return false
	}
	if block.Header.StateHash != processResult.txsResult.StateHash {
		app.logger.Error("CheckBlock: mismatched stateHash", "want", processResult.txsResult.StateHash, "got", block.Header.StateHash, "block", block.String())
		return false
	}
	if block.Header.ReceiptHash != processResult.txsResult.ReceiptHash {
		app.logger.Error("CheckBlock: mismatched receiptHash", "want", processResult.txsResult.ReceiptHash, "got", block.Header.ReceiptHash, "block", block.String())
		return false
	}
	return true
}

func (app *LinkApplication) verifySpecTxSign(block *types.Block) error {
	for _, tx := range block.Txs {
		var err error
		switch data := tx.(type) {
		case *types.ContractCreateTx:
			if app.mempool.GetTxFromCache(data.Hash()) == nil {
				err = data.VerifySign(app.crossState.GetMultiSignersInfo(types.TxContractCreateType))
			}
		case *types.ContractUpgradeTx:
			if app.mempool.GetTxFromCache(data.Hash()) == nil {
				err = data.VerifySign(app.crossState.GetMultiSignersInfo(types.TxContractCreateType))
			}
		case *types.MultiSignAccountTx:
			if app.mempool.GetTxFromCache(data.Hash()) == nil {
				_, vals := app.GetLastChangedVals()
				valSets := types.NewValidatorSet(vals)
				err = data.VerifySign(valSets)
			}
		default:
		}

		if err != nil {
			return err
		}
	}
	return nil
}

func (app *LinkApplication) verifyTxsOnProcess(block *types.Block) error {
	var wg sync.WaitGroup
	offset := (runtime.NumCPU() + 3) >> 2
	txs := block.Data.Txs
	size := len(txs)
	errRets := make([]*error, offset)
	for i := 0; i < offset; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			coIndex := index
			for ; index < size; index += offset {
				hash := txs[index].Hash()
				switch tx := txs[index].(type) {
				case *types.Transaction:
					cacheTx := app.mempool.GetTxFromCache(hash)
					if cacheTx != nil {
						from, _ := cacheTx.From()
						tx.StoreFrom(from)
					} else {
						_, err := tx.From()
						if err != nil {
							errRets[coIndex] = &err
							return
						}
					}
					err := checkBlacklistAddress(tx)
					if err != nil {
						errRets[coIndex] = &err
						return
					}
				case *types.TokenTransaction:
					cacheTx := app.mempool.GetTxFromCache(hash)
					if cacheTx != nil {
						from, _ := cacheTx.From()
						tx.StoreFrom(from)
					} else {
						_, err := tx.From()
						if err != nil {
							errRets[coIndex] = &err
							return
						}
					}
					err := checkBlacklistAddress(tx)
					if err != nil {
						errRets[coIndex] = &err
						return
					}

				case *types.UTXOTransaction:
					if cacheTx := app.mempool.GetTxFromCache(hash); cacheTx == nil {
						err := app.CheckTx(tx, true) //UTXO CheckBasic
						if err != nil {
							errRets[coIndex] = &err
							return
						}
					} else {
						from, _ := cacheTx.From()
						tx.StoreFrom(from)
					}

					// blacklist check
					if (tx.UTXOKind() & types.Aout) == types.Aout {
						for _, out := range tx.Outputs {
							switch aOutput := out.(type) {
							case *types.AccountOutput:
								if types.BlacklistInstance.IsBlackAddress(aOutput.To, tx.TokenID) {
									errRets[coIndex] = &types.ErrBlacklistAddress
									return
								}
							}
						}
					}
					if (tx.UTXOKind() & types.Ain) == types.Ain {
						fromAddr, err := tx.From()
						if err != nil {
							fromAddr = common.EmptyAddress
						}
						if types.BlacklistInstance.IsBlackAddress(fromAddr, tx.TokenID) {
							errRets[coIndex] = &types.ErrBlacklistAddress
							return
						}
					}
				case *types.ContractCreateTx, *types.ContractUpgradeTx:
					err := checkBlacklistAddress(tx)
					if err != nil {
						errRets[coIndex] = &err
						return
					}
				default:
				}

			}
		}(i)
	}

	wg.Wait()
	for _, err := range errRets {
		if err != nil {
			return *err
		}
	}
	return nil
}

func checkBlacklistAddress(tx types.Tx) error {
	var fromAddr, toAddr common.Address
	fromAddr, err := tx.From()
	if err != nil {
		fromAddr = common.EmptyAddress
	}
	if tx.To() != nil {
		toAddr = *tx.To()
	} else {
		toAddr = common.EmptyAddress
	}
	if types.BlacklistInstance.IsBlackAddress(fromAddr, toAddr, tx.(types.RegularTx).TokenAddress()) {
		return types.ErrBlacklistAddress
	}
	return nil
}

// preRun: only true on CreateBlock, false on CheckBlock
func (app *LinkApplication) processBlock(block *types.Block, processResult *ProcessResult, preRun bool) {
	app.logger.Info("processBlock: start", "preRun", preRun, "height", block.Height, "blockHash", block.Hash(), "dataHash", block.DataHash, "lastStateHash", app.lastTxsResult.StateHash)

	if !preRun {
		if err := app.verifyTxsOnProcess(block); err != nil {
			app.logger.Error("processBlock: verifyTxsOnProcess fail", "blockHash", block.Hash(), "err", err)
			return
		}
		app.logger.Info("processBlock: verifyTxsOnProcess done", "height", block.Height, "blockHash", block.Hash())
	}
	types.BlockBalanceRecordsInstance.Reset()
	receipts, logs, gasUsed, specialTxs, utxoOutputs, keyImages, err := app.processor.Process(block, processResult.tmpState, app.vmConfig)
	if err != nil {
		app.logger.Error("processBlock: process failed when Process", "blockHash", block.Hash(), "err", err)
		return
	}

	contextWasm := wasm.NewWASMContext(types.CopyHeader(block.Header), app.blockChain, nil, config.WasmGasRate)
	wasm := wasm.NewWASM(contextWasm, processResult.tmpState, evm.Config{EnablePreimageRecording: false})

	if gasUsed > 0 && app.poceedHandle != nil {
		totalGasFee := new(big.Int).Mul(new(big.Int).SetUint64(gasUsed), new(big.Int).SetInt64(types.ParGasPrice))
		app.logger.Info("processHandle", "foundation_addr", config.ContractFoundationAddr.String(), "totalGasFee", totalGasFee.String())
		processResult.tmpState.AddBalance(config.ContractFoundationAddr, totalGasFee)
		if err := app.poceedHandle(wasm, block.Coinbase(), totalGasFee, app.logger); err != nil {
			app.logger.Error("processBlock: process failed when setPoceeds", "blockHash", block.Hash(), "err", err)
			//return
		}
	}

	if block.Height%(10*app.lastCoe.VotePeriod) == 0 && len(app.lastTxsResult.Candidates) > 0 && app.awardHandle != nil {
		app.logger.Info("awardHandle")
		if err := app.awardHandle(wasm, app.logger); err != nil {
			app.logger.Error("processBlock: process failed when allocAward", "blockHash", block.Hash(), "err", err)
			//return
		}
	}

	processResult.logs = logs
	processResult.receipts = &receipts
	processResult.txsResult.GasUsed = gasUsed
	processResult.txsResult.ReceiptHash = receipts.Hash()
	processResult.txsResult.SetCandidates(app.lastTxsResult.Candidates)

	app.processBlockEvidence(block.Evidence.Evidence, processResult)

	processResult.txsResult.StateHash = processResult.tmpState.IntermediateRoot(false)
	processResult.txsResult.LogsBloom = types.CreateBloom(receipts)
	processResult.txsResult.SetSpecialTxs(specialTxs)
	processResult.txsResult.SetUTXOOutputs(utxoOutputs)
	processResult.txsResult.SetKeyImages(keyImages)

	app.logger.Info("processBlock: process done", "preRun", preRun, "height", block.Height, "blockHash", block.Hash(), "dataHash", block.DataHash, "tmpStateHash", processResult.txsResult.StateHash, "receiptHash", processResult.txsResult.ReceiptHash)
	processResult.isOk = true
}

func (app *LinkApplication) clearProcessResult(height uint64) {
	app.processLock.Lock()
	for blockHash, processResult := range app.processMap {
		if processResult.height == height {
			delete(app.processMap, blockHash)
		}
	}
	if len(app.processMap) > 4 {
		newMap := make(map[common.Hash]*ProcessResult, 4)
		for blockHash, processResult := range app.processMap {
			if processResult.height > height {
				newMap[blockHash] = processResult
			}
		}
		app.processMap = newMap
	}
	app.processLock.Unlock()
}

// CheckProcessResult check current block process result, must be executed after CheckBlock in FastSync
// XXX DEPRECATED
/*
func (app *LinkApplication) CheckProcessResult(blockHash common.Hash, txsResult *types.TxsResult) bool {
	app.logger.Info("CheckProcessResult: start", "blockHash", blockHash)

	app.processLock.Lock()
	processResult := app.processMap[blockHash]
	app.processLock.Unlock()

	if processResult == nil {
		app.logger.Info("CheckProcessResult: processBlock goroutine is not exist", "blockHash", blockHash)
		return false
	}

	if !processResult.isOk {
		app.logger.Info("CheckProcessResult: processBlock failed", "blockHash", blockHash)
		return false
	}

	if txsResult.GasUsed != processResult.txsResult.GasUsed {
		app.logger.Error("CheckProcessResult: mismatched gasUsed", "want", txsResult.GasUsed, "got", processResult.txsResult.GasUsed)
		return false
	}
	if txsResult.StateHash != processResult.txsResult.StateHash {
		app.logger.Error("CheckProcessResult: mismatched stateHash", "want", txsResult.StateHash, "got", processResult.txsResult.StateHash)
		return false
	}
	if txsResult.ReceiptHash != processResult.txsResult.ReceiptHash {
		app.logger.Error("CheckProcessResult: mismatched receiptHash", "want", txsResult.ReceiptHash, "got", processResult.txsResult.ReceiptHash)
		return false
	}

	return true
}
*/

func (app *LinkApplication) CommitBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit, fastsync bool) ([]*types.Validator, error) {
	if block == nil || blockParts == nil || seenCommit == nil {
		app.logger.Info("CommitBlock: nil params")
		return nil, fmt.Errorf("CommitBlock: nil params error")
	}

	blockHash := block.Hash()
	app.processLock.Lock()
	processResult := app.processMap[blockHash]
	app.processLock.Unlock()

	if processResult == nil {
		app.logger.Info("CommitBlock: processBlock goroutine is not exist", "blockHash", blockHash)
		return nil, fmt.Errorf("CommitBlock: non-exist processBlock")
	}

	if !processResult.isOk {
		return nil, fmt.Errorf("CommitBlock: processBlock failed")
	}

	app.logger.Info("CommitBlock: start", "height", block.Height, "blockHash", blockHash)

	// if !app.CheckBlockInCommit(block) {
	// 	return nil, fmt.Errorf("CommitBlock: CheckBlock failed")
	// }

	trieRoot, err := processResult.tmpState.Commit(false, block.Height)
	if err != nil {
		app.logger.Warn("CommitBlock: state commit failed", "err", err)
	}

	processResult.tmpState.Database().TrieDB().Commit(trieRoot, false)
	processResult.tmpState.Reset(trieRoot)
	processResult.txsResult.TrieRoot = trieRoot
	types.BlockBalanceRecordsInstance.SetBlockHash(block.Hash())
	types.BlockBalanceRecordsInstance.SetBlockTime(block.Time())
	// app.logger.Info("Save balance records", "block_height", block.Height, "records", string(processResult.tbrBlock.Json()))

	canList := app.updateCandidatesbyOrder(processResult, block.LastCommit.Hash())
	if block.Recover > 0 {
		canList = app.recoverCandidates(canList)
	}

	app.logger.Info("candidates list", "canList", canList)

	if canList != nil {
		processResult.txsResult.UpdateCandidates(canList)
	}
	app.balanceRecordStore.Save(block.Height, types.BlockBalanceRecordsInstance)
	app.blockChain.SaveBlock(block, blockParts, seenCommit, processResult.GetReceipts(), processResult.GetTxsResult())
	app.utxoStore.SaveUtxo(processResult.txsResult.KeyImages(), processResult.txsResult.UTXOOutputs(), block.Height)

	// candidate score metrics report
	if metrics.PrometheusMetricInstance.ProposerPubkeyEquals() {
		metricCans := processResult.tmpState.GetAllCandidates(log.Root())
		for _, metricCan := range metricCans {
			scoreMetric := metrics.PrometheusMetricInstance.GenCandidateScoreMetric(block.Height,
				metricCan.Address.String(), metricCan.Score)
			metrics.PrometheusMetricInstance.AddMetrics(scoreMetric)
		}
	}

	app.mempool.Lock()

	app.LockState()
	app.checkTxState = processResult.tmpState.Copy()
	app.mempool.KeyImageReset()
	app.lastCoe = GetCoefficient(processResult.tmpState, app.logger)
	app.logger.Info("GetCoefficient ", "Coefficient", app.lastCoe)
	types.BlacklistInstance.UpdateBlacklist()
	app.UnlockState()

	err = app.mempool.Update(app.blockChain.Height(), block.Data.Txs)
	app.mempool.Unlock()
	if err != nil {
		app.logger.Warn("CommitBlock: update mempool failed", "err", err)
	}

	app.storeState = processResult.tmpState

	block.Header.SetBloom(processResult.txsResult.LogsBloom)
	app.currentBlock = block
	app.lastTxsResult = processResult.txsResult

	if len(processResult.logs) > 0 {
		if err := app.eventbus.PublishEventLog(types.EventDataLog{Logs: processResult.logs}); err != nil {
			app.logger.Warn("CommitBlock: PublishEventLog fail", "err", err)
		} else {
			app.logger.Trace("CommitBlock: PublishEventLog", "nums", len(processResult.logs))
		}
	}

	app.clearProcessResult(block.Height)
	app.logger.Info("CommitBlock: done", "height", block.Height, "blockHash", blockHash, "trieRoot", trieRoot)

	return app.getValidators(canList), nil
}

// XXX DEPRECATED
func (app *LinkApplication) CheckBlockInCommit(block *types.Block) bool {
	blockHash := block.Hash()
	dataHash := block.Data.Hash()
	app.logger.Info("CheckBlockInCommit: start", "height", block.Height, "blockHash", blockHash, "dataHash", dataHash)

	if block.Height != app.currentBlock.Height+1 {
		app.logger.Error("CheckBlockInCommit: mismatched height", "lastBlockHeight", app.currentBlock.Height, "height", block.Height)
		return false
	}

	if block.DataHash != dataHash {
		app.logger.Error("CheckBlockInCommit: mismatched dataHash", "want", dataHash, "got", block.DataHash)
		return false
	}

	parentBlockHash := app.currentBlock.Hash()
	if block.Header.ParentHash != parentBlockHash {
		app.logger.Error("CheckBlockInCommit: mismatched parentHash", "want", parentBlockHash, "got", block.Header.ParentHash)
		return false
	}

	if block.Header.GasUsed != app.lastTxsResult.GasUsed {
		app.logger.Error("CheckBlockInCommit: mismatched gasUsed", "want", app.lastTxsResult.GasUsed, "got", block.Header.GasUsed)
		return false
	}
	if block.Header.StateHash != app.lastTxsResult.StateHash {
		app.logger.Error("CheckBlockInCommit: mismatched stateHash", "want", app.lastTxsResult.StateHash, "got", block.Header.StateHash)
		return false
	}
	if block.Header.ReceiptHash != app.lastTxsResult.ReceiptHash {
		app.logger.Error("CheckBlockInCommit: mismatched receiptHash", "want", app.lastTxsResult.ReceiptHash, "got", block.Header.ReceiptHash)
		return false
	}
	return true
}

func (app *LinkApplication) Height() uint64 {
	return app.blockChain.Height()
}

func (app *LinkApplication) LoadBlockMeta(height uint64) *types.BlockMeta {
	return app.blockChain.LoadBlockMeta(height)
}

func (app *LinkApplication) LoadBlock(height uint64) *types.Block {
	return app.blockChain.LoadBlock(height)
}

func (app *LinkApplication) LoadBlockPart(height uint64, index int) *types.Part {
	return app.blockChain.LoadBlockPart(height, index)
}

func (app *LinkApplication) LoadBlockCommit(height uint64) *types.Commit {
	return app.blockChain.LoadBlockCommit(height)
}

func (app *LinkApplication) LoadSeenCommit(height uint64) *types.Commit {
	return app.blockChain.LoadSeenCommit(height)
}

func (app *LinkApplication) GetValidators(height uint64) []*types.Validator {
	txsResult, err := app.blockChain.LoadTxsResult(height)
	if err != nil {
		app.logger.Error("failed to load validators", "err", err)
	}

	return app.getValidators(txsResult.Candidates)
}

//GetRecoverValidators return both white list and candidates
func (app *LinkApplication) GetRecoverValidators(height uint64) []*types.Validator {
	txsResult, err := app.blockChain.LoadTxsResult(height)
	if err != nil {
		app.logger.Error("failed to load validators", "err", err)
	}

	vals := app.getWhiteValidators()
	for _, v := range txsResult.Candidates {
		vals = append(vals, &types.Validator{
			Address:     v.Address,
			PubKey:      v.PubKey,
			CoinBase:    v.CoinBase,
			VotingPower: v.VotingPower,
		})
	}
	if len(vals) == 0 {
		panic("Enter Recover mode while get recover validator list is nil")
	}
	return vals
}

func (app *LinkApplication) GetStorageRoot(addr common.Address) common.Hash {
	app.LockState()
	defer app.UnlockState()
	return app.checkTxState.GetStorageRoot(addr)
}

func (app *LinkApplication) GetNonce(addr common.Address) uint64 {
	app.LockState()
	defer app.UnlockState()
	return app.checkTxState.GetNonce(addr)
}

func (app *LinkApplication) GetBalance(addr common.Address) *big.Int {
	app.LockState()
	defer app.UnlockState()
	return app.checkTxState.GetBalance(addr)
}

func (app *LinkApplication) GetPendingBlock() *types.Block {
	block := &types.Block{
		Header: app.currentBlock.Head(),
		Data: &types.Data{
			Txs: app.currentBlock.Txs,
		},
	}
	return block
}

func (app *LinkApplication) GetPendingStateDB() *state.StateDB {
	app.LockState()
	defer app.UnlockState()
	return app.checkTxState.Copy()
}

func (app *LinkApplication) GetLatestStateDB() *state.StateDB {
	return app.storeState.Copy()
}

// CheckTx assumes that txs' signature has been verified before.
func (app *LinkApplication) CheckTx(tx types.Tx, checkBasic bool) error {
	if checkBasic {
		if err := tx.CheckBasic(app); err != nil {
			app.logger.Debug("CheckTx basic: err", "err", err, "tx", tx)
			return err
		}
	} else {
		if err := tx.CheckState(app); err != nil {
			app.logger.Debug("CheckTx state: err", "err", err, "tx", tx)
			return err
		}
	}
	return nil
}

func (app *LinkApplication) updateCandidatesbyOrder(p *ProcessResult, hash common.Hash) types.CandidateInOrderList {
	if p.height%app.lastCoe.VotePeriod == 0 {
		return app.calculateCandidates(p.tmpState, hash)
	}

	canList := p.GetTxsResult().Candidates

	var i int
	//move the candidate which produce info < -3 to end
	canDel := make(types.CandidateInOrderList, 0)
	for _, v := range canList {
		if v.ProduceInfo > int(-config.Threshold) {
			canList[i] = v
			i++
			continue
		}
		if v.ProduceInfo != config.PunishThreshold {
			v.ProduceInfo = 0
			canDel = append(canDel, v)
		}
	}

	newCanList := append(canList[:i], canDel...)
	return newCanList
}

func (app *LinkApplication) processBlockEvidence(eviList types.EvidenceList, processResult *ProcessResult) {
	for _, evi := range eviList {
		switch ev := evi.(type) {
		case *types.DuplicateVoteEvidence:
			addr := ev.PubKey.Address().String()
			if v, ok := processResult.txsResult.CandidatesMap[addr]; ok {
				processResult.tmpState.UpdateCandidateScore(ev.PubKey,
					state.OPCLEAR, app.lastCoe.MaxScore, int64(processResult.height), app.logger)
				v.ProduceInfo = config.PunishThreshold
				v.Score = 0
				app.logger.Warn("Clear Score", "height", processResult.height, "ev", ev)
			}

		case *types.FaultValidatorsEvidence:
			award := ev.Proposer.Address().String()
			if v, ok := processResult.txsResult.CandidatesMap[award]; ok {
				if v.ProduceInfo < 0 {
					v.ProduceInfo = 0
				}
				v.ProduceInfo++
				if v.ProduceInfo > config.TwoConsecutive {
					processResult.tmpState.UpdateCandidateScore(ev.Proposer,
						state.OPADD, app.lastCoe.MaxScore, int64(processResult.height), app.logger)
					v.ProduceInfo = 0
					if v.Score < app.lastCoe.MaxScore {
						v.Score++
					}
					app.logger.Debug("Increase Score", "height", processResult.height, "ev", ev)
				}
			}
			if ev.Round > 0 {
				punish := ev.FaultVal.Address().String()
				if v, ok := processResult.txsResult.CandidatesMap[punish]; ok {
					if v.ProduceInfo > 0 {
						v.ProduceInfo = 0
					}
					v.ProduceInfo--
					if v.ProduceInfo <= -config.TwoConsecutive {
						processResult.tmpState.UpdateCandidateScore(ev.FaultVal,
							state.OPSUB, app.lastCoe.MaxScore, int64(processResult.height), app.logger)
						if v.Score > 1 {
							v.Score--
						}
						app.logger.Warn("Decrease Score", "height", processResult.height, "ev", ev)
					}
				}
			}
		}
	}
}

func (app *LinkApplication) getAllCandidates(s *state.StateDB, hash common.Hash) types.CandidateInOrderList {
	canState := s.GetAllCandidates(app.logger)
	app.conManager.SetCandidate(canState) //callback to tell p2p the outside candidates
	can := make(types.CandidateInOrderList, 0, len(canState))
	for _, v := range canState {
		if v.Score > 0 {
			h := crypto.Keccak256Hash(hash[:], v.Address)
			randNum := binary.BigEndian.Uint64(h[:8])
			can = append(can, &types.CandidateInOrder{
				Candidate: v.Candidate,
				Score:     v.Score,
				Rand:      int64(randNum & math.MaxInt64),
			})
		}
	}
	return can
}

// get from contract
func (app *LinkApplication) getCandidatesDeposit(s *state.StateDB, addrs []common.Address) []*big.Int {
	return s.GetCandidatesDeposit(addrs, app.logger)
}

func (app *LinkApplication) calculateCandidates(s *state.StateDB, hash common.Hash) types.CandidateInOrderList {
	can := app.getAllCandidates(s, hash)
	addrs := make([]common.Address, 0, len(can))
	for _, v := range can {
		addrs = append(addrs, v.CoinBase)
	}

	deposits := app.getCandidatesDeposit(s, addrs)

	maxDeposit, subScore := int64(1), int64(0) //Init maxDeposit in case that set candidate without pledge
	for i := range can {
		can[i].Deposit = deposits[i].Div(deposits[i], big.NewInt(config.Ether)).Int64()
		if can[i].Deposit > maxDeposit {
			maxDeposit = can[i].Deposit
		}
		subScore += can[i].Score
	}
	for _, v := range can {
		v.CalRank(app.lastCoe.Srate, app.lastCoe.Drate, app.lastCoe.Rrate, maxDeposit, subScore)
	}

	salt := binary.BigEndian.Uint64(hash[:8])
	can.RandomSort(int64(salt))

	for i, v := range can {
		f, _ := v.RankResult.Float64()
		app.logger.Info("candidates rank", "rank", f, "address", v.CoinBase.String())
		v.Rank = i
	}
	return can
}

//get from contract
func (app *LinkApplication) getWhiteValidators() []*types.Validator {
	return app.storeState.GetWhiteValidators(app.logger)
}

func (app *LinkApplication) recoverCandidates(cans types.CandidateInOrderList) types.CandidateInOrderList {
	lastVals := make(map[crypto.PubKey]struct{})
	for _, val := range app.lastVals {
		lastVals[val.PubKey] = struct{}{}
	}
	var i = 0
	lastCans := make(types.CandidateInOrderList, 0)
	for _, v := range cans {
		if _, ok := lastVals[v.PubKey]; !ok {
			cans[i] = v
			i++
		} else {
			lastCans = append(lastCans, v)
		}
	}
	newCanList := append(cans[:i], lastCans...)
	return newCanList
}

func (app *LinkApplication) getValidators(cans types.CandidateInOrderList) []*types.Validator {

	numValidators := len(cans) * app.lastCoe.Nume / app.lastCoe.Deno

	if numValidators > app.lastCoe.UpperLimit {
		numValidators = app.lastCoe.UpperLimit
	}
	chosenCans := cans[:numValidators]

	vals := app.getWhiteValidators()
	for _, v := range chosenCans {
		vals = append(vals, &types.Validator{
			Address:     v.Address,
			PubKey:      v.PubKey,
			CoinBase:    v.CoinBase,
			VotingPower: v.VotingPower,
		})
	}
	return vals
}

func GetCoefficient(statedb types.StateDB, logger log.Logger) *types.Coefficient {
	if co := statedb.GetCoefficient(logger); co != nil {
		return co
	}
	return types.DefaultCoefficient()
}

func SetPoceeds(wasm *wasm.WASM, coinbase common.Address, amount *big.Int, logger log.Logger) error {
	input := `setPoceeds|{"0":"` + coinbase.String() + `","1":"` + amount.String() + `"}`
	logger.Info("setPoceeds", "input", input)
	_, err := CallWasmContract(wasm, common.EmptyAddress, config.ContractFoundationAddr, big.NewInt(0), []byte(input), logger)
	return err
}

func AllocAward(wasm *wasm.WASM, logger log.Logger) error {
	input := "allocAward|{}"
	logger.Info("allocAward")
	_, err := CallWasmContract(wasm, common.EmptyAddress, config.ContractFoundationAddr, big.NewInt(0), []byte(input), logger)

	if err == nil {
		payloads := make([]types.Payload, 0)
		tbr := types.NewTxBalanceRecords()
		tbr.SetOptions(common.EmptyHash, types.TxNormal, payloads, 0, uint64(math.MaxUint64),
			big.NewInt(types.GasPrice), common.EmptyAddress, config.ContractFoundationAddr, common.EmptyAddress)
		otxs := wasm.GetOTxs()
		for _, otx := range otxs {
			tbr.AddBalanceRecord(otx)
		}
		if tbr.IsBalanceRecordEmpty() {
			return nil
		}
		br := types.GenBalanceRecord(common.EmptyAddress, config.ContractFoundationAddr, types.NoAddress, types.AccountAddress, types.TxFee, common.EmptyAddress, big.NewInt(0))
		tbr.AddBalanceRecord(br)
		types.BlockBalanceRecordsInstance.AddTxBalanceRecord(tbr)
	}

	return err
}

//CallWasmContract only be used by chain inner to call wasm contract directly
func CallWasmContract(wasm *wasm.WASM, sender, contractAddr common.Address, amount *big.Int, input []byte, logger log.Logger) ([]byte, error) {
	gas := uint64(math.MaxUint64) //Max gas
	st := wasm.StateDB

	innerContract := vm.NewContract(sender.Bytes(), contractAddr.Bytes(), amount, gas)
	innerContract.SetCallCode(contractAddr.Bytes(), st.GetCodeHash(contractAddr).Bytes(), st.GetCode(contractAddr))
	innerContract.Input = input
	eng := vm.NewEngine(innerContract, innerContract.Gas, st, logger)
	eng.Ctx = wasm
	eng.SetTrace(false) // trace app execution.

	app, err := eng.NewApp(innerContract.Address().String(), innerContract.Code, false)
	if err != nil {
		return nil, fmt.Errorf("exec.NewApp fail: %s", err)
	}

	snapshot := st.Snapshot()
	app.EntryFunc = vm.APPEntry
	ret, err := eng.Run(app, innerContract.Input)
	if err != nil {
		st.RevertToSnapshot(snapshot)
		return nil, fmt.Errorf("eng.Run fail: err=%s", err)
	}
	vmem := app.VM.VMemory()
	result, err := vmem.GetString(ret)
	if err != nil {
		st.RevertToSnapshot(snapshot)
		return nil, fmt.Errorf("vmem.GetString fail: err=%v", err)
	}
	return result, nil
}
