package types

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/lianxiangcloud/linkchain/accounts/abi"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/math"
)

// const variables
var (
	cabi     abi.ABI // get decimals abi for evm
	evmData  []byte  // get decimals data for evm
	wasmData []byte  // GetDecimals() data for wasm
)

var (
	utxoDecimalsDigits        = uint8(8)
	maxCommitChangeRateDigits = uint8(18)
	utxoCommitChangeRateBase  = int64(10)
)

func init() {
	var err error
	cabi, err = abi.JSON(strings.NewReader(`[{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}]`))
	if err != nil {
		log.Error("UTXOChangeRateGetter init cabi err", "err", err)
	}
	evmData, err = cabi.Pack("decimals")
	if err != nil {
		log.Error("UTXOChangeRateGetter pack cabi err", "err", err)
	}

	wasmData = hexutil.Bytes("GetDecimals|{}")
}

// UTXOChangeRateGetter provides getter for utxo commit change rate which is used in processing utxo tx
type UTXOChangeRateGetter struct {
	f     func(addr common.Address) (int64, error)
	cache sync.Map
}

// NewUTXOChangeRateGetter create a UTXOChangeRateGetter object
func NewUTXOChangeRateGetter(f func(addr common.Address) (int64, error)) *UTXOChangeRateGetter {
	return &UTXOChangeRateGetter{
		f:     f,
		cache: sync.Map{},
	}
}

// GetRate get utxo change rate by calling provided function.
// rates will be cached
func (m *UTXOChangeRateGetter) GetRate(addr common.Address) (rate int64, err error) {
	if common.IsLKC(addr) {
		return UTXO_COMMITMENT_CHANGE_RATE, nil
	}
	rateI, ok := m.cache.Load(addr)
	if !ok {
		rate, err = m.f(addr)
		if err != nil {
			log.Error("UTXOChangeRateGetter GetRate err", "err", err, "add", addr)
			return
		}
		m.cache.Store(addr, rate)
	} else {
		rate, ok = rateI.(int64)
		if !ok {
			log.Error("UTXOChangeRateGetter Load err", "add", addr, "rateI", rateI)
			return
		}
	}
	return
}

// UTXOChangeRateDataEVM returns data in getting rate from evm contract
func UTXOChangeRateDataEVM() []byte {
	return evmData
}

// UTXOChangeRateResultDecodeEVM decodes returned decimals from evm contract
func UTXOChangeRateResultDecodeEVM(data []byte) (uint8, error) {
	var rate uint8
	err := cabi.Unpack(&rate, "decimals", data)
	if err != nil {
		log.Error("UTXOChangeRateResultDecodeEVM unpack err", "err", err)
		return 0, err
	}
	return rate, nil
}

// UTXOChangeRateDataWASM returns data in getting rate from wasm contract
func UTXOChangeRateDataWASM() []byte {
	return wasmData
}

// UTXOChangeRateResultDecodeWASM decodes returned decimals from wasm contract
func UTXOChangeRateResultDecodeWASM(data []byte) (uint8, error) {
	r := struct {
		Rate string `json:"ret"`
	}{}
	err := json.Unmarshal(data, &r)
	if err != nil {
		log.Error("UTXOChangeRateResultDecodeWASM unmarshal err", "err", err)
		return 0, err
	}

	rate, ok := big.NewInt(0).SetString(r.Rate, 10)
	if !ok {
		log.Error("UTXOChangeRateResultDecodeWASM setstring err")
		return 0, fmt.Errorf("UTXOChangeRateResultDecodeWASM setstring err")
	}

	if rate.Sign() < 0 {
		log.Error("UTXOChangeRateResultDecodeWASM rate negative")
		return 0, fmt.Errorf("UTXOChangeRateResultDecodeWASM rate negative")
	}
	return uint8(rate.Uint64()), nil
}

// UTXOChangeRateFromUint8 calculates UTXO change rate from contract decimal value
// raw > 26			=> err
// 8 <= raw <= 26	=> 10exp(raw - 8)
// raw < 8			=> 10exp0=1
func UTXOChangeRateFromUint8(raw uint8) (int64, error) {
	if raw > maxCommitChangeRateDigits+utxoDecimalsDigits {
		return 0, ErrUTXOChangeRateTooLarge
	} else if raw < utxoDecimalsDigits {
		raw = 0
	} else {
		raw -= utxoDecimalsDigits
	}

	rate := math.BigPow(utxoCommitChangeRateBase, int64(raw)).Int64()
	return rate, nil
}
