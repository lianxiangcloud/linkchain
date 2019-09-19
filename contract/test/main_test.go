package main

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"strconv"
	"testing"

	"github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/vm/wasm"
)

var contractAddr = map[string]common.Address{
	"candidates":  config.ContractCandidatesAddr,
	"coefficient": config.ContractCoefficientAddr,
	"committee":   config.ContractCommitteeAddr,
	"foundation":  config.ContractFoundationAddr,
	"pledge":      config.ContractPledgeAddr,
	"validators":  config.ContractValidatorsAddr,
}

var OneLink = new(big.Int)
var InitAmount = new(big.Int)
var WinoutAmount = new(big.Int)

var testAddr0 = "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100"
var foundCall = "0x0000000000000000000000000000000000000000"

func TestMain(m *testing.M) {
	OneLink.SetString("1000000000000000000", 10) // octal
	InitAmount.SetInt64(500000)
	WinoutAmount.SetInt64(5000000)
	InitAmount.Mul(InitAmount, OneLink) // octal
	WinoutAmount.Mul(WinoutAmount, OneLink)

	fmt.Println(WinoutAmount.String())
	fmt.Println("beg" + InitAmount.String() + "end")
	m.Run()
}

func RegiesterCommittee(st *state.StateDB) *wasm.Contract {
	inputs := []string{
		"init|{}",
	}
	contract := Regiester("committee")
	for _, input := range inputs {
		_, err := CallContractByInput(st, contract, input)
		if err != nil {
			return nil
		}
	}
	return contract
}

func Regiester(contractName string) *wasm.Contract {
	//get code
	code, err := ioutil.ReadFile("../v1/" + contractName + "/output.wasm")
	if err != nil {
		fmt.Printf("read %s fail: %s\n", *wasmFileFlag, err)
		return nil
	}
	contract := InitContract(contractAddr[contractName], code)
	return contract
}

func TestCandidatesNormal(t *testing.T) {
	st := InitState()

	candidatesContract := Regiester("candidates")
	coefficientContract := Regiester("coefficient")
	committeeContract := Regiester("committee")
	foundationContract := Regiester("foundation")
	pledgeContract := Regiester("pledge")
	validatorsContract := Regiester("validators")

	initInputs := "init|{}"
	CallContractByInput(st, candidatesContract, initInputs)
	CallContractByInput(st, coefficientContract, initInputs)
	CallContractByInput(st, committeeContract, initInputs)
	CallContractByInput(st, foundationContract, initInputs)
	CallContractByInput(st, pledgeContract, initInputs)
	CallContractByInput(st, validatorsContract, initInputs)

	candidateCoinbase := []string{
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028110",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028111",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028112",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028113",
	}

	var orderId uint64 = 0
	for _, coinbase := range candidateCoinbase {
		pledgeInit(st, coinbase, orderId, t)
		orderId += 1
	}
	inputs := []string{
		"init|{}",
		`SetCandidate|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac70000","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028110","voting_power":10,"score":100,"punish_height":0}}`,
		`SetCandidate|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac71111","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028111","voting_power":0,"score":300,"punish_height":0}}`,
		`SetCandidate|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac72222","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028112","voting_power":10,"score":300,"punish_height":0}}`,
		`SetCandidate|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac73333","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028113","voting_power":10,"score":300,"punish_height":0}}`,
		`GetAllCandidates|{}`,
		`DeleteCandidate|{"0":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac71111"}`,
		`GetAllCandidates|{}`,
	}
	contract := Regiester("candidates")
	for _, input := range inputs {
		_, err := CallContractByInput(st, contract, input)
		if err != nil {
			t.Fail()
		}
	}
}

func TestCandidatesCoinbaseRepeat(t *testing.T) {
	st := InitState()
	RegiesterCommittee(st)

	pledgeContract := Regiester("pledge")
	initInputs := "init|{}"
	CallContractByInput(st, pledgeContract, initInputs)
	candidateCoinbase := []string{
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028110",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028111",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028112",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028113",
	}

	var orderId uint64 = 0
	for _, coinbase := range candidateCoinbase {
		pledgeInit(st, coinbase, orderId, t)
		orderId += 1
	}

	inputs := []string{
		"init|{}",
		`SetCandidate|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac70000","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028110","voting_power":10,"score":100,"punish_height":0}}`,
	}
	contract := Regiester("candidates")
	for _, input := range inputs {
		_, err := CallContractByInput(st, contract, input)
		if err != nil {
			t.Fail()
		}
	}

	_, err := CallContractByInput(st, contract, `SetCandidate|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac73333","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028110","voting_power":10,"score":300,"punish_height":0}}`)
	if err == nil {
		t.Fail()
	}
}

func TestCandidatesDelEmptyPubkey(t *testing.T) {
	st := InitState()
	RegiesterCommittee(st)

	pledgeContract := Regiester("pledge")
	initInputs := "init|{}"
	CallContractByInput(st, pledgeContract, initInputs)
	candidateCoinbase := []string{
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028110",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028111",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028112",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028113",
	}

	var orderId uint64 = 0
	for _, coinbase := range candidateCoinbase {
		pledgeInit(st, coinbase, orderId, t)
		orderId += 1
	}
	inputs := []string{
		"init|{}",
		`SetCandidate|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac70000","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028110","voting_power":10,"score":100,"punish_height":0}}`,
	}
	contract := Regiester("candidates")
	for _, input := range inputs {
		_, err := CallContractByInput(st, contract, input)
		if err != nil {
			t.Fail()
		}
	}

	_, err := CallContractByInput(st, contract,
		`DeleteCandidate|{"0":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac71111"}`)
	if err == nil {
		t.Fail()
	}
}

func TestValidators(t *testing.T) {
	st := InitState()
	RegiesterCommittee(st)
	inputs := []string{
		"init|{}",
		`SetValidator|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac71111","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028100","voting_power":10,"score":100}}`,
		`SetValidator|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac72222","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028100","voting_power":10,"score":100}}`,
		`SetValidator|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac73333","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028100","voting_power":10,"score":100}}`,
		`GetAllValidators|{}`,
		`DeleteValidator|{"0":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac72222"}`,
		`GetAllValidators|{}`,
	}
	contract := Regiester("validators")
	for _, input := range inputs {
		_, err := CallContractByInput(st, contract, input)
		if err != nil {
			t.Fail()
		}
	}
}

func TestCoefficientNormal(t *testing.T) {
	st := InitState()
	RegiesterCommittee(st)
	inputs := []string{
		"init|{}",
		`getCoefficient|{}`,
		`updateVoteRate|{"0":{"Deno":3,"Nume":2,"UpperLimit":10}}`,
		`getCoefficient|{}`,
		`updateCalRate|{"0":{"Srate":1,"Drate":2,"Rrate":1}}`,
		`getCoefficient|{}`,
		`updateVotePeriod|{"0":10000}`,
		`getCoefficient|{}`,
		`updateMaxScore|{"0":300}`,
		`getCoefficient|{}`,
		`updateUTXOFee|{"0":"0x123456789"}`,
		`getCoefficient|{}`,
	}
	contract := Regiester("coefficient")
	for _, input := range inputs {
		_, err := CallContractByInput(st, contract, input)
		if err != nil {
			t.Fail()
		}
	}
}

func TestCommitteeAddCommitteeSingleToThree(t *testing.T) {
	var testAddr0 = "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100"
	var testAddr2 = "0x54fb1c7d0f011dd63b08f85ed7b518ab82028102"
	var testAddr4 = "0x54fb1c7d0f011dd63b08f85ed7b518ab82028104"
	var testAddr5 = "0x54fb1c7d0f011dd63b08f85ed7b518ab82028105"

	st := InitState()
	inputs := []string{
		"init|{}",
		"getCommittee|{}",
		`proposaAddMember|{"0":{"address":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028100"}}`,
		`proposaAddMember|{"0":{"address":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028101"}}`,
		`proposaAddMember|{"0":{"address":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028102"}}`,
		`proposaAddMember|{"0":{"address":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028103"}}`,
		`proposaAddMember|{"0":{"address":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028104"}}`,
		`proposaAddMember|{"0":{"address":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028105"}}`,
		`getAllProposalID|{}`,
	}

	//ProposalID
	//\"0\":\"0xa2138fea860caf0010ef5c8194c95d1ad6badc65\", Add testAddr4
	//\"1\":\"0x8589627e04b6304574d6ed431b3d23d3a0d0ecfc\",
	//\"2\":\"0x1cd48f5cc96580564e416692d88164494c7cb6d4\",
	//\"3\":\"0xe388a80c84e135e8eee7b04f6d360f5afcb9ab26\",
	//\"4\":\"0x8597cb92274c061435de5fe9695fcfa567ee8cb2\",
	//\"5\":\"0x44754c8d10e8c03df335988aabfcda9de22853d7\"

	contract := Regiester("committee")
	for _, input := range inputs {
		_, err := CallContractByInput(st, contract, input)
		if err != nil {
			t.Fail()
		}
	}

	//repeat proposal
	_, err := CallContractByInput(st, contract, `proposaAddMember|{"0":{"address":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028105"}}`)
	if err == nil {
		t.Fail()
	}

	//no rights
	inputs = []string{
		`proposaAddMember|{"0":{"address":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028105"}}`,
	}
	for _, input := range inputs {
		_, err := CallContract(st, testAddr5, contract, input, nil)
		if err == nil {
			t.Fail()
		}
	}

	//Add testAddr4
	inputs = []string{
		`voteProposal|{"0":"0xa2138fea860caf0010ef5c8194c95d1ad6badc65"}`,
		`execProposal|{"0":"0xa2138fea860caf0010ef5c8194c95d1ad6badc65"}`,
		"getCommittee|{}",
	}
	for _, input := range inputs {
		_, err := CallContract(st, testAddr0, contract, input, nil)
		if err != nil {
			t.Fail()
		}
	}

	//Add testAddr2
	inputs = []string{
		`voteProposal|{"0":"0x8589627e04b6304574d6ed431b3d23d3a0d0ecfc"}`,
		`getProposal|{"0":"0x8589627e04b6304574d6ed431b3d23d3a0d0ecfc"}`,
		"getCommittee|{}",
	}
	for _, input := range inputs {
		_, err := CallContract(st, testAddr0, contract, input, nil)
		if err != nil {
			t.Fail()
		}
	}

	//exec Fail
	_, err = CallContract(st, testAddr0, contract, `execProposal|{"0":"0x8589627e04b6304574d6ed431b3d23d3a0d0ecfc"}`, nil)
	if err == nil {
		t.Fail()
	}
	//vote Fail
	_, err = CallContract(st, testAddr2, contract, `voteProposal|{"0":"0x8589627e04b6304574d6ed431b3d23d3a0d0ecfc"}`, nil)
	if err == nil {
		t.Fail()
	}

	_, err = CallContract(st, testAddr4, contract, `voteProposal|{"0":"0x8589627e04b6304574d6ed431b3d23d3a0d0ecfc"}`, nil)
	if err != nil {
		t.Fail()
	}

	//Add testAddr2
	inputs = []string{
		`execProposal|{"0":"0x8589627e04b6304574d6ed431b3d23d3a0d0ecfc"}`,
		"getCommittee|{}",
	}
	for _, input := range inputs {
		_, err := CallContract(st, testAddr0, contract, input, nil)
		if err != nil {
			t.Fail()
		}
	}
}

func TestCommitteeDelCommittee(t *testing.T) {
	st := InitState()
	inputs := []string{
		"init|{}",
	}
	contract := Regiester("committee")
	for _, input := range inputs {
		_, err := CallContractByInput(st, contract, input)
		if err != nil {
			t.Fail()
		}
	}
}

func TestCommitteeFinishDelCommittee(t *testing.T) {
	st := InitState()
	inputs := []string{
		"init|{}",
	}
	contract := Regiester("committee")
	for _, input := range inputs {
		_, err := CallContractByInput(st, contract, input)
		if err != nil {
			t.Fail()
		}
	}
}

func TestCommitteeChangeRightsSingle(t *testing.T) {
	st := InitState()
	inputs := []string{
		"init|{}",
	}
	contract := Regiester("committee")
	for _, input := range inputs {
		_, err := CallContractByInput(st, contract, input)
		if err != nil {
			t.Fail()
		}
	}
}

func TestCommitteeChangeRights(t *testing.T) {
	st := InitState()
	inputs := []string{
		"init|{}",
	}
	contract := Regiester("committee")
	for _, input := range inputs {
		_, err := CallContractByInput(st, contract, input)
		if err != nil {
			t.Fail()
		}
	}
}

func pledgeInit(st *state.StateDB, elector string, orderId uint64, t *testing.T) {
	pledgeContract := Regiester("pledge")
	input := `participate|{"0":"` + elector + `","1":"` + WinoutAmount.String() + `","2":` + strconv.FormatUint(orderId, 10) + `,"3":91}`
	_, err := CallContract(st, elector, pledgeContract, input, WinoutAmount)
	if err != nil {
		t.Fail()
	}

	inputs := []string{
		`setElectorStatus|{"0":"` + elector + `","1":4}`,
	}

	for _, input := range inputs {
		_, err := CallContractByInput(st, pledgeContract, input)
		if err != nil {
			t.Fail()
		}
	}
}

func addPledge(st *state.StateDB, support string, elector string, value *big.Int, orderId uint64, t *testing.T) {
	pledgeContract := Regiester("pledge")

	input := `setElectorStatus|{"0":"` + elector + `","1":3}`
	_, err := CallContractByInput(st, pledgeContract, input)
	if err != nil {
		t.Fail()
	}

	input = `deposit|{"0":"` + elector + `","1":"` + value.String() + `","2":` + strconv.FormatUint(orderId, 10) + `}`
	_, err = CallContract(st, support, pledgeContract, input, value)
	if err != nil {
		t.Fail()
	}

	inputs := []string{
		`setElectorStatus|{"0":"` + elector + `","1":4}`,
		`getElectorInfo|{"0":"` + elector + `"}`,
		`getDeposit|{}`,
	}

	for _, input := range inputs {
		_, err := CallContractByInput(st, pledgeContract, input)
		if err != nil {
			t.Fail()
		}
	}
}

func TestPledgeNormal(t *testing.T) {
	st := InitState()

	candidatesContract := Regiester("candidates")
	coefficientContract := Regiester("coefficient")
	committeeContract := Regiester("committee")
	foundationContract := Regiester("foundation")
	pledgeContract := Regiester("pledge")
	validatorsContract := Regiester("validators")

	initInputs := "init|{}"
	CallContractByInput(st, candidatesContract, initInputs)
	CallContractByInput(st, coefficientContract, initInputs)
	CallContractByInput(st, committeeContract, initInputs)
	CallContractByInput(st, foundationContract, initInputs)
	CallContractByInput(st, pledgeContract, initInputs)
	CallContractByInput(st, validatorsContract, initInputs)

	candidateCoinbase := []string{
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028110",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028111",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028112",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028113",
	}

	supportCoinbase := []string{
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028120",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028121",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028122",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028123",
	}

	var orderId uint64 = 0
	for _, coinbase := range candidateCoinbase {
		pledgeInit(st, coinbase, orderId, t)
		orderId += 1
	}

	for _, coinbase := range supportCoinbase {
		addPledge(st, coinbase, candidateCoinbase[0], WinoutAmount, orderId, t)
		orderId += 1
	}
	for _, coinbase := range supportCoinbase {
		addPledge(st, coinbase, candidateCoinbase[1], InitAmount, orderId, t)
		orderId += 1
	}
	CallContractByInput(st, pledgeContract, "getDeposit|{}")
	CallContractByInput(st, pledgeContract, `getPledgeRecord|{"0":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028110"}`)

}

func TestPledgeVote(t *testing.T) {

	st := InitState()

	candidatesContract := Regiester("candidates")
	coefficientContract := Regiester("coefficient")
	committeeContract := Regiester("committee")
	foundationContract := Regiester("foundation")
	pledgeContract := Regiester("pledge")
	validatorsContract := Regiester("validators")

	initInputs := "init|{}"
	CallContractByInput(st, candidatesContract, initInputs)
	CallContractByInput(st, coefficientContract, initInputs)
	CallContractByInput(st, committeeContract, initInputs)
	CallContractByInput(st, foundationContract, initInputs)
	CallContractByInput(st, pledgeContract, initInputs)
	CallContractByInput(st, validatorsContract, initInputs)

	input := `participate|{"0":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028101","1":"` + WinoutAmount.String() + `","2":123,"3":90}`
	_, err := CallContract(st, testAddr0, pledgeContract, input, WinoutAmount)
	if err != nil {
		t.Fail()
	}

	inputs := []string{
		`getElectorInfo|{"0":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028101"}`,
		`setElectorStatus|{"0":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028101","1":4}`,
		`getElectorInfo|{"0":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028101"}`,
		`getDeposit|{}`,
	}
	for _, input := range inputs {
		_, err := CallContractByInput(st, pledgeContract, input)
		if err != nil {
			t.Fail()
		}
	}
}

func BlockAward(st *state.StateDB, coinbase string, foundationContract *wasm.Contract, t *testing.T) {
	input := `setPoceeds|{"0":"` + coinbase + `","1":"3333333333"}`
	_, err := CallContract(st, foundCall, foundationContract, input, nil)
	if err != nil {
		t.Fail()
	}
	st.AddBalance(common.HexToAddress(ContractFoundationAddr), big.NewInt(3333333333))
}

func TestFoundationBlockAward(t *testing.T) {
	st := InitState()
	foundationContract := Regiester("foundation")
	BlockAward(st, testAddr0, foundationContract, t)
	//st.GetState(common.HexToHash
}

func TestFoundationNormal(t *testing.T) {
	st := InitState()

	RegiesterCommittee(st)

	pledgeContract := Regiester("pledge")
	candidatesContract := Regiester("candidates")
	foundationContract := Regiester("foundation")
	validatorsContract := Regiester("validators")

	initInputs := "init|{}"
	CallContractByInput(st, candidatesContract, initInputs)
	CallContractByInput(st, foundationContract, initInputs)
	CallContractByInput(st, validatorsContract, initInputs)
	CallContractByInput(st, pledgeContract, initInputs)

	candidateCoinbase := []string{
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028110",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028111",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028112",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028113", //score = 0 -> 100 -> 0
	}

	supportCoinbase := []string{
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028120",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028121",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028122",
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028123",
	}

	validCoinbase := []string{
		"0x54fb1c7d0f011dd63b08f85ed7b518ab82028100",
	}

	var orderId uint64 = 0
	for _, coinbase := range candidateCoinbase {
		pledgeInit(st, coinbase, orderId, t)
		orderId += 1
	}

	inputs := []string{
		`SetCandidate|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac70000","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028110","voting_power":10,"score":100,"punish_height":0}}`,
		`SetCandidate|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac71111","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028111","voting_power":0,"score":300,"punish_height":0}}`,
		`SetCandidate|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac72222","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028112","voting_power":10,"score":300,"punish_height":0}}`,
		`SetCandidate|{"0":{"pub_key":"0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac73333","coinbase":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028113","voting_power":10,"score":0,"punish_height":0}}`,
	}
	for _, input := range inputs {
		_, err := CallContractByInput(st, candidatesContract, input)
		if err != nil {
			t.Fail()
		}
	}

	for _, coinbase := range supportCoinbase {
		addPledge(st, coinbase, candidateCoinbase[0], new(big.Int).Mul(OneLink, big.NewInt(1000000)), orderId, t)
		orderId += 1
	}

	addPledge(st, supportCoinbase[0], candidateCoinbase[0], new(big.Int).Mul(OneLink, big.NewInt(1000000)), orderId, t)
	orderId += 1

	for _, coinbase := range supportCoinbase {
		addPledge(st, coinbase, candidateCoinbase[1], new(big.Int).Mul(OneLink, big.NewInt(1000000)), orderId, t)
		orderId += 1
	}

	for _, coinbase := range supportCoinbase {
		addPledge(st, coinbase, candidateCoinbase[2], new(big.Int).Mul(OneLink, big.NewInt(1000000)), orderId, t)
		orderId += 1
	}

	for _, coinbase := range candidateCoinbase {
		BlockAward(st, coinbase, foundationContract, t)
	}

	CallContractByInput(st, pledgeContract, "getDeposit|{}")
	CallContractByInput(st, candidatesContract, "GetAllCandidates|{}")
	allAward := st.GetBalance(common.HexToAddress(ContractFoundationAddr)).String()
	_, err := CallContract(st, foundCall, foundationContract, "allocAward|{}", nil)
	if err != nil {
		t.Fail()
	}

	for _, coinbase := range supportCoinbase {
		fmt.Println(coinbase, st.GetBalance(common.HexToAddress(coinbase)).String())
	}

	for _, coinbase := range candidateCoinbase {
		fmt.Println(coinbase, st.GetBalance(common.HexToAddress(coinbase)).String())
	}

	for _, coinbase := range validCoinbase {
		fmt.Println(coinbase, st.GetBalance(common.HexToAddress(coinbase)).String())
	}

	fmt.Println("Foundation Before award", allAward)

	allAward = st.GetBalance(common.HexToAddress(ContractFoundationAddr)).String()

	fmt.Println("Foundation After award", allAward)
}
