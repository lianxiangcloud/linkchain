package types

import (
	"math/big"
	"testing"
)

func TestCalNewFee(t *testing.T) {
	type gasTest struct {
		val    *big.Int
		gasFee uint64
	}

	lianke := new(big.Int).Mul(big.NewInt(GasPrice), big.NewInt(gasToLiankeRate))

	// test msg
	gasTestMsg := []gasTest{
		{big.NewInt(0), MinGasLimit},
		{big.NewInt(10), MinGasLimit},
		{lianke, MinGasLimit},
		{new(big.Int).Mul(big.NewInt(10), lianke), MinGasLimit},
		{new(big.Int).Mul(big.NewInt(100), lianke), 5e6},
		{new(big.Int).Mul(big.NewInt(1e10), lianke), MaxGasLimit},
		{new(big.Int).Mul(big.NewInt(1e11), lianke), MaxGasLimit},
	}

	// check
	for _, v := range gasTestMsg {
		calFee := CalNewAmountGas(v.val)
		if v.gasFee != calFee {
			t.Fatal("CalNewAmountGas failed.", "val", v.val, "gasFee", v.gasFee, "calFee", calFee)
		}
	}
}
