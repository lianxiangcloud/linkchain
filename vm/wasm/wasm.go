package wasm

import (
	"encoding/hex"
	"fmt"
	"math/big"

	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/xunleichain/tc-wasm/vm"
)

var (
	defaultDifficulty        = big.NewInt(10000000)
	wasmIDLength             = 4
	wasmID            uint32 = 0x6d736100
)

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

	UnsafeTransferFunc func(types.StateDB, common.Address, common.Address, *big.Int)
)

// Context provides the EVM with auxiliary information. Once provided
// it shouldn't be modified.
type Context struct {
	// CanTransfer returns whether the account contains
	// sufficient ether to transfer the value
	CanTransfer CanTransferFunc
	// Transfer transfers ether from one account to the other
	Transfer TransferFunc
	// GetHash returns the hash corresponding to n
	UnsafeTransfer UnsafeTransferFunc
	GetHash        GetHashFunc

	// Message information
	Token    common.Address // Provides the tx token type
	Origin   common.Address // Provides information for ORIGIN
	GasPrice *big.Int       // Provides information for GASPRICE

	// Block information
	Coinbase    common.Address // Provides information for COINBASE
	GasLimit    uint64         // Provides information for GASLIMIT
	BlockNumber *big.Int       // Provides information for NUMBER
	Time        *big.Int       // Provides information for TIME
	Difficulty  *big.Int       // Provides information for DIFFICULTY

	WasmGasRate uint64
}

// NewWASMContext creates a new context for use in the WASM.
func NewWASMContext(header *types.Header, chain ChainContext, author *common.Address, gasRate uint64) Context {
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
		WasmGasRate:    gasRate,
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

// UnsafeTransfer subtracts amount from sender and adds amount to recipient using the given Db
func UnsafeTransfer(db types.StateDB, recipient, token common.Address, amount *big.Int) {
	db.AddTokenBalance(recipient, token, amount)
}

// run runs the given contract and takes care of running precompiles with a fallback to the byte code interpreter.
func run(wasm *WASM, c types.Contract, input []byte) ([]byte, uint64, error) {
	contract := c.(*Contract)
	localMaxGas := new(big.Int).Mul(new(big.Int).SetUint64(contract.Gas), new(big.Int).SetUint64(wasm.WasmGasRate)).Uint64()

	innerContract := vm.NewContract(contract.CallerAddress.Bytes(), contract.Address().Bytes(), contract.Value(), contract.Gas)
	innerContract.SetCallCode(contract.CodeAddr.Bytes(), contract.CodeHash.Bytes(), contract.Code)
	innerContract.Input = contract.Input
	innerContract.CreateCall = contract.CreateCall
	eng := vm.NewEngine(innerContract, localMaxGas, wasm.StateDB, log.New("mod", "wasm"))
	eng.Ctx = wasm
	eng.SetTrace(false)
	addr := contract.CodeAddr
	app, err := eng.NewApp(addr.String(), nil, false)
	if err != nil {
		if err == vm.ErrContractNoCode {
			return nil, contract.Gas, nil
		}
		log.Error("WASM eng.NewApp", "err", err, "contract", addr.String())
		return nil, contract.Gas, fmt.Errorf("WASM eng.NewApp,err:%v", err)
	}

	fnIndex := app.GetExportFunction(vm.APPEntry)
	if fnIndex < 0 {
		return []byte(""), contract.Gas, fmt.Errorf("GetExportFunction(APPEntry) fail")
	}

	ret, err := eng.Run(app, input)
	gasused, modgas := new(big.Int).DivMod(new(big.Int).SetUint64(eng.GasUsed()), new(big.Int).SetUint64(wasm.WasmGasRate), big.NewInt(0))
	subModGas := uint64(0)
	if modgas.Uint64() > 0 {
		subModGas = uint64(1)
	}
	log.Debug("wasm add refundFee", "wasm refundFee", wasm.refundFee, "eng fee", eng.GetFee())
	wasm.refundFee += eng.GetFee()

	gas := contract.Gas - gasused.Uint64() - subModGas

	if err != nil {
		log.Error("WASM eng.Run ret:", "ret", ret, "gas", gas, "err", err)
		return nil, gas, err
	}

	// @Todo: Bugs here.
	retData, err := app.VM.VMemory().GetString(ret)
	log.Debug("WASM eng.Run ret:", "ret", ret, "retData", string(retData), "gas", gas, "eng.GasUsed()", eng.GasUsed(), "err", err)
	if err != nil {
		return nil, gas, err
	}

	return []byte(retData), gas, err
}

type Config struct {
}

type WASM struct {
	// Context provides auxiliary blockchain related information
	Context
	// StateDB gives access to the underlying state
	StateDB types.StateDB

	env *vm.EnvTable
	eng *vm.Engine
	app *vm.APP

	otxs      []types.BalanceRecord
	refundFee uint64
}

// NewWASM returns a new WASM. The returned WASM is not thread safe and should
// only ever be used *once*.
func NewWASM(c types.Context, statedb types.StateDB, vmc types.VmConfig) *WASM {
	ctx := c.(Context)

	return &WASM{
		Context:  ctx,
		StateDB:  statedb,
		otxs:     make([]types.BalanceRecord, 0),
	}
}

// reset
//func (wasm *WASM) Reset(origin common.Address, gasPrice *big.Int, nonce uint64) {
func (wasm *WASM) Reset(msg types.Message) {
	wasm.Context.Origin = msg.MsgFrom()                      //origin
	wasm.Context.GasPrice = new(big.Int).Set(msg.GasPrice()) //gasPrice
	wasm.Context.Token = common.EmptyAddress
	wasm.otxs = make([]types.BalanceRecord, 0)
}

func (wasm *WASM) GetCode(bz []byte) []byte {
	return wasm.StateDB.GetCode(common.BytesToAddress(bz))
}

// Cancel cancels any running WASM operation. This may be called concurrently and
// it's safe to be called multiple times.
func (wasm *WASM) Cancel() {

}

func (wasm *WASM) UTXOCall(c types.ContractRef, addr, token common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error) {
	var (
		to       = AccountRef(addr)
		snapshot = wasm.StateDB.Snapshot()
	)
	if !wasm.StateDB.Exist(addr) {
		wasm.StateDB.CreateAccount(addr)
	}
	wasm.UnsafeTransfer(wasm.StateDB, to.Address(), token, value)

	// Initialise a new contract and set the code that is to be used by the WASM.
	// The contract is a scoped environment for this execution context only.
	caller := c.(ContractRef)
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, wasm.StateDB.GetCodeHash(addr), wasm.StateDB.GetCode(addr))
	contract.Input = input

	ret, leftOverGas, err = run(wasm, contract, input)
	if err == nil {
		wasm.refundFee = 0
	}
	contract.Gas = leftOverGas
	// When an error was returned by the WASM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
	gasUsed := gas - contract.Gas
	if gasUsed < contract.ByteCodeGas {
		byteCodeGas = contract.ByteCodeGas - gasUsed
	}
	if err != nil {
		wasm.StateDB.RevertToSnapshot(snapshot)
		if err != vm.ErrExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, byteCodeGas, err
}

// Call executes the contract associated with the addr with the given input as
// parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
func (wasm *WASM) Call(c types.ContractRef, addr, token common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error) {
	caller := c.(ContractRef)

	// Fail if we're trying to transfer more than the available balance
	if !wasm.Context.CanTransfer(wasm.StateDB, caller.Address(), token, value) {
		return nil, gas, 0, vm.ErrInsufficientBalance
	}

	var (
		to       = AccountRef(addr)
		snapshot = wasm.StateDB.Snapshot()
	)
	if !wasm.StateDB.Exist(addr) {
		wasm.StateDB.CreateAccount(addr)
	}
	wasm.Transfer(wasm.StateDB, caller.Address(), to.Address(), token, value)

	br := types.GenBalanceRecord(caller.Address(), to.Address(), types.AccountAddress, types.AccountAddress, types.TxTransfer, token, value)
	wasm.otxs = append(wasm.otxs, br)

	// Initialise a new contract and set the code that is to be used by the WASM.
	// The contract is a scoped environment for this execution context only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, wasm.StateDB.GetCodeHash(addr), wasm.StateDB.GetCode(addr))
	contract.Input = input

	ret, leftOverGas, err = run(wasm, contract, input)
	if err == nil {
		wasm.refundFee = 0
	}
	contract.Gas = leftOverGas
	// When an error was returned by the WASM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
	gasUsed := gas - contract.Gas
	if gasUsed < contract.ByteCodeGas {
		byteCodeGas = contract.ByteCodeGas - gasUsed
	}
	if err != nil {
		wasm.StateDB.RevertToSnapshot(snapshot)
		if err != vm.ErrExecutionReverted {
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
func (wasm *WASM) CallCode(c types.ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error) {
	caller := c.(ContractRef)

	// Fail if we're trying to transfer more than the available balance
	if !wasm.CanTransfer(wasm.StateDB, caller.Address(), common.EmptyAddress, value) {
		return nil, gas, 0, vm.ErrInsufficientBalance
	}

	var (
		snapshot = wasm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)
	// initialise a new contract and set the code that is to be used by the
	// WASM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, wasm.StateDB.GetCodeHash(addr), wasm.StateDB.GetCode(addr))
	contract.Input = input

	ret, leftOverGas, err = run(wasm, contract, input)
	contract.Gas = leftOverGas
	gasUsed := gas - contract.Gas
	if gasUsed < contract.ByteCodeGas {
		byteCodeGas = contract.ByteCodeGas - gasUsed
	}
	if err != nil {
		wasm.StateDB.RevertToSnapshot(snapshot)
		if err != vm.ErrExecutionReverted {
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
func (wasm *WASM) DelegateCall(c types.ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error) {
	caller := c.(ContractRef)

	var (
		snapshot = wasm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)

	// Initialise a new contract and make initialise the delegate values
	contract := NewContract(caller, to, nil, gas).AsDelegate()
	contract.SetCallCode(&addr, wasm.StateDB.GetCodeHash(addr), wasm.StateDB.GetCode(addr))
	contract.Input = input

	ret, leftOverGas, err = run(wasm, contract, input)
	contract.Gas = leftOverGas
	gasUsed := gas - contract.Gas
	if gasUsed < contract.ByteCodeGas {
		byteCodeGas = contract.ByteCodeGas - gasUsed
	}
	if err != nil {
		wasm.StateDB.RevertToSnapshot(snapshot)
		if err != vm.ErrExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, byteCodeGas, err
}

// StaticCall executes the contract associated with the addr with the given input
// as parameters while disallowing any modifications to the state during the call.
// Opcodes that attempt to perform such modifications will result in exceptions
// instead of performing the modifications.
func (wasm *WASM) StaticCall(c types.ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error) {
	caller := c.(ContractRef)
	// Make sure the readonly is only set if we aren't in readonly yet
	// this makes also sure that the readonly flag isn't removed for
	// child calls.
	// if !wasm.interpreter.readOnly {
	// 	wasm.interpreter.readOnly = true
	// 	defer func() { wasm.interpreter.readOnly = false }()
	// }

	var (
		to       = AccountRef(addr)
		snapshot = wasm.StateDB.Snapshot()
	)
	// Initialise a new contract and set the code that is to be used by the
	// WASM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, to, new(big.Int), gas)
	contract.SetCallCode(&addr, wasm.StateDB.GetCodeHash(addr), wasm.StateDB.GetCode(addr))
	contract.Input = input

	// When an error was returned by the WASM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in Homestead this also counts for code storage gas errors.
	ret, leftOverGas, err = run(wasm, contract, input)
	contract.Gas = leftOverGas
	gasUsed := gas - contract.Gas
	if gasUsed < contract.ByteCodeGas {
		byteCodeGas = contract.ByteCodeGas - gasUsed
	}
	if err != nil {
		wasm.StateDB.RevertToSnapshot(snapshot)
		if err != vm.ErrExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, byteCodeGas, err
}

// Create creates a new contract using code as deployment code.
func (wasm *WASM) Create(c types.ContractRef, data []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	caller := c.(ContractRef)

	// parse Constructor's arguments && bytecode
	input, code, err := vm.ParseInitArgsAndCode(data)
	if err != nil {
		log.Warn("WASM Create: parse InitArgs Length fail", "err", err)
		return nil, common.EmptyAddress, gas, fmt.Errorf("Invalid InitArgs Length for Contract Init Function")
	}

	contractAddr = crypto.CreateAddress(caller.Address(), wasm.StateDB.GetNonce(caller.Address()), code)

	// Ensure there's no existing contract already at the designated address
	contractHash := wasm.StateDB.GetCodeHash(contractAddr)
	if wasm.StateDB.GetNonce(contractAddr) != 0 || (contractHash != common.EmptyHash && contractHash != emptyCodeHash) {
		return nil, common.EmptyAddress, 0, vm.ErrContractAddressCollision
	}
	// Create a new account on the state
	snapshot := wasm.StateDB.Snapshot()
	wasm.StateDB.CreateAccount(contractAddr)
	wasm.StateDB.SetNonce(contractAddr, 1)
	wasm.StateDB.SetCode(contractAddr, code)

	encodeinput := hex.EncodeToString(input)
	strInput, _ := hex.DecodeString(encodeinput)

	wasm.UnsafeTransfer(wasm.StateDB, contractAddr, common.EmptyAddress, value)

	// initialise a new contract and set the code that is to be used by the
	// WASM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, AccountRef(contractAddr), value, gas)
	contract.SetCallCode(&contractAddr, crypto.Keccak256Hash(code), code)
	contract.Input = []byte(strInput)
	contract.CreateCall = true

	// TODO :wasm not found code ,return err,create fail,
	ret, leftOverGas, err = run(wasm, contract, contract.Input)
	if err == nil {
		wasm.refundFee = 0
	}
	ret = code
	contract.Gas = leftOverGas

	// check whether the max code size has been exceeded
	maxCodeSizeExceeded := len(ret) > vm.MaxCodeSize
	// if the contract creation ran successfully and no errors were returned
	// calculate the gas required to store the code. If the code could not
	// be stored due to not enough gas set an error and let it be handled
	// by the error checking condition below.
	if err == nil && !maxCodeSizeExceeded {
		createDataGas := uint64(len(ret)) * cfg.CreateDataGas / wasm.WasmGasRate
		contract.Gas = leftOverGas
		if contract.UseGas(createDataGas) {
			wasm.StateDB.SetCode(contractAddr, ret)
		} else {
			err = vm.ErrCodeStoreOutOfGas
		}
	}

	// When an error was returned by the WASM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
	if maxCodeSizeExceeded || err != nil {
		wasm.StateDB.RevertToSnapshot(snapshot)
		if err != vm.ErrExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	// Assign err if contract code size exceeds the max while the err is still empty.
	if maxCodeSizeExceeded && err == nil {
		err = vm.ErrMaxCodeSizeExceeded
	}
	return ret, contractAddr, contract.Gas, err
}

// Interpreter returns the WASM interpreter
// func (wasm *WASM) Interpreter() *Interpreter { return wasm.interpreter }

func (wasm *WASM) Upgrade(caller types.ContractRef, contractAddr common.Address, code []byte) {
	wasm.StateDB.SetCode(contractAddr, code)
	vm.AppCache.Delete(contractAddr.String())
}

//Token
func (wasm *WASM) SetToken(token common.Address) {
	wasm.Token = token
}

// Coinbase
func (wasm *WASM) GetCoinbase() common.Address {
	return wasm.Coinbase
}

func (wasm *WASM) GetBlockNumber() *big.Int {
	return wasm.BlockNumber
}

//Time
func (wasm *WASM) GetTime() *big.Int {
	return wasm.Time
}

func (wasm *WASM) GasRate() uint64 {
	return wasm.WasmGasRate
}

//StateDB
func (wasm *WASM) GetStateDB() types.StateDB {
	return wasm.StateDB
}

func (wasm *WASM) GetOTxs() []types.BalanceRecord {
	return wasm.otxs
}

func (wasm *WASM) AddOtx(br types.BalanceRecord) {
	wasm.otxs = append(wasm.otxs, br)
}

// IsWasmContract check contract's id
func IsWasmContract(code []byte) bool {
	if len(code) > wasmIDLength {
		if wasmID == (uint32(code[0]) | uint32(code[1])<<8 | uint32(code[2])<<16 | uint32(code[3])<<24) {
			return true
		}
	}
	return false
}

func (wasm *WASM) RefundAllFee() uint64 {
	return wasm.refundFee
}

func (wasm *WASM) RefundFee() uint64 {
	return wasm.RefundAllFee()
}
