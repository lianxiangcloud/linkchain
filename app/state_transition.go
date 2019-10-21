// Copyright 2014 The go-ethereum Authors
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
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/lianxiangcloud/linkchain/accounts/abi"
	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/vm/evm"
	"github.com/lianxiangcloud/linkchain/vm/wasm"
)

var (
	errInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
	errIntrinsicGasOverflow      = errors.New("intrinsic gas overflow")
)
var (
	cabi, _ = abi.JSON(strings.NewReader(jsondata))
	cerrID  = cabi.Methods[cerrName].Id()
)

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, contractCreation bool, gasRate uint64) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if contractCreation {
		gas = cfg.TxGasContractCreation
	} else {
		gas = cfg.TxGas
	}
	// Bump the required gas by the amount of transactional data
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		// Make sure we don't exceed uint64 for all data combinations
		if (math.MaxUint64-gas)/cfg.TxDataNonZeroGas < nz {
			log.Warn("IntrinsicGas", "gas", gas, "nz", nz)
			return 0, errIntrinsicGasOverflow
		}
		gas += nz * cfg.TxDataNonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/cfg.TxDataZeroGas < z {
			log.Warn("IntrinsicGas", "gas", gas, "z", z)
			return 0, errIntrinsicGasOverflow
		}
		gas += z * cfg.TxDataZeroGas
	}
	if gasRate > 0 {
		// cal wasm discount gas
		gas = gas / gasRate
		if gas < 1 {
			// min gas 1
			gas = 1
		}
	}
	return gas, nil
}

func (tx *processTransaction) useGas(amount uint64) error {
	if tx.Gas < amount {
		return evm.ErrOutOfGas
	}
	tx.Gas -= amount
	return nil
}

func (tx *processTransaction) Transit() (res *TransitionResult, vmerr, err error) {
	res = &TransitionResult{}
	if err = tx.preTransit(); err != nil {
		return
	}

	var (
		transferGas = uint64(0)
		snapshot    = tx.State.Snapshot()
	)

	// Only vmerr Could be thrown afterwards
	if vmerr = tx.transitInputs(); vmerr != nil {
		goto Done
	}
	if transferGas, vmerr = tx.payTransferGas(); vmerr != nil {
		goto Done
	}
	if vmerr = tx.transitOutputs(res); vmerr != nil {
		goto Done
	}

Done:
	// No err/vmerr Shall be thrown afterwards
	tx.postTransit(res, transferGas, snapshot, vmerr)
	return
}

func (tx *processTransaction) postTransit(res *TransitionResult, transferGas uint64, snapshot int, vmerr error) {
	tx.refundGas(transferGas, snapshot, vmerr)
	tx.setNonce()
	tx.genTransitTxRecord(res, vmerr)
}

func (tx *processTransaction) setNonce() {
	// set nonce should be processed AFTER contract create
	for _, in := range tx.Inputs {
		tx.State.SetNonce(in.From, in.Nonce+1) // nonce is checked before
		log.Debug("transitInputs: setNonce", "hash", tx.Hash, "oldNonce", in.Nonce, "newNonce", tx.State.GetNonce(in.From))
	}
}

func (tx *processTransaction) preTransit() (err error) {
	if err = tx.checkNonce(); err != nil {
		return
	}
	if err = tx.buyGas(); err != nil {
		return
	}
	if err = tx.payIntrinsicGas(); err != nil {
		return
	}
	return
}

//TODO: processed in checkValid
func (tx *processTransaction) checkNonce() (err error) {
	for _, ain := range tx.Inputs {
		nonce := tx.State.GetNonce(ain.From)
		if nonce < ain.Nonce {
			log.Warn("preCheck: nonce too high", "hash", tx.Hash, "from", ain.From, "stateNonce", nonce, "TxNonce", ain.Nonce)
			return types.ErrNonceTooHigh
		} else if nonce > ain.Nonce {
			log.Warn("preCheck: nonce too low", "hash", tx.Hash, "from", ain.From, "stateNonce", nonce, "TxNonce", ain.Nonce)
			return types.ErrNonceTooLow
		}
	}
	return
}

func (tx *processTransaction) buyGas() (err error) { // default use Input[0] to buy gas
	if tx.Type == types.TxContractCreate || tx.Type == types.TxContractUpgrade { // cct & cut bypass
		return
	}
	if tx.Type == types.TxUTXO && common.IsLKC(tx.TokenAddress) { // lkc utxo bypass
		return
	}
	if tx.RefundAddr == common.EmptyAddress {
		return
	}

	mgval := new(big.Int).Mul(new(big.Int).SetUint64(tx.Gas), tx.GasPrice)
	from := tx.RefundAddr
	if tx.State.GetBalance(from).Cmp(mgval) < 0 {
		log.Warn("buyGas: not enough gas", "hash", tx.Hash, "needval", mgval, "balance", tx.State.GetBalance(from))
		return errInsufficientBalanceForGas
	}
	tx.State.SubBalance(from, mgval)
	log.Debug("buyGas", "hash", tx.Hash, "needval", mgval, "balance", tx.State.GetBalance(from))
	return
}

func (tx *processTransaction) payIntrinsicGas() (err error) {
	var intrinsicGas uint64
	var intrinsicGasSum uint64
	for _, aout := range tx.Outputs {
		if aout.Type == Createout || aout.Type == Updateout {
			data := aout.Data
			gasRate := cfg.EvmGasRate
			if wasm.IsWasmContract(data) {
				gasRate = cfg.WasmGasRate
			}
			if intrinsicGas, err = IntrinsicGas(aout.Data, true, gasRate); err != nil {
				return
			}
			if (math.MaxUint64 - intrinsicGasSum) <= intrinsicGas {
				return errIntrinsicGasOverflow
			}
			intrinsicGasSum += intrinsicGas
		} else if aout.Type == Cout {
			data := tx.State.GetCode(aout.To)
			gasRate := cfg.EvmGasRate
			if wasm.IsWasmContract(data) {
				gasRate = cfg.WasmGasRate
			}
			if intrinsicGas, err = IntrinsicGas(aout.Data, false, gasRate); err != nil {
				return
			}
			if (math.MaxUint64 - intrinsicGasSum) <= intrinsicGas {
				return errIntrinsicGasOverflow
			}
			intrinsicGasSum += intrinsicGas
		}
	}
	if err = tx.useGas(intrinsicGas); err != nil {
		return
	}
	log.Debug("payIntrinsicGas", "hash", tx.Hash, "intrinsicGas", intrinsicGasSum, "gasleft", tx.Gas)
	return
}

func (tx *processTransaction) transitInputs() (vmerr error) {
	// check if account balance sufficient
	// TODO: move to checkTx
	for _, in := range tx.Inputs {
		balance := tx.State.GetTokenBalance(in.From, tx.TokenAddress)
		if balance.Cmp(in.Value) < 0 {
			log.Warn("transitInputs: insufficient balance", "hash", tx.Hash, "value", in.Value, "balance", balance, "from", in.From, "token", tx.TokenAddress)
			return types.ErrInsufficientFunds
		}
	}
	// sub balance
	for _, in := range tx.Inputs {
		tx.State.SubTokenBalance(in.From, tx.TokenAddress, in.Value)
		log.Debug("transitInputs: sub balance", "hash", tx.Hash, "from", in.From, "token", tx.TokenAddress, "value", in.Value)
	}
	return
}

func (tx *processTransaction) payTransferGas() (transferGas uint64, vmerr error) {
	transferGas = tx.Gas     // gas legality is checked in checkTx if no contract out
	if len(tx.Outputs) > 0 { //TODO: fix this to support multiple COUT/AOUT
		isContract := tx.Outputs[0].Type == Cout || tx.Outputs[0].Type == Createout || tx.Outputs[0].Type == Updateout
		isNewFeeRule := tx.Outputs[0].Amount.Sign() > 0
		isLianke := common.IsLKC(tx.TokenAddress)
		if isContract {
			if isNewFeeRule && isLianke {
				transferGas = types.CalNewAmountGas(tx.Outputs[0].Amount, types.EverContractLiankeFee)
			} else {
				transferGas = 0
			}
		}
	}
	if vmerr = tx.useGas(transferGas); vmerr != nil {
		log.Warn("pay transfer gas error", "hash", tx.Hash, "transferGas", transferGas, "gas", tx.Gas)
		return 0, vmerr
	}
	log.Debug("pay transfer gas", "hash", tx.Hash, "transferGas", transferGas, "gas", tx.Gas)
	return
}

func (tx *processTransaction) transitOutputs(res *TransitionResult) (vmerr error) {
	var (
		ret         []byte
		addr        common.Address
		byteCodeGas uint64
	)

	for _, out := range tx.Outputs {
		if out.Type == Createout {
			msgcode := out.Data
			//TODO: redefine these interface
			vm := tx.Vmenv.GetRealVm(msgcode, &common.EmptyAddress)
			vm.Reset(types.NewMessage(tx.RefundAddr, nil, tx.TokenAddress, tx.State.GetNonce(tx.RefundAddr), nil, 0, tx.GasPrice, nil))
			vm.SetToken(tx.TokenAddress)
			from := evm.AccountRef(tx.RefundAddr)
			ret, addr, tx.Gas, vmerr = vm.Create(from, out.Data, tx.Gas, out.Amount) //TODO: rewind mechanism
			log.Debug("transitOutputs: contract create", "hash", tx.Hash, "gas", tx.Gas, "vmerr", vmerr)
			if vmerr != nil {
				if bytes.HasPrefix(ret, cerrID) {
					var reason string
					if err := cabi.Unpack(&reason, cerrName, ret[len(cerrID):]); err == nil {
						vmerr = fmt.Errorf("%v: %s", vmerr, reason)
					}
				}
				log.Warn("transitOutputs: vm create error", "hash", tx.Hash, "msg", string(ret))
				tx.Gas += vm.RefundAllFee()
				return
			}
			tx.Gas += vm.RefundFee()
			res.Rets = append(res.Rets, ret)
			res.Addrs = append(res.Addrs, addr)
			res.Otxs = append(res.Otxs, vm.GetOTxs()...)
		} else if out.Type == Updateout {
			msgcode := tx.State.GetCode(out.To)
			//TODO: redefine these interface
			vm := tx.Vmenv.GetRealVm(msgcode, &common.EmptyAddress)
			vm.Reset(types.NewMessage(tx.RefundAddr, nil, tx.TokenAddress, tx.State.GetNonce(tx.RefundAddr), nil, 0, tx.GasPrice, nil))
			vm.SetToken(tx.TokenAddress)
			from := evm.AccountRef(tx.RefundAddr)
			vm.Upgrade(from, out.To, out.Data)
			log.Debug("transitOutputs: contract update", "hash", tx.Hash)

		} else if out.Type == Aout {
			tx.State.AddTokenBalance(out.To, tx.TokenAddress, out.Amount) // this cannot undone, so we dont refund here
			log.Debug("transitOutputs: add balance", "hash", tx.Hash, "to", out.To, "token", tx.TokenAddress, "amount", out.Amount)

		} else if out.Type == Cout {
			msgcode := tx.State.GetCode(out.To)
			//TODO: redefine these interface
			vm := tx.Vmenv.GetRealVm(msgcode, &common.EmptyAddress)
			vm.Reset(types.NewMessage(tx.RefundAddr, nil, tx.TokenAddress, tx.State.GetNonce(tx.RefundAddr), nil, 0, tx.GasPrice, nil))
			vm.SetToken(tx.TokenAddress)
			from := evm.AccountRef(tx.RefundAddr)
			ret, tx.Gas, byteCodeGas, vmerr = vm.UTXOCall(from, out.To, tx.TokenAddress, out.Data, tx.Gas, out.Amount)
			log.Debug("transitOutputs: contract call", "hash", tx.Hash, "gas", tx.Gas, "vmerr", vmerr)
			if vmerr != nil {
				if bytes.HasPrefix(ret, cerrID) {
					var reason string
					if err := cabi.Unpack(&reason, cerrName, ret[len(cerrID):]); err == nil {
						vmerr = fmt.Errorf("%v: %s", vmerr, reason)
					}
				}
				log.Warn("transitOutputs: vm call error", "hash", tx.Hash, "msg", string(ret))
				tx.Gas += vm.RefundAllFee()
				return
			}
			if out.To == cfg.ContractBlacklistAddr { // check black list contract
				log.Debug("transitOutputs: start to deal black addrs changes", "hash", tx.Hash, "msg", string(ret))
				types.BlacklistInstance.DealBlackAddrsChanges(ret)
			}
			res.Rets = append(res.Rets, ret)
			res.Otxs = append(res.Otxs, vm.GetOTxs()...)
			res.ByteCodeGas += byteCodeGas
			tx.Gas += vm.RefundFee()
		}
	}
	usedGas := tx.InitialGas - tx.Gas
	fee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(usedGas), tx.GasPrice)
	log.Debug("transition end", "hash", tx.Hash, "usedGas", usedGas, "fee", fee)
	res.Fee = fee
	return
}

func (tx *processTransaction) refundGas(transferGas uint64, snapshot int, vmerr error) {
	if vmerr != nil {
		// revert state
		tx.State.RevertToSnapshot(snapshot)
		// refund utxo value
		if tx.Type == types.TxUTXO && (tx.Kind&types.UinAout) == types.UinAout {
			refundValue := big.NewInt(0)
			for _, out := range tx.Outputs {
				if out.Type != Uout {
					refundValue.Add(refundValue, out.Amount)
				}
			}
			for _, in := range tx.Inputs {
				if in.Type != Uin {
					refundValue.Sub(refundValue, in.Value)
				}
			}
			tx.State.AddTokenBalance(tx.RefundAddr, tx.TokenAddress, refundValue)
		}
		// calculate refund gas (VM gas is refunded in processOutputs)
		tx.Gas += transferGas
	}
	// cct/cut do not buy gas or refund gas
	if tx.Type == types.TxContractCreate || tx.Type == types.TxContractUpgrade {
		tx.Gas = tx.InitialGas
		return
	}
	// do refund gas
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(tx.Gas), tx.GasPrice)
	if remaining.Sign() > 0 {
		log.Debug("refundGas to", "hash", tx.Hash, "addr", tx.RefundAddr, "gas", tx.Gas, "remaining", remaining)
		tx.State.AddBalance(tx.RefundAddr, remaining)
	}
}

func (tx *processTransaction) genTransitTxRecord(res *TransitionResult, vmerr error) {
	// 交易记录开关
	res.Gas = tx.InitialGas - tx.Gas
	res.Fee = new(big.Int).Mul(new(big.Int).SetUint64(tx.InitialGas-tx.Gas), tx.GasPrice)
	frontotxs := make([]types.BalanceRecord, 0) //otxs that need to insert to top
	if vmerr == nil {
		var otx types.BalanceRecord
		switch tx.Type {
		case types.TxNormal, types.TxToken:
			out := tx.Outputs[0]
			if out.Type == Createout {
				otx = types.GenBalanceRecord(tx.RefundAddr, res.Addrs[0], types.AccountAddress, types.AccountAddress, types.TxCreateContract, common.EmptyAddress, out.Amount)
			} else {
				otx = types.GenBalanceRecord(tx.RefundAddr, out.To, types.AccountAddress, types.AccountAddress, types.TxTransfer, tx.TokenAddress, out.Amount)
			}
			frontotxs = append(frontotxs, otx)
		case types.TxUTXO:
			for _, in := range tx.Inputs {
				if common.IsLKC(tx.TokenAddress) {
					fee := new(big.Int).Mul(new(big.Int).SetUint64(tx.InitialGas), tx.GasPrice)
					otx = types.GenBalanceRecord(in.From, common.EmptyAddress, types.AccountAddress, types.PrivateAddress, types.TxTransfer, tx.TokenAddress, big.NewInt(0).Sub(in.Value, fee))
				} else {
					otx = types.GenBalanceRecord(in.From, common.EmptyAddress, types.AccountAddress, types.PrivateAddress, types.TxTransfer, tx.TokenAddress, in.Value)
				}
				frontotxs = append(frontotxs, otx)
			}
			for _, out := range tx.Outputs {
				otx = types.GenBalanceRecord(tx.RefundAddr, out.To, types.PrivateAddress, types.AccountAddress, types.TxTransfer, tx.TokenAddress, out.Amount)
				frontotxs = append(frontotxs, otx)
			}

		case types.TxContractCreate:
			out := tx.Outputs[0]
			otx := types.GenBalanceRecord(tx.RefundAddr, res.Addrs[0], types.AccountAddress, types.AccountAddress, types.TxCreateContract, common.EmptyAddress, out.Amount)
			frontotxs = append(frontotxs, otx)
		}
	}
	if len(frontotxs) > 0 {
		res.Otxs = append(frontotxs, res.Otxs...)
	}
	// Fee Balance Record (even with zero fee)
	fromAddrType := types.AccountAddress
	if tx.RefundAddr == common.EmptyAddress {
		fromAddrType = types.PrivateAddress
	}
	otx := types.GenBalanceRecord(tx.RefundAddr, cfg.ContractFoundationAddr, fromAddrType, types.AccountAddress, types.TxFee, tx.TokenAddress, res.Fee)
	res.Otxs = append(res.Otxs, otx)
	return
}
