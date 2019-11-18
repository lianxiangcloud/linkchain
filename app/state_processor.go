// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package app

import (
	"math/big"

	"github.com/lianxiangcloud/linkchain/blockchain"
	"github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	lctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/vm"
	"github.com/lianxiangcloud/linkchain/vm/evm"
	"github.com/lianxiangcloud/linkchain/vm/wasm"
)

const (
	//function signature: 0x08c379a0
	jsondata = `[{ "type" : "function", "name" : "Error", "constant" : true,  "inputs":[{ "name" : "message", "type" : "string" } ], "outputs":[{ "name" : "", "type" : "string" } ] }]`
	cerrName = "Error"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	bc  *blockchain.BlockStore // Canonical block chain
	app *LinkApplication
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(bc *blockchain.BlockStore, app *LinkApplication) *StateProcessor {
	return &StateProcessor{
		bc:  bc,
		app: app,
	}
}

type processState struct {
	//outputs
	Receipts    types.Receipts
	AllLogs     []*types.Log
	UsedGas     uint64
	SpecialTxs  []types.Tx
	UtxoOutputs []*types.UTXOOutputData
	KeyImages   []*lctypes.Key
	//states
	Block        *types.Block
	Statedb      *state.StateDB
	Vmenv        vm.VmFactory
	KeyImagesMap map[lctypes.Key]bool
}

func initProcessState(block *types.Block, statedb *state.StateDB, cfg evm.Config, bc *blockchain.BlockStore) (s *processState) {
	length := len(block.Data.Txs)
	s = &processState{
		Receipts:    make(types.Receipts, 0),
		AllLogs:     make([]*types.Log, 0, length),
		UsedGas:     0,
		SpecialTxs:  []types.Tx{},
		UtxoOutputs: make([]*types.UTXOOutputData, 0),
		KeyImages:   make([]*lctypes.Key, 0),
		// states
		Block:        block,
		Statedb:      statedb,
		Vmenv:        vm.NewVM(),
		KeyImagesMap: make(map[lctypes.Key]bool),
	}
	header := types.CopyHeader(block.Header)
	evmGasRate := config.EvmGasRate
	contextEvm := evm.NewEVMContext(header, bc, nil, evmGasRate)
	s.Vmenv.AddVm(&contextEvm, statedb, cfg)
	wasmGasRate := config.WasmGasRate
	contextWasm := wasm.NewWASMContext(header, bc, nil, wasmGasRate)
	s.Vmenv.AddVm(&contextWasm, statedb, cfg)

	return
}

func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg evm.Config) (types.Receipts, []*types.Log, uint64, []types.Tx, []*types.UTXOOutputData, []*lctypes.Key, error) {

	// init
	s := initProcessState(block, statedb, cfg, p.bc)
	// Iterate over and process the individual transactions
	for idx, txRaw := range block.Data.Txs {
		if err := s.checkValid(txRaw, p.app); err != nil {
			log.Error("Process checkValid Error", "hash", txRaw.Hash(), "err", err)
			return nil, nil, 0, nil, nil, nil, err
		}
		if err := s.resetEnv(txRaw, idx); err != nil {
			log.Error("Process resetEnv Error", "hash", txRaw.Hash(), "err", err)
			return nil, nil, 0, nil, nil, nil, err
		}
		if err := s.txRawProcess(txRaw); err != nil {
			log.Error("Process txRawProcess Error", "hash", txRaw.Hash(), "err", err)
			return nil, nil, 0, nil, nil, nil, err
		}
		//TODO: replace AsMessage in /types
		tx, err := GenerateTransaction(txRaw, statedb, &s.Vmenv)
		if err != nil {
			log.Error("Process GenerateTransaction Error", "hash", txRaw.Hash(), "err", err)
			return nil, nil, 0, nil, nil, nil, err
		}
		transRes, vmerr, err := tx.Transit()
		if err != nil {
			log.Error("Process Transit Error", "hash", tx.Hash, "err", err)
			return nil, nil, 0, nil, nil, nil, err
		}
		s.postProcess(tx, transRes, vmerr)
	}
	log.Debug("Process", "hash", block.Hash, "receipts", s.Receipts, "allLogs", s.AllLogs, "usedGas", s.UsedGas, "specialTxs", s.SpecialTxs, "utxoOutputs", s.UtxoOutputs, "keyImages", s.KeyImages)
	return s.Receipts, s.AllLogs, s.UsedGas, s.SpecialTxs, s.UtxoOutputs, s.KeyImages, nil
}

func (s *processState) checkValid(txi types.Tx, app *LinkApplication) (err error) {
	switch tx := txi.(type) {
	case *types.Transaction:
		err = tx.CheckBasicWithState(nil, s.Statedb)

	case *types.TokenTransaction:
		err = tx.CheckBasicWithState(nil, s.Statedb)

	case *types.UTXOTransaction:
		if err = tx.CheckStoreState(app, s.Statedb); err != nil {
			return
		}
		kms := tx.GetInputKeyImages()
		for _, km := range kms {
			if s.KeyImagesMap[*km] {
				return types.ErrUtxoTxDoubleSpend
			}
			s.KeyImagesMap[*km] = true
		}
	}
	return
}

func (s *processState) resetEnv(txi types.Tx, idx int) (err error) {
	s.Statedb.Prepare(txi.Hash(), s.Block.Hash(), idx)
	return nil
}

func GenerateTransaction(txi types.Tx, state *state.StateDB, vmenv *vm.VmFactory) (txo *processTransaction, err error) {
	txo = &processTransaction{}
	// generic
	txo.Type = txi.TypeName()
	txo.State = state
	txo.Vmenv = vmenv
	txo.Hash = txi.Hash()

	txo.Inputs = make([]txInput, 0)
	txo.Outputs = make([]txOutput, 0)
	switch tx := txi.(type) {
	case *types.ContractUpgradeTx:
		from, err := tx.From()
		if err != nil {
			return nil, err
		}
		in := txInput{
			From:  from,
			Value: tx.Value(),
			Nonce: tx.Nonce(),
			Type:  Ain,
		}
		txo.Inputs = append(txo.Inputs, in)
		out := txOutput{
			To:     *tx.To(),
			Amount: tx.Value(),
			Data:   tx.Data(),
			Type:   Updateout,
		}
		txo.Outputs = append(txo.Outputs, out)
		txo.Kind = types.AinAout
		txo.TokenAddress = tx.TokenAddress()
		// Gas (Not bought yet!)
		txo.Gas = tx.Gas()
		txo.GasPrice = tx.GasPrice()
		txo.InitialGas = tx.Gas()
		txo.RefundAddr = from
	case *types.TokenTransaction:
		from, err := tx.From()
		if err != nil {
			return nil, err
		}
		in := txInput{
			From:  from,
			Value: tx.Value(),
			Nonce: tx.Nonce(),
			Type:  Ain,
		}
		txo.Inputs = append(txo.Inputs, in)
		out := txOutput{
			To:     *tx.To(),
			Amount: tx.Value(),
			Data:   tx.Data(),
			Type:   Aout,
		}
		if state.IsContract(*tx.To()) {
			out.Type = Cout
		}
		txo.Outputs = append(txo.Outputs, out)
		txo.Kind = types.AinAout
		txo.TokenAddress = tx.TokenAddress()
		// Gas (Not bought yet!)
		txo.Gas = tx.Gas()
		txo.GasPrice = tx.GasPrice()
		txo.InitialGas = tx.Gas()
		txo.RefundAddr = from
	case *types.Transaction:
		from, err := tx.From()
		if err != nil {
			return nil, err
		}
		in := txInput{
			From:  from,
			Value: tx.Value(),
			Nonce: tx.Nonce(),
			Type:  Ain,
		}
		txo.Inputs = append(txo.Inputs, in)

		toAddr := common.EmptyAddress
		if tx.To() != nil {
			toAddr = *tx.To()
		}
		out := txOutput{
			To:     toAddr,
			Amount: tx.Value(),
			Data:   tx.Data(),
			Type:   Aout,
		}
		if tx.To() == nil {
			out.Type = Createout
		} else if state.IsContract(*tx.To()) {
			out.Type = Cout
		}
		txo.Outputs = append(txo.Outputs, out)
		txo.Kind = types.AinAout
		txo.TokenAddress = tx.TokenAddress()
		// Gas (Not bought yet!)
		txo.Gas = tx.Gas()
		txo.GasPrice = tx.GasPrice()
		txo.InitialGas = tx.Gas()
		txo.RefundAddr = from
	case *types.MultiSignAccountTx:
		from, err := tx.From()
		if err != nil {
			return nil, err
		}
		in := txInput{
			From:  from,
			Value: big.NewInt(0),
			Nonce: tx.Nonce(),
			Type:  Ain,
		}
		txo.Inputs = append(txo.Inputs, in)
		txo.Kind = types.Ain
		txo.Gas = 0
		txo.GasPrice = big.NewInt(0)
		txo.InitialGas = 0
		txo.RefundAddr = common.EmptyAddress
	case *types.UTXOTransaction:
		from := common.EmptyAddress
		//utxo tx now depend on FROM not Ain
		if (tx.UTXOKind()&types.Ain) == types.Ain || !common.IsLKC(tx.TokenAddress()) { //TODO: fix this to accept multiple inputs
			from, err = tx.From()
			if err != nil {
				return nil, err
			}
			realValue := big.NewInt(0)
			if (tx.UTXOKind() & types.Ain) == types.Ain {
				for _, input := range tx.Inputs {
					switch ip := input.(type) {
					case *types.AccountInput:
						realValue.Set(ip.Amount)
					default:
					}
				}
				in := txInput{
					From:  from,
					Value: realValue,
					Nonce: state.GetNonce(from), //tx.Nonce is volatile
					Type:  Ain,
				}
				txo.Inputs = append(txo.Inputs, in)
			}
		}

		if (tx.UTXOKind() & types.Aout) == types.Aout {
			msg, err := tx.AsMessage()
			if err != nil {
				return nil, err
			}
			for _, accOutputData := range msg.OutputData() {
				out := txOutput{
					To:     accOutputData.To,
					Amount: accOutputData.Amount,
					Data:   accOutputData.Data,
					Type:   Aout,
				}
				if state.IsContract(accOutputData.To) {
					out.Type = Cout
				}
				txo.Outputs = append(txo.Outputs, out)
			}
		}
		txo.Kind = tx.UTXOKind()
		txo.TokenAddress = tx.TokenAddress()
		// Gas (Already bought!)
		txo.Gas = tx.Gas()
		txo.GasPrice = tx.GasPrice()
		txo.InitialGas = tx.Gas()
		txo.RefundAddr = from
	default:
		err = types.ErrGenerateProcessTransaction
		return nil, err
	}
	log.Debug("GenerateTransaction", "hash", txi.Hash(), "txo", txo)
	return
}

func (s *processState) receiptProcess(tx *processTransaction, transRes *TransitionResult, vmerr error) {
	if tx.Type == types.TxMultiSignAccount { // bypass this step
		s.Receipts = append(s.Receipts, &types.Receipt{})
		return
	}
	s.UsedGas += transRes.Gas
	receipt := types.NewReceipt(nil, vmerr, s.UsedGas)
	receipt.TxHash = tx.Hash
	receipt.GasUsed = transRes.Gas
	if len(transRes.Addrs) > 0 {
		receipt.ContractAddress = transRes.Addrs[0]
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = tx.State.GetLogs(tx.Hash)
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	s.Receipts = append(s.Receipts, receipt)
	if receipt.Logs != nil {
		s.AllLogs = append(s.AllLogs, receipt.Logs...)
	}
	log.Debug("receiptProcess", "hash", tx.Hash, "receipt", receipt, "vmerr", vmerr)
}

func balanceRecordProcess(tx *processTransaction, transRes *TransitionResult, vmerr error) {
	var (
		tbr      = types.NewTxBalanceRecords()
		hash     = tx.Hash
		typeName = tx.Type
		payloads = make([]types.Payload, 0)
		nonce    = uint64(0)
		gasPrice = tx.GasPrice
		gasLimit = tx.InitialGas
		from     = common.EmptyAddress
		to       = common.EmptyAddress
		tokenID  = tx.TokenAddress
	)
	if tx.Type == types.TxMultiSignAccount { // bypass this step
		return
	}

	if len(tx.Inputs) > 0 {
		from = tx.Inputs[0].From
		nonce = tx.Inputs[0].Nonce
	}
	if len(tx.Outputs) > 0 {
		to = tx.Outputs[0].To
		for _, output := range tx.Outputs {
			if output.Data != nil {
				payloads = append(payloads, output.Data)
			}
		}
	}
	for _, br := range transRes.Otxs {
		tbr.AddBalanceRecord(br)
	}
	tbr.SetOptions(hash, typeName, payloads, nonce, gasLimit, gasPrice, from, to, tokenID)
	log.Debug("balanceRecordProcess", "hash", tx.Hash, "txtype", tx.Type, "tbr", tbr)
	types.BlockBalanceRecordsInstance.AddTxBalanceRecord(tbr)
}

func (s *processState) postProcess(tx *processTransaction, transRes *TransitionResult, vmerr error) {
	s.receiptProcess(tx, transRes, vmerr)
	// 交易记录开关
	balanceRecordProcess(tx, transRes, vmerr)
}

func (s *processState) txRawProcess(txi types.Tx) (err error) {
	switch tx := txi.(type) {
	case *types.MultiSignAccountTx:
		s.SpecialTxs = append(s.SpecialTxs, txi)
	case *types.UTXOTransaction:
		s.UtxoOutputs = append(s.UtxoOutputs, tx.GetOutputData(s.Block.Height)...)
		s.KeyImages = append(s.KeyImages, tx.GetInputKeyImages()...)
	default:
	}
	return nil
}

// Deprecated
func ApplyMessage(vmenv vm.VmInterface, msg types.Message, tokenAddr common.Address) (ret []byte, gas uint64, byteCodeGas uint64, fee *big.Int, refundFee uint64, vmerr error, err error) {
	prot := processTransaction{}
	prot.TokenAddress = msg.TokenAddress()
	prot.Gas = msg.Gas()
	prot.GasPrice = msg.GasPrice()
	prot.InitialGas = msg.Gas()
	prot.RefundAddr = msg.MsgFrom()
	prot.State = vmenv.GetStateDB().(*state.StateDB)
	prot.Vmenv = vm.NewVMwithInstance(vmenv)

	prot.Hash = common.EmptyHash
	prot.Type = msg.TxType()
	prot.Inputs = make([]txInput, 0)
	prot.Outputs = make([]txOutput, 0)
	if prot.Type == types.TxUTXO {
		prot.Kind = msg.UTXOKind()
		for _, accOutputData := range msg.OutputData() {
			out := txOutput{
				To:     accOutputData.To,
				Amount: accOutputData.Amount,
				Data:   accOutputData.Data,
				Type:   Aout,
			}
			if prot.State.IsContract(accOutputData.To) {
				out.Type = Cout
			}
			prot.Outputs = append(prot.Outputs, out)
		}
	} else {
		in := txInput{
			From:  msg.MsgFrom(),
			Value: msg.Value(),
			Nonce: msg.Nonce(),
			Type:  Ain,
		}
		prot.Inputs = append(prot.Inputs, in)

		outAddr := common.EmptyAddress
		if msg.To() != nil {
			outAddr = *msg.To()
		}
		out := txOutput{
			To:     outAddr,
			Amount: msg.Value(),
			Data:   msg.Data(),
			Type:   Aout,
		}
		if msg.To() == nil {
			out.Type = Createout
		} else if prot.State.IsContract(*msg.To()) {
			out.Type = Cout
		}
		prot.Kind = types.AinAout
		prot.Outputs = append(prot.Outputs, out)
	}

	res, vmerr, err := prot.Transit()
	if len(res.Rets) > 0 {
		ret = res.Rets[0]
	}
	return ret, res.Gas, res.ByteCodeGas, res.Fee, res.RefundFee, vmerr, err
}
