package types

import (
	"errors"
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/hexutil"
)

/*
	if (x % (1e+18) != 0) {
		fee = (x/(1e+18)+1) * rate *(1e+13)
	} else {
		fee = (x/(1e+18)) * rate *(1e+13)
	}
	if fee < min {
		fee = min
	}
	if max != 0 && fee > max {
		fee = max
	}
*/

const (
	MaxGasLimit  int64 = 5e9 // max gas limit
	MinGasLimit  int64 = 5e5 // min gas limit
	MaxFeeCounts int64 = 1024000

	EverLiankeFee         int64 = 5e4  // ever poundage fee unit(gas)
	EverContractLiankeFee int64 = 25e3 // ever poundate contract fee uint(gas)

	gasToLiankeRate int64 = 1e7 // lianke = 1e+7 gas
	GasPrice        int64 = 1e11
)

type SweepGas struct {
	Balance *hexutil.Big `json:"balance"    gencodec:"required"`
	Amount  *hexutil.Big `json:"amount"    gencodec:"required"`
	Change  *hexutil.Big `json:"change"    gencodec:"required"`
	Gas     uint64       `json:"gas"    gencodec:"required"`
}

func CalNewAmountGas(value *big.Int, feeRule int64) uint64 {
	var liankeCount *big.Int
	lianke := new(big.Int).Mul(big.NewInt(GasPrice), big.NewInt(gasToLiankeRate))
	if new(big.Int).Mod(value, lianke).Uint64() != 0 {
		liankeCount = new(big.Int).Div(value, lianke)
		liankeCount.Add(liankeCount, big.NewInt(1))
	} else {
		liankeCount = new(big.Int).Div(value, lianke)
	}
	calFeeGas := new(big.Int).Mul(big.NewInt(feeRule), liankeCount)

	if calFeeGas.Cmp(big.NewInt(MinGasLimit)) < 0 {
		calFeeGas.Set(big.NewInt(MinGasLimit))
	}
	if MaxGasLimit != 0 && calFeeGas.Cmp(big.NewInt(MaxGasLimit)) > 0 {
		calFeeGas.Set(big.NewInt(MaxGasLimit))
	}
	return calFeeGas.Uint64()
}

func calAllFee(balance *big.Int, amount *big.Int, min *big.Int, max *big.Int) (am *big.Int, changeAm *big.Int, fee uint64) {
	fee = CalNewAmountGas(amount, EverLiankeFee)

	gasUsed := new(big.Int).Mul(big.NewInt(GasPrice), new(big.Int).SetUint64(fee))
	newBalance := new(big.Int).Add(amount, gasUsed)
	changeAmount := new(big.Int).Sub(balance, newBalance)
	if max.Cmp(min) <= 0 || amount.Cmp(min) == 0 {
		am = amount
		changeAm = changeAmount
		return
	}

	if changeAmount.Sign() == 0 {
		am = amount
		changeAm = changeAmount
		return
	} else if changeAmount.Sign() > 0 {
		// amount is less
		min = new(big.Int).Set(amount)
		fix := new(big.Int).Add(amount, max)
		amount = new(big.Int).Div(fix, big.NewInt(2))

		return calAllFee(balance, amount, min, max)
	} else {
		// amount is more
		max = new(big.Int).Set(amount)
		fix := new(big.Int).Add(amount, min)
		amount = new(big.Int).Div(fix, big.NewInt(2))

		return calAllFee(balance, amount, min, max)
	}
}

func CalSweepBalanceFee(balance *big.Int) (amount *big.Int, changeAmount *big.Int, fee uint64, err error) {
	minGasUsed := new(big.Int).Mul(big.NewInt(GasPrice), big.NewInt(MinGasLimit))
	if balance.Cmp(minGasUsed) <= 0 {
		return big.NewInt(0), big.NewInt(0), 0, errors.New("balance too low")
	}

	amount, changeAmount, fee = calAllFee(balance, new(big.Int).Set(balance), big.NewInt(0), new(big.Int).Set(balance))
	return
}
