package rtypes

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	cptypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/types"
)

func TestRPCTx(t *testing.T) {
	txs := getTestTxs()
	rtx := NewRPCTx(txs[0], &types.TxEntry{
		BlockHash: common.HexToHash("0x0"),
	})
	bs, err := json.Marshal(rtx)
	assert.Nil(t, err)

	var tx = &RPCTx{}
	err = json.Unmarshal(bs, tx)
	assert.Nil(t, err)
	assert.Equal(t, txs[0].TypeName(), tx.TxType)
	assert.Equal(t, txs[0].Hash(), tx.Tx.Hash())
}

func TestRPCBlock(t *testing.T) {
	txs := getTestTxs()

	privVals, validators := genValidators()
	blockID := types.BlockID{common.EmptyHash, types.PartSetHeader{}}

	commit := genCommit(privVals, validators)

	block := &types.Block{
		Header: &types.Header{
			LastBlockID: blockID,
		},
		Data: &types.Data{
			Txs: txs,
		},
		LastCommit: commit,
	}

	rb := NewRPCBlock(block, true, true)
	bs, err := json.Marshal(rb)
	if err != nil {
		t.Fatalf("json.Marshal err:%v", err)
	}

	// fmt.Println(string(bs))
	rb = &RPCBlock{}
	err = json.Unmarshal(bs, rb)
	assert.Nil(t, err)

	var receipts types.Receipts
	for _, tx := range txs {
		receipts = append(receipts, &types.Receipt{TxHash: tx.Hash()})
	}
	wBlock := NewWholeBlock(block, receipts)

	bs, err = json.Marshal(wBlock)
	if err != nil {
		t.Fatalf("json.Marshal err:%v", err)
	}

	// fmt.Println(string(bs))

	var wb = &WholeBlock{}
	err = json.Unmarshal(bs, wb)
	assert.Nil(t, err)

	txsDecoded := wb.Block.Txs
	for i, itx := range txsDecoded {
		tx, ok := itx.(*RPCTx)
		if !ok {
			t.Fatalf("tx not types.Tx type")
		}
		assert.Equal(t, txs[i].Hash(), tx.TxHash)
		assert.Equal(t, txs[i].Hash(), tx.Tx.Hash())
		assert.Equal(t, txs[i].Hash(), wb.Receipts[i].TxHash)
	}
}

func TestRPCOutput(t *testing.T) {
	out := cptypes.Key{1}
	commit := cptypes.Key{2}
	output := RPCOutput{
		Out:     RPCKey(out),
		Height:  88,
		Commit:  RPCKey(commit),
		TokenID: common.EmptyAddress,
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(string(data))

	var obj RPCOutput
	if err = json.Unmarshal(data, &obj); err != nil {
		t.Error(err)
		return
	}
	t.Logf("%#v", obj)
}

func getTestTxs() []types.Tx {
	normalTx := getNormalTx()
	tokenTx := getTokenTx()
	return []types.Tx{
		normalTx,
		tokenTx,
		getContractCreateTx(),
		getContractUpgradeTx(),
		//getMultiSignAccountTx(types.TxUpdateAddrRouteType),
	}
}

func getNormalTx() *types.Transaction {
	acc, err := crypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	tx := types.NewTransaction(1, common.HexToAddress("0x2"), big.NewInt(3), 4, big.NewInt(5), []byte{6})
	err = tx.Sign(types.GlobalSTDSigner, acc)
	if err != nil {
		panic(err)
	}
	_, err = tx.From()
	if err != nil {
		panic(err)
	}
	return tx
}

func getTokenTx() *types.TokenTransaction {
	acc, err := crypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	token := common.HexToAddress("0x1")
	tx := types.NewTokenTransaction(token, 2, common.HexToAddress("0x3"), big.NewInt(4), 5, big.NewInt(6), []byte{7})
	err = tx.Sign(types.GlobalSTDSigner, acc)
	if err != nil {
		panic(err)
	}
	_, err = tx.From()
	if err != nil {
		panic(err)
	}
	return tx
}

func getContractCreateTx() *types.ContractCreateTx {
	return &types.ContractCreateTx{
		ContractCreateMainInfo: types.ContractCreateMainInfo{
			FromAddr:     common.HexToAddress("0x1"),
			AccountNonce: 2,
			Amount:       big.NewInt(3),
			Payload:      []byte{5},
		},
	}
}

func getContractUpgradeTx() *types.ContractUpgradeTx {
	return &types.ContractUpgradeTx{
		ContractUpgradeMainInfo: types.ContractUpgradeMainInfo{
			FromAddr:     common.HexToAddress("0x1"),
			Recipient:    common.HexToAddress("0x2"),
			AccountNonce: 3,
			Payload:      []byte{4},
		},
	}
}

func getMultiSignAccountTx(txType types.SupportType) *types.MultiSignAccountTx {
	return &types.MultiSignAccountTx{
		MultiSignMainInfo: types.MultiSignMainInfo{
			AccountNonce:  0,
			SupportTxType: txType,
			SignersInfo: types.SignersInfo{
				MinSignerPower: 20,
				Signers: []*types.SignerEntry{
					&types.SignerEntry{
						Power: 10,
						Addr:  common.HexToAddress("0x1"),
					},
					&types.SignerEntry{
						Power: 10,
						Addr:  common.HexToAddress("0x2"),
					},
					&types.SignerEntry{
						Power: 10,
						Addr:  common.HexToAddress("0x3"),
					},
				},
			},
		},
		Signatures: []types.ValidatorSign{
			types.ValidatorSign{Addr: []byte{1}, Signature: []byte{2}},
			types.ValidatorSign{Addr: []byte{3}, Signature: []byte{4}},
		},
	}
}

func genValidators() ([]types.PrivValidator, []*types.Validator) {
	privVals, validators := make([]types.PrivValidator, 0, 4), make([]*types.Validator, 0, 4)

	for i := 0; i < 4; i++ {
		privVals = append(privVals, types.GenFilePV(""))
		validators = append(validators, types.NewValidator(privVals[i].GetPubKey(), common.EmptyAddress, 1))
	}
	return privVals, validators
}

func genCommit(privVals []types.PrivValidator, validators []*types.Validator) *types.Commit {
	valNum := len(validators)

	valSet := types.NewValidatorSet(validators)
	voteSet := types.NewVoteSet("chainID", 10, 0, types.VoteTypePrecommit, valSet)

	for i := 0; i < valNum; i++ {
		addr := privVals[i].GetAddress()
		valIndex, _ := valSet.GetByAddress(addr)
		vote := &types.Vote{
			ValidatorAddress: addr,
			ValidatorIndex:   valIndex,
			ValidatorSize:    valNum,
			Height:           10,
			Round:            0,
			Timestamp:        time.Now().UTC(),
			Type:             types.VoteTypePrecommit,
			BlockID:          types.BlockID{common.EmptyHash, types.PartSetHeader{}},
		}
		if err := privVals[i].SignVoteWithoutSave("chainID", vote); err != nil {
			panic("SignVote fail")
		}
		voteSet.AddVote(vote)
	}
	commit := voteSet.MakeCommit()
	return commit
}
