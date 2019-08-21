package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	_ "net/http/pprof"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	db2 "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/vm/wasm"
	"github.com/xunleichain/tc-wasm/vm"
)

var (
	wasmFileFlag  = flag.String("file", "", "wasm binary file")
	fnNameFlag    = flag.String("fun", vm.APPEntry, "the name of the function we want to run")
	inputFlag     = flag.String("input", "", "input data for function")
	contractValue = flag.Int64("value", 0, "contract msg value")
	contractGas   = flag.Int64("gas", 0, "contract msg gas")
	wasmGasRate   = flag.Int64("wasm_gas_rate", 1000, "wasm gas rate,default 1000")

	ContractCandidatesAddr  = "0x0000000000000000000043616e64696461746573"
	ContractCoefficientAddr = "0x000000000000000000436f656666696369656e74"
	ContractCommitteeAddr   = "0x0000000000000000000000436f6d6d6974746565"
	ContractFoundationAddr  = "0x00000000000000000000466f756e646174696f6e"
	ContractPledgeAddr      = "0x0000000000000000000000000000506c65646765"
	ContractValidatorsAddr  = "0x0000000000000000000056616c696461746f7273"
)

func InitContract(addr common.Address, code []byte) *wasm.Contract {
	caller := MockAccountRef(common.HexToAddress("0x54fb1c7d0f011dd63b08f85ed7b518ab82028100"))
	to := MockAccountRef(addr)
	value := big.NewInt(0)
	gas := uint64(10000000000)

	contract := wasm.NewContract(caller, to, value, gas)
	contract.SetCallCode(&common.EmptyAddress, crypto.Keccak256Hash(code), code)
	contract.CreateCall = true
	return contract
}

func CallContract(st *state.StateDB, caller string, contract *wasm.Contract, input string, value *big.Int) (string, error) {
	to := MockAccountRef(contract.Address())

	gas := uint64(10000000000)
	callerA := MockAccountRef(common.HexToAddress(caller))
	newContract := wasm.NewContract(callerA, to, value, gas)
	newContract.SetCallCode(&common.EmptyAddress, crypto.Keccak256Hash(contract.Code), contract.Code)
	return CallContractByInput(st, newContract, input)
}

func CallContractByInput(st *state.StateDB, contract *wasm.Contract, input string) (string, error) {
	fmt.Println("-----------------input------------------")
	fmt.Println(input)

	testHeader := types.Header{}
	ctx := wasm.NewWASMContext(&testHeader, &MockChainContext{}, &common.EmptyAddress, (uint64)(*wasmGasRate))
	ctx.Origin = common.EmptyAddress
	ctx.GasPrice = big.NewInt(1999)

	encodeinput := hex.EncodeToString([]byte(input))
	strInput, _ := hex.DecodeString(encodeinput)
	contract.Input = strInput

	innerContract := vm.NewContract(contract.CallerAddress.Bytes(), contract.Address().Bytes(), contract.Value(), contract.Gas)
	innerContract.SetCallCode(contract.CodeAddr.Bytes(), contract.CodeHash.Bytes(), contract.Code)
	innerContract.Input = contract.Input
	innerContract.CreateCall = contract.CreateCall
	eng := vm.NewEngine(innerContract, contract.Gas, st, log.Test())
	wasm.Inject(st, wasm.NewWASM(ctx, st, nil))
	eng.SetTrace(false)
	app, err := eng.NewApp(contract.Address().String(), contract.Code, false)
	if err != nil {
		return "", fmt.Errorf("NewApp failed")
	}

	fnIndex := app.GetExportFunction(vm.APPEntry)
	if fnIndex < 0 {
		fmt.Printf("eng.GetExportFunction Not Exist: func=%s\n", "thunderchain_main")
		return "", fmt.Errorf("Function Not Exist")
	}
	app.EntryFunc = vm.APPEntry
	ret, err := eng.Run(app, contract.Input)
	if err != nil {
		fmt.Printf("eng.Run %s done: gas_used=%d, gas_left=%d\n", *fnNameFlag, eng.GasUsed(), eng.Gas())
		fmt.Printf("eng.Run fail: func=%s, index=%d, err=%s, input=%s\n", *fnNameFlag, fnIndex, err, input)
		return "", err
	}
	vmem := app.VM.VMemory()
	pBytes, err := vmem.GetString(ret)
	retStr := string(pBytes[:])
	if err != nil {
		fmt.Printf("vmem.GetString fail: err=%v", err)
		return retStr, err
	}
	fmt.Printf("eng.Run %s done: gas_used=%d, gas_left=%d, return with(%d) %s\n", *fnNameFlag, eng.GasUsed(), eng.Gas(), len(pBytes), string(pBytes))
	return retStr, nil
}

func InitState() *state.StateDB {
	db := db2.NewMemDB()
	st, _ := state.New(common.EmptyHash, state.NewDatabase(db))
	st.IntermediateRoot(false)
	return st
}

func main() {

}
