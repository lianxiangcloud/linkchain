package app

import (
	"math/big"

	common "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/state"
	types "github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/vm"
)

const (
	// Inputs
	Ain = "ain"
	Uin = "uin"
	// Outputs
	Aout      = "aout"
	Uout      = "uout"
	Cout      = "cout"
	Createout = "createout"
	Updateout = "updateout"
)

type txInput struct {
	From  common.Address
	Value *big.Int
	Nonce uint64
	Type  string
}

type txOutput struct {
	To     common.Address
	Amount *big.Int
	Data   []byte
	Type   string
}

type processTransaction struct {
	// generic
	Type         string
	Kind         types.UTXOKind
	Inputs       []txInput
	Outputs      []txOutput
	TokenAddress common.Address
	// gas related
	Gas        uint64
	GasPrice   *big.Int
	InitialGas uint64
	RefundAddr common.Address // choose the signer if has any, otherwise emptyAddress
	// enviroment related
	State *state.StateDB
	Vmenv *vm.VmFactory
	// Miscellaneous
	Hash common.Hash
}

type TransitionResult struct {
	Rets        [][]byte
	Gas         uint64
	ByteCodeGas uint64
	Fee         *big.Int
	Addrs       []common.Address
	Otxs        []types.BalanceRecord
}
