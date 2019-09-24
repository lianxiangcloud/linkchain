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
	"bytes"
	"fmt"
	"strings"

	"math/big"

	"github.com/lianxiangcloud/linkchain/accounts/abi"
	"github.com/lianxiangcloud/linkchain/blockchain"
	"github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	lctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
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

var (
	cabi, _ = abi.JSON(strings.NewReader(jsondata))
	cerrID  = cabi.Methods[cerrName].Id()
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

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg evm.Config) (types.Receipts, []*types.Log, uint64, []types.Tx, []*types.UTXOOutputData, []*lctypes.Key, error) {
	var (
		length       = len(block.Data.Txs)
		receipts     = make(types.Receipts, 0)
		usedGas      = new(uint64)
		header       = types.CopyHeader(block.Header)
		allLogs      = make([]*types.Log, 0, length)
		specialTxs   = []types.Tx{}
		utxoOutputs  = make([]*types.UTXOOutputData, 0)
		keyImages    = make([]*lctypes.Key, 0)
		keyImagesMap = make(map[lctypes.Key]bool)
	)

	vmenv := vm.NewVM()
	evmGasRate := config.EvmGasRate
	contextEvm := evm.NewEVMContext(header, p.bc, nil, evmGasRate)
	vmenv.AddVm(&contextEvm, statedb, cfg)
	wasmGasRate := config.WasmGasRate
	contextWasm := wasm.NewWASMContext(header, p.bc, nil, wasmGasRate)
	vmenv.AddVm(&contextWasm, statedb, cfg)

	// Iterate over and process the individual transactions
	for idx, txRaw := range block.Data.Txs {
		switch tx := txRaw.(type) {
		case *types.Transaction, *types.TokenTransaction, *types.ContractCreateTx, *types.ContractUpgradeTx:
			tbr := types.NewTxBalanceRecords()
			//case types.RegularTx:
			statedb.Prepare(txRaw.Hash(), block.Hash(), idx)
			receipt, otxs, err := p.applyTransaction(statedb, tx.(types.RegularTx), usedGas, &vmenv)
			for _, br := range otxs {
				tbr.AddBalanceRecord(br)
			}

			if err != nil {
				log.Error("applytransaction", "height", block.Height, "idx", idx, "tx", txRaw.Hash().String(), "receipt", receipt.Hash().String(), "err", err)
				return nil, nil, 0, nil, nil, nil, err
			}
			payloads := make([]types.Payload, 0)
			if tx.(types.RegularTx).Data() != nil {
				payloads = append(payloads, tx.(types.RegularTx).Data())
			}
			nonce := tx.(types.RegularTx).Nonce()
			gasLimit := tx.(types.RegularTx).Gas()
			gasPrice := tx.(types.RegularTx).GasPrice()
			tokenId := tx.(types.RegularTx).TokenAddress()
			from, err := tx.From()
			if err != nil {
				from = common.EmptyAddress
			}
			var to common.Address
			toPtr := tx.To()
			if toPtr == nil {
				to = common.EmptyAddress
			} else {
				to = *toPtr
			}
			tbr.SetOptions(tx.Hash(), tx.TypeName(), payloads, nonce, gasLimit, gasPrice, from, to, tokenId)
			types.BlockBalanceRecordsInstance.AddTxBalanceRecord(tbr)
			receipts = append(receipts, receipt)
			allLogs = append(allLogs, receipt.Logs...)
		case *types.MultiSignAccountTx:
			log.Debug("Process", "MultiSignAccountTx", tx)
			specialTxs = append(specialTxs, tx)
			from, _ := tx.From()
			statedb.SetNonce(from, statedb.GetNonce(from)+1)
			receipts = append(receipts, &types.Receipt{})
		case *types.UTXOTransaction:
			if err := tx.CheckStoreState(p.app, statedb); err != nil {
				return nil, nil, 0, nil, nil, nil, err
			}

			kms := tx.GetInputKeyImages()
			for _, km := range kms {
				if keyImagesMap[*km] {
					return nil, nil, 0, nil, nil, nil, types.ErrUtxoTxDoubleSpend
				}
				keyImagesMap[*km] = true
			}

			//any account mode enter state process
			tbr := types.NewTxBalanceRecords()
			tbr.Type = tx.TypeName()
			tbr.Hash = tx.Hash()

			var receipt *types.Receipt
			var err error
			var otxs []types.BalanceRecord
			log.Debug("Process", "UTXOTransaction", tx, "UTXOKind", tx.UTXOKind())
			if (tx.UTXOKind()&types.Ain) == types.Ain || (tx.UTXOKind()&types.Aout) == types.Aout {
				statedb.Prepare(txRaw.Hash(), block.Hash(), idx)
				receipt, otxs, err = p.applyUTXOTransaction(statedb, tx, usedGas, &vmenv)
				if err != nil {
					log.Error("applytransaction", "height", block.Height, "idx", idx, "tx", txRaw.Hash().String(), "receipt", receipt.Hash().String(), "err", err)
					return nil, nil, 0, nil, nil, nil, err
				}
				for _, br := range otxs {
					tbr.AddBalanceRecord(br)
				}
				allLogs = append(allLogs, receipt.Logs...)
				// balance records
				payloads := make([]types.Payload, 0)
				var from, to common.Address
				if (tx.UTXOKind() & types.Aout) == types.Aout {
					for _, out := range tx.Outputs {
						switch aOutput := out.(type) {
						case *types.AccountOutput:
							if aOutput.Data != nil {
								if aOutput.Data != nil {
									payloads = append(payloads, aOutput.Data)
								}
							}
							to = aOutput.To
						}
					}
				}
				var nonce uint64
				if (tx.UTXOKind() & types.Ain) == types.Ain {
					for _, in := range tx.Inputs {
						switch aInput := in.(type) {
						case *types.AccountInput:
							nonce = aInput.Nonce
						}
					}
					from, err = tx.From()
					if err != nil {
						from = common.EmptyAddress
					}
				}
				gasPrice := tx.GasPrice()
				gasLimit := tx.Gas()
				tbr.SetOptions(tx.Hash(), tx.TypeName(), payloads, nonce, gasLimit, gasPrice, from, to, tx.TokenID)
			} else {
				from, err := tx.From()
				log.Debug("Process UTXO->UTXO", "from", from.String(), "err", err)
				gas := tx.Gas()
				*usedGas += gas
				receipt = &types.Receipt{
					CumulativeGasUsed: *usedGas,
					TxHash:            tx.Hash(),
					GasUsed:           gas,
					Status:            types.ReceiptStatusSuccessful,
				}
				fee := new(big.Int).Mul(big.NewInt(0).SetUint64(gas), big.NewInt(types.GasPrice))
				br := types.GenBalanceRecord(common.EmptyAddress, config.ContractFoundationAddr, types.PrivateAddress,
					types.AccountAddress, types.TxFee, common.EmptyAddress, fee)
				tbr.AddBalanceRecord(br)
				payloads := make([]types.Payload, 0)
				gasPrice := tx.GasPrice()
				gasLimit := tx.Gas()
				tbr.SetOptions(tx.Hash(), tx.TypeName(), payloads, 0, gasLimit, gasPrice, common.EmptyAddress,
					common.EmptyAddress, common.EmptyAddress)
			}
			types.BlockBalanceRecordsInstance.AddTxBalanceRecord(tbr)
			receipts = append(receipts, receipt)
			utxoOutputs = append(utxoOutputs, tx.GetOutputData(block.Height)...)
			keyImages = append(keyImages, kms...)
		default:
			err := fmt.Errorf("unknow tx type")
			return nil, nil, 0, nil, nil, nil, err
		}
	}

	return receipts, allLogs, *usedGas, specialTxs, utxoOutputs, keyImages, nil
}

func (p *StateProcessor) applyUTXOTransaction(statedb *state.StateDB, tx *types.UTXOTransaction, usedGas *uint64, vmenv *vm.VmFactory) (*types.Receipt, []types.BalanceRecord, error) {
	msg, err := tx.AsMessage()
	if err != nil {
		return nil, nil, err
	}

	var msgcode []byte
	if len(msg.OutputData()) > 0 {
		msgcode = msg.OutputData()[0].Data
		toAddr := msg.OutputData()[0].To
		if statedb.IsContract(toAddr) {
			msgcode = statedb.GetCode(toAddr)
		}
	}
	realvm := vmenv.GetRealVm(msgcode, nil)
	realvm.Reset(msg)
	realvm.SetToken(msg.TokenAddress())

	log.Debug("applyUTXOTransaction")
	// Apply the transaction to the current state (included in the env)
	ret, gas, _, fee, vmerr, err := ApplyUTXOMessage(realvm, tx, msg.TokenAddress())
	if vmerr != nil || err != nil {
		if vmerr != nil && bytes.HasPrefix(ret, cerrID) {
			var reason string
			if err := cabi.Unpack(&reason, cerrName, ret[len(cerrID):]); err == nil {
				vmerr = fmt.Errorf("%v: %s", vmerr, reason)
			}
		}
		log.Report("ApplyUTXOMessage", "logID", types.LogIdContractExecutionError, "hash", tx.Hash().Hex(), "vmerr", vmerr, "err", err)
	}
	if err != nil {
		return nil, nil, err
	}

	otxs := realvm.GetOTxs()
	if vmerr != nil {
		otxs = make([]types.BalanceRecord, 0)
	}
	if fee != nil {
		from := common.EmptyAddress
		if (tx.UTXOKind() & types.Ain) == types.Ain {
			from, err = tx.From()
			if err != nil {
				from = common.EmptyAddress
			}
			br := types.GenBalanceRecord(from, config.ContractFoundationAddr, types.AccountAddress, types.AccountAddress, types.TxFee, msg.TokenAddress(), fee)
			otxs = append(otxs, br)
		} else {
			br := types.GenBalanceRecord(from, config.ContractFoundationAddr, types.PrivateAddress, types.AccountAddress, types.TxFee, msg.TokenAddress(), fee)
			otxs = append(otxs, br)
		}
	}

	// Update the state with pending changes
	//root := statedb.IntermediateRoot(false).Bytes()
	*usedGas += gas
	receipt := types.NewReceipt(nil, vmerr, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, otxs, err
}

// applyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func (p *StateProcessor) applyTransaction(statedb *state.StateDB, tx types.RegularTx, usedGas *uint64, vmenv *vm.VmFactory) (*types.Receipt, []types.BalanceRecord, error) {
	msg, err := tx.AsMessage()
	if err != nil {
		return nil, nil, err
	}

	from := msg.MsgFrom()
	toPtr := msg.To()
	isContractCreationTx := (msg.TxType() == types.TxContractCreate || msg.TxType() == types.TxContractUpgrade)
	log.Debug("applyTransaction", "msg.TxType", msg.TxType())

	msgcode := msg.Data()
	if msg.To() != nil && statedb.IsContract(*msg.To()) {
		msgcode = statedb.GetCode(*(msg.To()))
	}
	realvm := vmenv.GetRealVm(msgcode, toPtr)

	realvm.Reset(msg)
	realvm.SetToken(msg.TokenAddress())

	// Apply the transaction to the current state (included in the env)
	ret, gas, _, fee, vmerr, err := ApplyMessage(realvm, tx, msg.TokenAddress())
	if vmerr != nil || err != nil {
		if vmerr != nil && bytes.HasPrefix(ret, cerrID) {
			var reason string
			if err := cabi.Unpack(&reason, cerrName, ret[len(cerrID):]); err == nil {
				vmerr = fmt.Errorf("%v: %s", vmerr, reason)
			}
		}
		log.Report("ApplyMessage", "logID", types.LogIdContractExecutionError, "hash", tx.Hash().Hex(), "from", from.Hex(), "nonce", tx.Nonce(), "vmerr", vmerr, "err", err)
	}
	if err != nil {
		return nil, nil, err
	}

	otxs := realvm.GetOTxs()
	if vmerr != nil {
		otxs = make([]types.BalanceRecord, 0)
	}

	if fee != nil {
		from, err := tx.From()
		if err != nil {
			from = common.EmptyAddress
		}
		br := types.GenBalanceRecord(from, config.ContractFoundationAddr, types.AccountAddress, types.AccountAddress, types.TxFee, msg.TokenAddress(), fee)
		otxs = append(otxs, br)
	}

	// Update the state with pending changes
	//root := statedb.IntermediateRoot(false).Bytes()
	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing wether the root touch-delete accounts.
	receipt := types.NewReceipt(nil, vmerr, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if (isContractCreationTx && msg.TxType() != types.TxContractUpgrade) || (msg.TxType() == types.TxNormal && toPtr == nil) {
		if vmerr == nil {
			receipt.ContractAddress = crypto.CreateAddress(from, msg.Nonce(), msg.Data())
			log.Info("ContractCreate succ", "ContractAddress", receipt.ContractAddress)
		}
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, otxs, err
}

//HashForIndex cal the key for contractTx
func HashForIndex(x interface{}) (h common.Hash) {
	b := ser.MustEncodeToBytes(x)
	return crypto.Keccak256Hash(b)
}
