package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/vm/wasm"
	"github.com/xunleichain/tc-wasm/vm"
)

var (
	helpParams = "-file path/to/tcvm.wasm -call path/to/tcvm.params"

	testAddr1 = common.BytesToAddress(crypto.Keccak256([]byte("addr-1 for call contract"))[:20])
	testAddr2 = common.BytesToAddress(crypto.Keccak256([]byte("addr-2 for contract"))[:20])

	testTime     = big.NewInt(1565078742)
	testBalance1 = big.NewInt(987650000999999999)
	testBalance2 = big.NewInt(987650000555555555)
	testGasPrice = big.NewInt(1999)
	testGasRate  = uint64(1000)

	wasmFileFlag  = flag.String("file", "", "file with wasm bytecode")
	callFuncFlag  = flag.String("call", "", "file with called function and data")
	contractGas   = flag.Uint64("gas", 52100, "contract msg gas")
	contractValue = flag.Uint64("value", 0, "contract msg value")
)

type MockChainContext struct {
	// GetHeader returns the hash corresponding to their hash.
}

func (m *MockChainContext) GetHeader(uint64) *types.Header {
	h := types.Header{}
	return &h
}

type MockAccountRef common.Address

// Address casts AccountRef to a Address
func (ar MockAccountRef) Address() common.Address { return (common.Address)(ar) }

func main() {
	flag.Parse()

	if len(*wasmFileFlag) == 0 {
		fmt.Printf("Usage:\n    %s %s\n\n", os.Args[0], helpParams)
		fmt.Printf("Use \"%s -h\" for more information\n", os.Args[0])
		return
	}

	byteCode, err := ioutil.ReadFile(*wasmFileFlag)
	if err != nil {
		fmt.Printf("ERR read %s failed, err: %v\n", *wasmFileFlag, err)
		return
	}
	byteCode = bytes.Trim(byteCode, "\"\r\n")
	code, err := hex.DecodeString(string(byteCode[2:])) // delete 0x
	if err != nil {
		fmt.Printf("ERR hex.DecodeString failed, byteCode file %s, err: %s\n", *wasmFileFlag, err)
		return
	}
	fmt.Printf("INFO code[%d]: %s\n", len(code), string(byteCode))

	initInput := []byte("Init|{}")
	fmt.Printf("INFO initInput[%d]: 0x%s [%s]\n",
		len(initInput), hex.EncodeToString(initInput), string(initInput))

	caller := MockAccountRef(testAddr1)
	to := MockAccountRef(testAddr2)
	value := big.NewInt(0).SetUint64(*contractValue)

	contract := vm.NewContract(caller.Address().Bytes(), to.Address().Bytes(), value, *contractGas)
	contract.SetCallCode(common.EmptyAddress.Bytes(), crypto.Keccak256Hash(code).Bytes(), code)
	contract.Input = initInput
	contract.CreateCall = true

	testHeader := types.Header{}
	ctx := wasm.NewWASMContext(&testHeader, &MockChainContext{}, &common.EmptyAddress, testGasRate)
	ctx.Time = testTime
	ctx.Origin = common.EmptyAddress
	ctx.GasPrice = testGasPrice

	st, _ := state.New(common.EmptyHash, state.NewDatabase(db.NewMemDB()))
	st.AddBalance(caller.Address(), testBalance1)
	st.AddBalance(to.Address(), testBalance2)

	eng := vm.NewEngine(contract, contract.Gas, st, log.With("mod", "wasm"))
	eng.SetTrace(false)
	wasm.Inject(&ctx, st, nil)

	start := time.Now()

	app, err := eng.NewApp(contract.Address().String(), contract.Code, false)
	if err != nil {
		fmt.Printf("ERR vm/Engine.NewApp failed, err: %s\n", err)
		return
	}

	parseTime := time.Since(start).Seconds()

	fnIndex := app.GetExportFunction(vm.APPEntry)
	if fnIndex < 0 {
		fmt.Printf("INFO vm/APP.GetExportFunction, func=%s not exist\n", vm.APPEntry)
		return
	}
	app.EntryFunc = vm.APPEntry

	ret, err := eng.Run(app, contract.Input)
	if err != nil {
		fmt.Printf("ERR init vm/Engine.Run failed, func=%s gasUsed=%d gasLeft=%d, err: %s",
			vm.APPEntry, eng.GasUsed(), eng.Gas(), err)
		return
	}

	vmem := app.VM.VMemory()
	rBytes, err := vmem.GetString(ret)
	if err != nil {
		fmt.Printf("ERR init vm/MemManager.GetBytes failed, err: %v", err)
		return
	}

	initTime := time.Since(start).Seconds()

	fmt.Printf("INFO init done, gasUsed=%d gasLeft=%d time=[%f:%f], return[%d]: %s\n",
		eng.GasUsed(), eng.Gas(), parseTime, initTime-parseTime, len(rBytes), string(rBytes))

	if len(*callFuncFlag) == 0 {
		fmt.Println("INFO init finished. You can provide the called function and data via parameter call")
		return
	}

	callInput, err := ioutil.ReadFile(*callFuncFlag)
	if err != nil {
		if !strings.Contains(*callFuncFlag, "|{") {
			fmt.Printf("ERR read %s failed, err: %s\n", *callFuncFlag, err)
			return
		}
		callInput = []byte(*callFuncFlag)
	}
	callInput = bytes.Trim(callInput, "\r\n")
	fmt.Printf("INFO callInput[%d]: 0x%s [%s]\n",
		len(callInput), hex.EncodeToString(callInput), string(callInput))

	contract.Input = callInput
	contract.CreateCall = false

	start = time.Now()

	ret, err = eng.Run(app, contract.Input)
	if err != nil {
		fmt.Printf("ERR call vm/Engine.Run failed, func=%s gasUsed=%d gasLeft=%d, err: %s",
			vm.APPEntry, eng.GasUsed(), eng.Gas(), err)
		return
	}

	vmem = app.VM.VMemory()
	rBytes, err = vmem.GetString(ret)
	if err != nil {
		fmt.Printf("ERR call vm/MemManager.GetBytes failed, err: %v", err)
		return
	}

	callTime := time.Since(start).Seconds()

	fmt.Printf("INFO call done, gasUsed=%d gasLeft=%d time=[%f], return[%d]: %s\n",
		eng.GasUsed(), eng.Gas(), callTime, len(rBytes), string(rBytes))

	return
}
