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

package evm

import (
	"math/big"
	"sync/atomic"
	"time"

	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
)

var defaultDifficulty = big.NewInt(10000000)

// emptyCodeHash is used by create to ensure deployment is disallowed to already
// deployed contract addresses (relevant after the account abstraction).
var emptyCodeHash = crypto.Keccak256Hash(nil)

// ChainContext supports retrieving headers from the
// current blockchain to be used during transaction processing.
type ChainContext interface {
	// GetHeader returns the hash corresponding to their hash.
	GetHeader(uint64) *types.Header
}

type (
	// CanTransferFunc is the signature of a transfer guard function
	CanTransferFunc func(types.StateDB, common.Address, common.Address, *big.Int) bool
	// TransferFunc is the signature of a transfer function
	TransferFunc func(types.StateDB, common.Address, common.Address, common.Address, *big.Int)
	// GetHashFunc returns the nth block hash in the blockchain
	// and is used by the BLOCKHASH EVM op code.
	GetHashFunc func(uint64) common.Hash
	//UnsafeTransferFunc add token balance to ToAddr
	UnsafeTransferFunc func(types.StateDB, common.Address, common.Address, *big.Int)
)

// Context provides the EVM with auxiliary information. Once provided
// it shouldn't be modified.
type Context struct {
	// CanTransfer returns whether the account contains
	// sufficient ether to transfer the value
	CanTransfer CanTransferFunc
	// Transfer transfers ether from one account to the other
	Transfer       TransferFunc
	UnsafeTransfer UnsafeTransferFunc
	// GetHash returns the hash corresponding to n
	GetHash GetHashFunc

	// Message information
	Token    common.Address //Provides the tx token type
	Origin   common.Address // Provides information for ORIGIN
	GasPrice *big.Int       // Provides information for GASPRICE

	// Block information
	Coinbase    common.Address // Provides information for COINBASE
	GasLimit    uint64         // Provides information for GASLIMIT
	BlockNumber *big.Int       // Provides information for NUMBER
	Time        *big.Int       // Provides information for TIME
	Difficulty  *big.Int       // Provides information for DIFFICULTY

	EvmGasRate uint64
}

// NewEVMContext creates a new context for use in the EVM.
func NewEVMContext(header *types.Header, chain ChainContext, author *common.Address, gasRate uint64) Context {
	// If we don't have an explicit author (i.e. not mining), extract from the header
	var beneficiary common.Address
	if author == nil {
		beneficiary = header.Coinbase // Ignore error, we're past header validation
	} else {
		beneficiary = *author
	}

	ctx := Context{
		CanTransfer:    CanTransfer,
		Transfer:       Transfer,
		UnsafeTransfer: UnsafeTransfer,
		GetHash:        GetHashFn(header, chain),
		Coinbase:       beneficiary,
		BlockNumber:    new(big.Int).SetUint64(header.Height),
		Time:           new(big.Int).SetUint64(header.Time),
		Difficulty:     new(big.Int).Set(defaultDifficulty),
		GasLimit:       header.GasLimit,
		EvmGasRate:     gasRate,
	}

	return ctx
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by number
func GetHashFn(ref *types.Header, chain ChainContext) func(n uint64) common.Hash {
	var cache map[uint64]common.Hash

	return func(n uint64) common.Hash {
		// If there's no hash cache yet, make one
		if cache == nil {
			cache = map[uint64]common.Hash{
				ref.Height - 1: ref.ParentHash,
			}
		}
		// Try to fulfill the request from the cache
		if hash, ok := cache[n]; ok {
			return hash
		}
		// Not cached, iterate the blocks and cache the hashes
		for header := chain.GetHeader(ref.Height - 1); header != nil; header = chain.GetHeader(header.Height - 1) {
			cache[header.Height-1] = header.ParentHash
			if n == header.Height-1 {
				return header.ParentHash
			}

			if header.Height == 0 {
				break
			}
		}
		return common.EmptyHash
	}
}

// CanTransfer checks wether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db types.StateDB, addr, token common.Address, amount *big.Int) bool {
	return db.GetTokenBalance(addr, token).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db types.StateDB, sender, recipient, token common.Address, amount *big.Int) {
	db.SubTokenBalance(sender, token, amount)
	db.AddTokenBalance(recipient, token, amount)
}

// UnsafeTransfer adds amount to recipient using the given Db
func UnsafeTransfer(db types.StateDB, recipient, token common.Address, amount *big.Int) {
	db.AddTokenBalance(recipient, token, amount)
}

// run runs the given contract and takes care of running precompiles with a fallback to the byte code interpreter.
func run(evm *EVM, c types.Contract, input []byte, readOnly bool) ([]byte, error) {
	contract := c.(*Contract)
	if contract.CodeAddr != nil {
		precompiles := PrecompiledContractsHomestead
		if p := precompiles[*contract.CodeAddr]; p != nil {
			return RunPrecompiledContract(p, input, contract)
		}
	}
	return evm.interpreter.Run(contract, input, readOnly)
}

// EVM is the Ethereum Virtual Machine base object and provides
// the necessary tools to run a contract on the given state with
// the provided context. It should be noted that any error
// generated through any of the calls should be considered a
// revert-state-and-consume-all-gas operation, no checks on
// specific errors should ever be performed. The interpreter makes
// sure that any errors generated are to be considered faulty code.
//
// The EVM should never be reused and is not thread safe.
type EVM struct {
	// Context provides auxiliary blockchain related information
	Context
	// StateDB gives access to the underlying state
	StateDB types.StateDB
	// Depth is the current call stack
	depth int

	// virtual machine configuration options used to initialise the
	// evm.
	vmConfig Config
	// global (to this context) ethereum virtual machine
	// used throughout the execution of the tx.
	interpreter *Interpreter
	// abort is used to abort the EVM calling operations
	// NOTE: must be set atomically
	abort int32
	// callGasTemp holds the gas available for the current call. This is needed because the
	// available gas is calculated in gasCall* according to the 63/64 rule and later
	// applied in opCall*.
	callGasTemp uint64

	otxs        []types.BalanceRecord
	fees        []uint64
	refundFees  []uint64
	feeSaved    bool
}

// NewEVM returns a new EVM. The returned EVM is not thread safe and should
// only ever be used *once*.
func NewEVM(c types.Context, statedb types.StateDB, vmc types.VmConfig) *EVM {
	ctx := c.(Context)
	vmConfig := vmc.(Config)

	evm := &EVM{
		Context:    ctx,
		StateDB:    statedb,
		vmConfig:   vmConfig,
		otxs:       make([]types.BalanceRecord, 0),
		fees:       make([]uint64, 0),
		refundFees: make([]uint64, 0),
	}

	evm.interpreter = NewInterpreter(evm, vmConfig)
	return evm
}

func (evm *EVM) Reset(msg types.Message) {
	evm.depth = 0
	evm.abort = 0
	evm.callGasTemp = 0

	evm.Context.Origin   = msg.MsgFrom()
	evm.Context.GasPrice = new(big.Int).Set(msg.GasPrice())
	evm.Context.Token    = msg.TokenAddress()
	evm.otxs             = make([]types.BalanceRecord, 0)
	evm.fees             = make([]uint64, 0)
	evm.refundFees       = make([]uint64, 0)

	evm.interpreter.readOnly = false
	evm.interpreter.returnData = nil
}

// Cancel cancels any running EVM operation. This may be called concurrently and
// it's safe to be called multiple times.
func (evm *EVM) Cancel() {
	atomic.StoreInt32(&evm.abort, 1)
}

func (evm *EVM) UTXOCall(c types.ContractRef, addr, token common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error) {
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, 0, nil
	}

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(cfg.CallCreateDepth) {
		return nil, gas, 0, ErrDepth
	}

	var (
		to       = AccountRef(addr)
		snapshot = evm.StateDB.Snapshot()
	)
	if !evm.StateDB.Exist(addr) {
		precompiles := PrecompiledContractsHomestead
		if precompiles[addr] == nil && value.Sign() == 0 {
			// Calling a non existing account, don't do antything
			return nil, gas, 0, nil
		}
		evm.StateDB.CreateAccount(addr)
	}
	evm.UnsafeTransfer(evm.StateDB, to.Address(), token, value)

	// Initialise a new contract and set the code that is to be used by the EVM.
	// The contract is a scoped environment for this execution context only.
	caller := c.(ContractRef)
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	ret, err = run(evm, contract, input, false)

	// When an error was returned by the EVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
    gasUsed := gas - contract.Gas
    if gasUsed < contract.ByteCodeGas {
        byteCodeGas = contract.ByteCodeGas - gasUsed
    }
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != types.ExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, byteCodeGas, err
}

// Call executes the contract associated with the addr with the given input as
// parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
func (evm *EVM) Call(c types.ContractRef, addr, token common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error) {
	caller := c.(ContractRef)
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, 0, nil
	}

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(cfg.CallCreateDepth) {
		return nil, gas, 0, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if !evm.Context.CanTransfer(evm.StateDB, caller.Address(), token, value) {
		return nil, gas, 0, ErrInsufficientBalance
	}

	var (
		to       = AccountRef(addr)
		snapshot = evm.StateDB.Snapshot()
	)
	if !evm.StateDB.Exist(addr) {
		precompiles := PrecompiledContractsHomestead
		if precompiles[addr] == nil && value.Sign() == 0 {
			// Calling a non existing account, don't do antything, but ping the tracer
			if evm.vmConfig.Debug && evm.depth == 0 {
				evm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)
				evm.vmConfig.Tracer.CaptureEnd(ret, 0, 0, nil)
			}
			return nil, gas, 0, nil
		}
		evm.StateDB.CreateAccount(addr)
	}
	evm.Transfer(evm.StateDB, caller.Address(), to.Address(), token, value)
	if evm.depth == 0 {
		br := types.GenBalanceRecord(caller.Address(), to.Address(), types.AccountAddress, types.AccountAddress, types.TxTransfer, token, value)
		evm.otxs = append(evm.otxs, br)
	} else {
		br := types.GenBalanceRecord(caller.Address(), to.Address(), types.AccountAddress, types.AccountAddress, types.TxContract, token, value)
		evm.otxs = append(evm.otxs, br)
	}

	// Initialise a new contract and set the code that is to be used by the EVM.
	// The contract is a scoped environment for this execution context only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	start := time.Now()

	// Capture the tracer start/end events in debug mode
	if evm.vmConfig.Debug && evm.depth == 0 {
		evm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)

		defer func() { // Lazy evaluation of the parameters
			evm.vmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
		}()
	}
	ret, err = run(evm, contract, input, false)

	// When an error was returned by the EVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
    gasUsed := gas - contract.Gas
    if gasUsed < contract.ByteCodeGas {
        byteCodeGas = contract.ByteCodeGas - gasUsed
    }
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != types.ExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, byteCodeGas, err
}

// CallCode executes the contract associated with the addr with the given input
// as parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
//
// CallCode differs from Call in the sense that it executes the given address'
// code with the caller as context.
func (evm *EVM) CallCode(c types.ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error) {
	caller := c.(ContractRef)
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, 0, nil
	}

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(cfg.CallCreateDepth) {
		return nil, gas, 0, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if !evm.CanTransfer(evm.StateDB, caller.Address(), common.EmptyAddress, value) {
		return nil, gas, 0, ErrInsufficientBalance
	}

	var (
		snapshot = evm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)
	// initialise a new contract and set the code that is to be used by the
	// EVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	ret, err = run(evm, contract, input, false)
    gasUsed := gas - contract.Gas
    if gasUsed < contract.ByteCodeGas {
        byteCodeGas = contract.ByteCodeGas - gasUsed
    }
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != types.ExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, byteCodeGas, err
}

// DelegateCall executes the contract associated with the addr with the given input
// as parameters. It reverses the state in case of an execution error.
//
// DelegateCall differs from CallCode in the sense that it executes the given address'
// code with the caller as context and the caller is set to the caller of the caller.
func (evm *EVM) DelegateCall(c types.ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error) {
	caller := c.(ContractRef)
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, 0, nil
	}
	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(cfg.CallCreateDepth) {
		return nil, gas, 0, ErrDepth
	}

	var (
		snapshot = evm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)

	// Initialise a new contract and make initialise the delegate values
	contract := NewContract(caller, to, nil, gas).AsDelegate()
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	ret, err = run(evm, contract, input, false)
    gasUsed := gas - contract.Gas
    if gasUsed < contract.ByteCodeGas {
        byteCodeGas = contract.ByteCodeGas - gasUsed
    }
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != types.ExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, byteCodeGas, err
}

// StaticCall executes the contract associated with the addr with the given input
// as parameters while disallowing any modifications to the state during the call.
// Opcodes that attempt to perform such modifications will result in exceptions
// instead of performing the modifications.
func (evm *EVM) StaticCall(c types.ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error) {
	caller := c.(ContractRef)
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, 0, nil
	}
	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(cfg.CallCreateDepth) {
		return nil, gas, 0, ErrDepth
	}
	// Make sure the readonly is only set if we aren't in readonly yet
	// this makes also sure that the readonly flag isn't removed for
	// child calls.
	if !evm.interpreter.readOnly {
		evm.interpreter.readOnly = true
		defer func() { evm.interpreter.readOnly = false }()
	}

	var (
		to       = AccountRef(addr)
		snapshot = evm.StateDB.Snapshot()
	)
	// Initialise a new contract and set the code that is to be used by the
	// EVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, to, new(big.Int), gas)
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	// When an error was returned by the EVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in Homestead this also counts for code storage gas errors.
	ret, err = run(evm, contract, input, true) 
    gasUsed := gas - contract.Gas
    if gasUsed < contract.ByteCodeGas {
        byteCodeGas = contract.ByteCodeGas - gasUsed
    }
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != types.ExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, byteCodeGas, err
}

type codeAndHash struct {
	code []byte
	hash common.Hash
}

func (c *codeAndHash) Hash() common.Hash {
	if c.hash == (common.Hash{}) {
		c.hash = crypto.Keccak256Hash(c.code)
	}
	return c.hash
}

// Create creates a new contract using code as deployment code.
func (evm *EVM) create(caller ContractRef, codeAndHash *codeAndHash, gas uint64, value *big.Int, contractAddr common.Address) ([]byte, common.Address, uint64, error) {
	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if evm.depth > int(cfg.CallCreateDepth) {
		return nil, common.EmptyAddress, gas, ErrDepth
	}

	if evm.depth != 0 {
		if !evm.CanTransfer(evm.StateDB, caller.Address(), common.EmptyAddress, value) {
			return nil, common.EmptyAddress, gas, ErrInsufficientBalance
		}
		nonce := evm.StateDB.GetNonce(caller.Address())
		evm.StateDB.SetNonce(caller.Address(), nonce+1)
	}

	// Ensure there's no existing contract already at the designated address
	contractHash := evm.StateDB.GetCodeHash(contractAddr)
	if evm.StateDB.GetNonce(contractAddr) != 0 || (contractHash != common.EmptyHash && contractHash != emptyCodeHash) {
		return nil, common.EmptyAddress, 0, ErrContractAddressCollision
	}
	// Create a new account on the state
	snapshot := evm.StateDB.Snapshot()
	evm.StateDB.CreateAccount(contractAddr)
	evm.StateDB.SetNonce(contractAddr, 1)

	if evm.depth == 0 {
		evm.UnsafeTransfer(evm.StateDB, contractAddr, common.EmptyAddress, value)
	} else {
		evm.Transfer(evm.StateDB, caller.Address(), contractAddr, common.EmptyAddress, value)
		br := types.GenBalanceRecord(caller.Address(), contractAddr, types.AccountAddress, types.AccountAddress, types.TxCreateContract, common.EmptyAddress, value)
		evm.otxs = append(evm.otxs, br)
	}

	// initialise a new contract and set the code that is to be used by the
	// EVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, AccountRef(contractAddr), value, gas)
	contract.SetCodeOptionalHash(&contractAddr, codeAndHash)

	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, contractAddr, gas, nil
	}

	if evm.vmConfig.Debug && evm.depth == 0 {
		evm.vmConfig.Tracer.CaptureStart(caller.Address(), contractAddr, true, codeAndHash.code, gas, value)
	}
	start := time.Now()

	ret, err := run(evm, contract, nil, false)

	// check whether the max code size has been exceeded
	maxCodeSizeExceeded := len(ret) > cfg.MaxCodeSize
	// if the contract creation ran successfully and no errors were returned
	// calculate the gas required to store the code. If the code could not
	// be stored due to not enough gas set an error and let it be handled
	// by the error checking condition below.
	if err == nil && !maxCodeSizeExceeded {
		createDataGas := uint64(len(ret)) * cfg.CreateDataGas
		if contract.UseGas(createDataGas) {
			evm.StateDB.SetCode(contractAddr, ret)
		} else {
			err = ErrCodeStoreOutOfGas
		}
	}

	// When an error was returned by the EVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
	if maxCodeSizeExceeded || err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != types.ExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	// Assign err if contract code size exceeds the max while the err is still empty.
	if maxCodeSizeExceeded && err == nil {
		err = errMaxCodeSizeExceeded
	}
	if evm.vmConfig.Debug && evm.depth == 0 {
		evm.vmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
	}
	return ret, contractAddr, contract.Gas, err
}

// Create creates a new contract using code as deployment code.
func (evm *EVM) Create(c types.ContractRef, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {

	caller := c.(ContractRef)
	contractAddr = crypto.CreateAddress(caller.Address(), evm.StateDB.GetNonce(caller.Address()), code)

	return evm.create(caller, &codeAndHash{code: code}, gas, value, contractAddr)
}

// Create2 creates a new contract using code as deployment code.
//
// The different between Create2 with Create is Create2 uses sha3(0xff ++ msg.sender ++ salt ++ sha3(init_code))[12:]
// instead of the usual sender-and-nonce-hash as the address where the contract is initialized at.
func (evm *EVM) Create2(c types.ContractRef, code []byte, gas uint64, endowment *big.Int, salt *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	caller := c.(ContractRef)
	codeAndHash := &codeAndHash{code: code}
	contractAddr = crypto.CreateAddress2(caller.Address(), common.BigToHash(salt), codeAndHash.Hash().Bytes())
	return evm.create(caller, codeAndHash, gas, endowment, contractAddr)
}

// Interpreter returns the EVM interpreter
func (evm *EVM) Interpreter() types.Interpreter { return evm.interpreter }

func (evm *EVM) Upgrade(c types.ContractRef, contactAddr common.Address, code []byte) {
	log.Error("evm should not support upgrade")
}

//Token
func (evm *EVM) SetToken(token common.Address) {
	evm.Token = token
}

//Coinbase
func (evm *EVM) GetCoinbase() common.Address {
	return evm.Coinbase
}

func (evm *EVM) GetBlockNumber() *big.Int {
	return evm.BlockNumber
}

//Time
func (evm *EVM) GetTime() *big.Int {
	return evm.Time
}

func (evm *EVM) GasRate() uint64 {
	return evm.EvmGasRate
}

//StateDB
func (evm *EVM) GetStateDB() types.StateDB {
	return evm.StateDB
}

func (evm *EVM) GetOTxs() []types.BalanceRecord {
	return evm.otxs
}

func (evm *EVM) AddOtx(br types.BalanceRecord) {
	evm.otxs = append(evm.otxs, br)
}

func (evm *EVM) RefundFee() uint64 {
	var refundFee uint64
	for _, fee := range evm.refundFees {
		refundFee += fee
	}
	return refundFee
}

func (evm *EVM) RefundAllFee() uint64 {
	var refundFee uint64
	for _, fee := range evm.fees {
		refundFee += fee
	}
	return refundFee + evm.RefundFee()
}