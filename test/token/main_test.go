package main

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/state"
	erm "github.com/lianxiangcloud/linkchain/vm/runtime"
)

var (
	//testSigner = types.MakeSigner(nil)
	gasPrice = big.NewInt(1e11)
	gasLimit = uint64(1e5)
	zeroAddr = common.EmptyAddress

	Tokenaddress = common.HexToAddress("0xCCCCCCCCCC")

	tokeAmount = big.NewInt(13)
	sender     = common.HexToAddress("0x1111111111")
)

//1,Create contract with balance 10000
//2,sender add 13 tokens of Tokenaddress
//3,balancetest function will call opbalancetoken to get sender balance of Tokenaddress
//then add the balance amount to contract,balanceof contract should be 10013
//4,transfertokentest will tranfer 101 contractAddr token from contract to sender
//contractAddr token will left 10000-101, sender will own 101 contractAddr token and 13 Tokenaddress
func TestTokenEVMCall(t *testing.T) {
	var (
		state, _ = state.New(common.EmptyHash, state.NewDatabase(dbm.NewMemDB()))
		cfg      = &erm.Config{State: state,
			Origin: sender}
		method string
		input  []byte
		ret    []byte
		err    error
	)

	state.AddTokenBalance(sender, Tokenaddress, tokeAmount)
	origin := cfg.Origin

	ret, caddr, _, err := erm.Create(ccode, cfg)
	if err != nil {
		panic(err)
	}

	fmt.Printf("contract addr %v\n", caddr)

	if 0 != state.GetTokenBalance(caddr, caddr).Cmp(big.NewInt(10000)) {
		t.Error("test fail! issue opcode error!!")
	}

	method = "balanceOf"
	input, err = cabi.Pack(method, origin)
	if err != nil {
		panic(err)
	}
	ret, _, err = erm.Call(caddr, input, cfg)
	fmt.Printf("%s err(%v) ret:%s\n\n", method, err, big.NewInt(0).SetBytes(ret))

	method = "balancetest"
	input, err = cabi.Pack(method, big.NewInt(0))
	if err != nil {
		panic(err)
	}
	ret, _, err = erm.TokenCall(caddr, input, cfg, Tokenaddress)
	fmt.Printf("%s err(%v) ret:%x\n\n", method, err, ret)

	method = "balanceOf"
	input, err = cabi.Pack(method, origin)
	if err != nil {
		panic(err)
	}
	ret, _, err = erm.Call(caddr, input, cfg)
	fmt.Printf("%s err(%v) ret:%s\n\n", method, err, big.NewInt(0).SetBytes(ret))

	if 0 != big.NewInt(0).SetBytes(ret).Cmp(big.NewInt(10013)) {
		t.Error("test fail! balancetoken or tokenaddress opcode error!!")
	}

	method = "transfertokentest"
	input, err = cabi.Pack(method, big.NewInt(0))
	if err != nil {
		panic(err)
	}
	ret, _, err = erm.TokenCall(caddr, input, cfg, Tokenaddress)
	fmt.Printf("%s err(%v) ret:%x\n\n", method, err, ret)

	cToken, sToken := state.GetTokenBalance(caddr, caddr), state.GetTokenBalance(sender, caddr)

	if 0 != cToken.Cmp(big.NewInt(9899)) || 0 != sToken.Cmp(big.NewInt(101)) {
		t.Error("test fail! transfertoken opcode error!!")
	}
}
