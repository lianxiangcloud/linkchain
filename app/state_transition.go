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
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"

	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/vm"
	"github.com/lianxiangcloud/linkchain/vm/evm"
)

var (
	errInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
)

/*
The State Transitioning Model

A state transition is a change made when a transaction is applied to the current world state
The state transitioning model does all all the necessary work to work out a valid new state root.

1) Nonce handling
2) Pre pay gas
3) Create a new state object if the recipient is \0*32
4) Value transfer
== If contract creation ==
  4a) Attempt to run transaction data
  4b) If valid, use result as code for the new state object
== end ==
5) Run Script section
6) Derive new state root
*/
type StateTransition struct {
	msg        *types.Message
	gas        uint64
	gasPrice   *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      types.StateDB
	vmenv      vm.VmInterface
	tokenAddr  common.Address
}

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, contractCreation, homestead bool, gasRate uint64) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if contractCreation && homestead {
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
			return 0, evm.ErrOutOfGas
		}
		gas += nz * cfg.TxDataNonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/cfg.TxDataZeroGas < z {
			log.Warn("IntrinsicGas", "gas", gas, "z", z)
			return 0, evm.ErrOutOfGas
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

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(vmenv vm.VmInterface, msg types.IMessage, tx types.Tx, tokenAddr common.Address) *StateTransition {
	st := &StateTransition{
		vmenv:     vmenv,
		state:     vmenv.GetStateDB(),
		tokenAddr: tokenAddr,
	}
	message, _ := msg.AsMessage()
	log.Debug("NewStateTransition", "message", message)
	if message.TxType() == types.TxUTXO {
		from, err := message.From(tx, vmenv.GetStateDB())
		log.Debug("NewStateTransition", "from", from, "err", err)
	}
	st.msg = &message
	return st
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any EVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyUTXOMessage(vmenv vm.VmInterface, tx *types.UTXOTransaction, tokenAddr common.Address) ([]byte, uint64, uint64, *big.Int, error, error) {
	return NewStateTransition(vmenv, tx, tx, tokenAddr).UTXOTransitionDb()
}

func ApplyMessage(vmenv vm.VmInterface, msg types.IMessage, tokenAddr common.Address) ([]byte, uint64, uint64, *big.Int, error, error) {
	return NewStateTransition(vmenv, msg, nil, tokenAddr).Transition()
}

func (st *StateTransition) Transition() (ret []byte, usedGas uint64, byteCodeGas uint64, fee *big.Int, vmerr error, err error) {
	if st.msg.TxType() == types.TxUTXO {
		return st.UTXOTransitionDb()
	}
	return st.TransitionDb()
}

// to returns the recipient of the message.
func (st *StateTransition) to() common.Address {
	if st.msg == nil || st.msg.To() == nil /* contract creation */ {
		return common.EmptyAddress
	}
	return *st.msg.To()
}

func (st *StateTransition) useGas(amount uint64) error {
	if st.gas < amount {
		return evm.ErrOutOfGas
	}
	st.gas -= amount

	return nil
}

func (st *StateTransition) buyGas() error {
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.msg.GasPrice())
	if st.state.GetBalance(st.msg.MsgFrom()).Cmp(mgval) < 0 {
		return errInsufficientBalanceForGas
	}

	st.gas += st.msg.Gas()
	st.initialGas = st.msg.Gas()
	st.state.SubBalance(st.msg.MsgFrom(), mgval)
	return nil
}

func (st *StateTransition) preCheck() error {
	// Make sure this transaction's nonce is correct.
	nonce := st.state.GetNonce(st.msg.MsgFrom())
	if nonce < st.msg.Nonce() {
		log.Warn("UTXO pre check too high", "from", st.msg.MsgFrom(), "stateNonece", nonce, "TxNonce", st.msg.Nonce())
		return types.ErrNonceTooHigh
	} else if nonce > st.msg.Nonce() {
		log.Warn("UTXO pre check too low", "from", st.msg.MsgFrom(), "stateNonece", nonce, "TxNonce", st.msg.Nonce())
		return types.ErrNonceTooLow
	}
	return nil
}

//if AccountInput => UTXOOutput, collect All Fee
//if AccountInput => UTXOOutput + AccountOutput, collect Fee by vm consume or gas policy
//if UTXOInput => AccountOutput + UTXOOutput, collect Fee by vm consume or gas policy
func (st *StateTransition) UTXOTransitionDb() (ret []byte, usedGas uint64, byteCodeGas uint64, fee *big.Int, vmerr error, err error) {
	msg := st.msg
	log.Debug("UTXOTransitionDb", "msg", msg)
	contractValue := big.NewInt(0)

	//accountInput precheck nonce
	if (msg.UTXOKind() & types.Ain) == types.Ain {
		if err = st.preCheck(); err != nil {
			return
		}
	}

	recordFee := big.NewInt(0).Mul(msg.GasPrice(), big.NewInt(0).SetUint64(msg.Gas()))
	recordAmount := big.NewInt(0).Sub(msg.Value(), recordFee)

	for {
		//Fee -> Gas
		st.gas += msg.Gas()
		st.initialGas = msg.Gas()

		//only AccountInput from accountInput's owner address
		if (msg.UTXOKind() & types.Aout) != types.Aout {
			//do not care about UTXOIuput or AccountInput, CheckTx make sure Fee is efficient, subtract all gas
			st.useGas(st.gas)
			if (msg.UTXOKind() & types.Ain) == types.Ain {
				st.state.SubTokenBalance(msg.MsgFrom(), msg.TokenAddress(), msg.Value())
                if !common.IsLKC(msg.TokenAddress()) {
                    paidFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(msg.Gas()), big.NewInt(types.ParGasPrice))
				    st.state.SubBalance(msg.MsgFrom(), paidFee)
                }

				br := types.GenBalanceRecord(msg.MsgFrom(), common.EmptyAddress, types.AccountAddress, types.PrivateAddress, types.TxTransfer, msg.TokenAddress(), recordAmount)
				st.vmenv.AddOtx(br)
			}
			log.Debug("no acountOutput, charge remaining gas, Process finished")
			break
		}

		//AccountOutput only transfer value, support multi output
		accountOutputs := msg.OutputData()
		if len(accountOutputs) <= 0 {
			err = fmt.Errorf("invalid accountOutput")
			return
		}
		if !st.state.IsContract(accountOutputs[0].To) {
			st.useGas(st.gas)

			if (msg.UTXOKind() & types.Ain) == types.Ain {
				log.Debug("acountOutput only transfer value", "from", msg.MsgFrom(), "token", msg.TokenAddress().String(), "value", msg.Value())
				st.state.SubTokenBalance(msg.MsgFrom(), msg.TokenAddress(), msg.Value())
				if !common.IsLKC(msg.TokenAddress()) {
					paidFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(msg.Gas()), big.NewInt(types.ParGasPrice))
					st.state.SubBalance(msg.MsgFrom(), paidFee)
				}
				br := types.GenBalanceRecord(msg.MsgFrom(), common.EmptyAddress, types.AccountAddress, types.PrivateAddress, types.TxTransfer, msg.TokenAddress(), recordAmount)
				st.vmenv.AddOtx(br)
			}

			for _, accountOutput := range accountOutputs {
				st.state.AddTokenBalance(accountOutput.To, msg.TokenAddress(), accountOutput.Amount)
				log.Debug("acountOutput only transfer value", "to", accountOutput.To, "token", msg.TokenAddress().String(), "value", accountOutput.Amount)
				br := types.GenBalanceRecord(common.EmptyAddress, accountOutput.To, types.PrivateAddress, types.AccountAddress, types.TxTransfer, msg.TokenAddress(), accountOutput.Amount)
				st.vmenv.AddOtx(br)
			}

			break
		}

		//call contract
		contractAddr := accountOutputs[0].To
		contractData := accountOutputs[0].Data
		contractValue = accountOutputs[0].Amount
		inputValue := big.NewInt(0).Set(msg.Value())
		fee = big.NewInt(0).Mul(msg.GasPrice(), big.NewInt(0).SetUint64(msg.Gas()))
		if inputValue.Cmp(fee) <= 0 {
			inputValue = big.NewInt(0)
		} else {
			inputValue.Sub(inputValue, fee)
		}

		gasRate := st.vmenv.GasRate()
		gas, _ := IntrinsicGas(contractData, false, true, gasRate)
		log.Debug("UTXOTransitionDb IntrinsicGas", "gas", gas, "gasRate", gasRate, "st.gas", st.gas, "value", inputValue)
		if err = st.useGas(gas); err != nil {
			log.Warn("UTXOTransitionDb out of gas", "need IntrinsicGas", gas, "have gas", st.gas)
			return
		}

		//from here UTXO already spended, we regard all err as vmerr
		if (msg.UTXOKind() & types.Uin) == types.Uin {
			coe := GetCoefficient(st.state, log.Root())
			utxoGas := coe.UTXOFee.Uint64()
			if vmerr = st.useGas(utxoGas); vmerr != nil {
				log.Warn("UTXOTransitionDb out of gas", "need UTXOGas", utxoGas, "have gas", st.gas)
				break
			}
		}

		var transfervalueGas uint64
		isNewFeeRule := inputValue.Sign() > 0
		if isNewFeeRule {
			transfervalueGas = types.CalNewAmountGas(inputValue)
			if vmerr = st.useGas(transfervalueGas); vmerr != nil {
				log.Warn("UTXOTransitionDb out of gas", "transfer value need gas", transfervalueGas, "have gas", st.gas)
				st.useGas(st.gas)
				break
			}
		}

		if (msg.UTXOKind() & types.Ain) == types.Ain {
			fromBalance := st.state.GetTokenBalance(msg.MsgFrom(), msg.TokenAddress())
			if fromBalance.Cmp(msg.Value()) < 0 {
				log.Warn("UTXOTransitionDb insufficient balance", "transfer value", msg.Value(), "from", msg.MsgFrom().String(), "token", msg.TokenAddress().String(), "balance", fromBalance)
				st.useGas(st.gas)
				vmerr = types.ErrInsufficientFunds
				break
			}
		}

		if (msg.UTXOKind() & types.Ain) == types.Ain {
			log.Debug("UTXOTransitionDb sub accountInput's amount", "from", msg.MsgFrom(), "token", msg.TokenAddress().String(),  "value", msg.Value())
			st.state.SubTokenBalance(msg.MsgFrom(), msg.TokenAddress(), msg.Value())
			if !common.IsLKC(msg.TokenAddress()) {
				paidFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(msg.Gas()), big.NewInt(types.ParGasPrice))
				st.state.SubBalance(msg.MsgFrom(), paidFee)
			}
			br := types.GenBalanceRecord(msg.MsgFrom(), common.EmptyAddress, types.AccountAddress, types.PrivateAddress, types.TxTransfer, msg.TokenAddress(), recordAmount)
			st.vmenv.AddOtx(br)
		}

		sender := evm.AccountRef(msg.MsgFrom())
		log.Debug("UTXOTransitionDb before contract Call", "st.gas", st.gas)
		ret, st.gas, byteCodeGas, vmerr = st.vmenv.UTXOCall(sender, contractAddr, msg.TokenAddress(), contractData, st.gas, contractValue)
		log.Debug("UTXOTransitionDb after contract Call", "st.gas", st.gas, "byteCodeGas", byteCodeGas, "vmerr", vmerr)

		if vmerr != nil && isNewFeeRule {
			log.Debug("UTXOTransitionDb vmerr happened, refund partial gas", "transfervalueGas", transfervalueGas)
			st.gas += transfervalueGas
		}
		// check black list contract
		if vmerr == nil && contractAddr == cfg.ContractBlacklistAddr {
			log.Debug("start to deal black addrs changes", "msg", string(ret))
			types.BlacklistInstance().DealBlackAddrsChanges(ret)
		}

		break
	}

	fee = new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), msg.GasPrice())
	st.refundGas()

	if (msg.UTXOKind() & types.Ain) == types.Ain {
		st.state.SetNonce(msg.MsgFrom(), st.state.GetNonce(msg.MsgFrom())+1)
	}

	//vmerr happened, UTXO spended, we must refund spended money to caller's address(that is RefundAddr)
	//UTXO+Amount = ContractValue + Fee = Refund + Gas
	//Refund = ContractValue + Fee - Gas
	//Fee - Gas refunded, so refund ContractValue here.
	if vmerr != nil {
		st.state.AddTokenBalance(msg.MsgFrom(), msg.TokenAddress(), contractValue)
		log.Debug("UTXOTransitionDb vmerr happened, refund spended money to", "from", msg.MsgFrom(), "token", msg.TokenAddress().String(), "refundValue", contractValue)
	}

	log.Debug("UTXOTransitionDb end", "coinBase", st.vmenv.GetCoinbase(), "gasUsed", st.gasUsed(), "initalgas", st.initialGas, "fee", fee.String())

	return ret, st.gasUsed(), byteCodeGas, fee, vmerr, err
}

// TransitionDb will transition the state by applying the current message and
// returning the result including the the used gas. It returns an error if it
// failed. An error indicates a consensus issue.
func (st *StateTransition) TransitionDb() (ret []byte, usedGas uint64, byteCodeGas uint64, fee *big.Int, vmerr error, err error) {
	if err = st.preCheck(); err != nil {
		return
	}
	msg := st.msg
	log.Debug("TransitionDb", "msg", msg)
	sender := evm.AccountRef(msg.MsgFrom())
	isContractCreationTx := (msg.TxType() == types.TxContractCreate || msg.TxType() == types.TxContractUpgrade)
	contractCreation := (msg.To() == nil && msg.TxType() == types.TxNormal) || (isContractCreationTx)

	if isContractCreationTx {
		st.gas += msg.Gas()
		st.initialGas = msg.Gas()
	} else {
		if err = st.buyGas(); err != nil {
			return
		}
	}

	var (
		vmenv        = st.vmenv
		intrinsicGas uint64
	)

	gasRate := vmenv.GasRate()

	// Pay intrinsic gas
	intrinsicGas, err = IntrinsicGas(msg.Data(), contractCreation, true, gasRate)
	if err != nil {
		return
	}

	log.Debug("TransitionDb", "IntrinsicGas", intrinsicGas, "gasRate", gasRate, "st.gas", st.gas)

	if err = st.useGas(intrinsicGas); err != nil {
		log.Warn("useGas", "st.gas", st.gas, "intrinsicGas", intrinsicGas, "err", err)
		return
	}

	if msg.TxType() == types.TxContractUpgrade {
		vmenv.Upgrade(msg.MsgFrom(), *msg.To(), msg.Data())
	} else if msg.To() == nil {
		if contractCreation {
			ret, _, st.gas, vmerr = vmenv.Create(sender, msg.Data(), st.gas, msg.Value())
			log.Debug("contract Create", "st.gas", st.gas, "vmerr", vmerr)
		} else {
			st.state.SetNonce(msg.MsgFrom(), st.state.GetNonce(sender.Address())+1)
			st.state.SubTokenBalance(msg.MsgFrom(), msg.TokenAddress(), msg.Value())
			br := types.GenBalanceRecord(msg.MsgFrom(), common.EmptyAddress, types.AccountAddress, types.NoAddress, types.TxTransfer, msg.TokenAddress(), msg.Value())
			vmenv.AddOtx(br)

			log.Debug("contract Create, but this is from")
		}
	} else {
		st.state.SetNonce(msg.MsgFrom(), st.state.GetNonce(sender.Address())+1)

		var totalFee uint64
		isNewFeeRule := st.state.IsContract(*msg.To()) && msg.Value().Sign() > 0
		if isNewFeeRule {
			totalFee = types.CalNewAmountGas(msg.Value())
			if vmerr = st.useGas(totalFee); vmerr != nil {
				st.useGas(st.gas)
			}
		}
		if vmerr == nil {
			log.Debug("before contract Call", "st.gas", st.gas)
			ret, st.gas, byteCodeGas, vmerr = vmenv.Call(sender, *msg.To(), msg.TokenAddress(), msg.Data(), st.gas, msg.Value())
			log.Debug("after contract Call", "st.gas", st.gas)
		}
		if vmerr != nil && isNewFeeRule {
			st.gas += totalFee
		}
		// check black list contract
		if vmerr == nil && *msg.To() == cfg.ContractBlacklistAddr {
			log.Debug("start to deal black addrs changes", "msg", string(ret))
			types.BlacklistInstance().DealBlackAddrsChanges(ret)
		}

		log.Debug("contract Call", "st.gas", st.gas, "byteCodeGas", byteCodeGas, "vmerr", vmerr)
	}

	if msg.To() != nil && !st.state.IsContract(*msg.To()) {
		log.Debug("deduct all gas", "remainGas", st.gas, "gasLimit", msg.Gas(), "gasPrice", msg.GasPrice().String())
		st.useGas(st.gas)
	} else {
		// create a contract or call a contract
		// no need to consume all gas
		log.Debug("gas info", "gasLimit", msg.Gas(), "gasUsed", st.gasUsed(), "gasPrice", msg.GasPrice().String())
	}

	if isContractCreationTx {
		fee = new(big.Int).SetUint64(0)
		st.gas += st.gasUsed()
	} else {
		fee = new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), msg.GasPrice())
		st.refundGas()
	}

	log.Debug("TransitionDB:", "gasUsed", st.gasUsed())
	return ret, st.gasUsed(), byteCodeGas, fee, vmerr, err
}

func isThirdPartyData(data []byte) bool {
	// third party json payload
	var extra map[string]interface{}
	if err := json.Unmarshal(data, &extra); err == nil {
		return true
	}
	return false
}

func (st *StateTransition) refundGas() {
	// Return ETH for remaining gas, exchanged at the original rate.
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.msg.GasPrice())
	if remaining.Sign() > 0 {
		log.Debug("refundGas to", "from", st.msg.MsgFrom(), "gas", st.gas, "remaining", remaining)
		st.state.AddBalance(st.msg.MsgFrom(), remaining)
	}
}

// gasUsed returns the amount of gas used up by the state transition.
func (st *StateTransition) gasUsed() uint64 {
	return st.initialGas - st.gas
}
