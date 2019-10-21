package vm

import (
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/vm/evm"
	"github.com/lianxiangcloud/linkchain/vm/wasm"
)

type VmInterface interface {
	Reset(types.Message)
	Cancel()
	UTXOCall(caller types.ContractRef, addr, token common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error)
	Call(caller types.ContractRef, addr, token common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error)
	CallCode(caller types.ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error)
	DelegateCall(caller types.ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error)
	StaticCall(caller types.ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, byteCodeGas uint64, err error)
	Create(caller types.ContractRef, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error)
	Upgrade(caller types.ContractRef, addr common.Address, code []byte)
	// Interpreter() types.Interpreter
	SetToken(addr common.Address)
	GetCoinbase() common.Address
	GetBlockNumber() *big.Int
	GetTime() *big.Int
	GasRate() uint64
	GetStateDB() types.StateDB
	GetOTxs() []types.BalanceRecord
	AddOtx(br types.BalanceRecord)
	RefundFee() uint64
	RefundAllFee() uint64
}

type VmFactory struct {
	evm  *evm.EVM
	wasm *wasm.WASM
}

func NewVM() VmFactory {
	return VmFactory{}
}

func NewVMwithInstance(vm VmInterface) *VmFactory {
	vmenv := VmFactory{}
	switch realvm := vm.(type) {
	case *evm.EVM:
		vmenv.evm = realvm
	case *wasm.WASM:
		vmenv.wasm = realvm
	default:
		log.Error("VmFactory.NewVMwithInstance", "context", "unknown type")
	}
	return &vmenv
}

func (v *VmFactory) AddVm(context types.Context, statedb types.StateDB, cfg types.VmConfig) {
	switch ctx := context.(type) {
	case *evm.Context:
		log.Debug("VmFactory.AddVm", "Context", "NewEVM")
		v.evm = evm.NewEVM(*ctx, statedb, cfg)
	case *wasm.Context:
		log.Debug("VmFactory.AddVm", "Context", "NewWASM")
		v.wasm = wasm.NewWASM(*ctx, statedb, cfg)
	default:
		log.Error("VmFactory.AddVm", "context", "unknown type")
	}
}

func (v *VmFactory) isWasmCode(code []byte) bool {
	return wasm.IsWasmContract(code)
}

func (v *VmFactory) GetRealVm(code []byte, toPtr *common.Address) VmInterface {
	storageCode := code
	if v.isWasmCode(storageCode) {
		log.Debug("GetRealVm", "VmInterface", "wasm")
		return v.wasm
	}

	log.Debug("GetRealVm", "VmInterface", "evm")
	return v.evm
}

func (v *VmFactory) GetEvm() VmInterface {
	return v.evm
}

func (v *VmFactory) GetWasm() VmInterface {
	return v.wasm
}
