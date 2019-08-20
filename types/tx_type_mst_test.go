package types

import (
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/assert"

	"fmt"
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
)

func TestMultSignAccountTx(t *testing.T) {
	key1, err := crypto.GenerateKey()
	assert.Nil(t, err)
	key2, err := crypto.GenerateKey()
	assert.Nil(t, err)
	addr1 := crypto.PubkeyToAddress(key1.PublicKey)
	addr2 := crypto.PubkeyToAddress(key2.PublicKey)

	signersInfo := SignersInfo{
		MinSignerPower: 1,
		Signers: []*SignerEntry{
			&SignerEntry{Power: 1, Addr: addr1},
			&SignerEntry{Power: 1, Addr: addr2},
		},
	}
	accountNonce := uint64(0)
	supportTxType := TxContractCreateType

	mainInfo := MultiSignMainInfo{
		AccountNonce:  accountNonce,
		SupportTxType: supportTxType,
		SignersInfo:   signersInfo,
	}

	signautures := []ValidatorSign{
		ValidatorSign{Addr: []byte{1}, Signature: []byte{2}},
	}

	tx := NewMultiSignAccountTx(&mainInfo, signautures)

	_, err = GenMultiSignBytes(mainInfo)
	assert.Nil(t, err)

	from, _ := tx.From()
	assert.Equal(t, MultiSignNonceAddr, from)

	assert.Nil(t, tx.To())
	assert.Equal(t, TxMultiSignAccount, tx.TypeName())
	_ = tx.String()

	_, p := RandValidator(false, 1)
	err = tx.Sign(p)
	assert.Nil(t, err)
}

func TestMultiSignAccountTxCheckBasic(t *testing.T) {
	var tx = new(MultiSignAccountTx)

	censor := &MockTxCensor{}

	censor.On("NodeType").Return("")
	censor.On("GetLastChangedVals").Return(uint64(0), nil)
	err := tx.CheckBasic(censor)
	assert.NotNil(t, err)
}

func TestMultiSignAccountTxCheckState(t *testing.T) {
	var tx = new(MultiSignAccountTx)
	tx.AccountNonce = uint64(1)

	censor := &MockTxCensor{}
	state := &MockState{}
	censor.On("State").Return(state)
	censor.On("LockState").Return()
	censor.On("UnlockState").Return()

	state.On("GetNonce", mock.Anything).Return(tx.Nonce() + 1).Once()
	err := tx.CheckState(censor)
	assert.Equal(t, ErrNonceTooLow, err)

	state.On("GetNonce", mock.Anything).Return(tx.Nonce() - 1).Once()
	err = tx.CheckState(censor)
	assert.Equal(t, ErrNonceTooHigh, err)

	state.On("GetNonce", mock.Anything).Return(tx.Nonce())
	state.On("SetNonce", mock.Anything, mock.Anything).Return()
	err = tx.CheckState(censor)
	assert.Nil(t, err)
}

func TestMultiSignVerify(t *testing.T) {
	mainInfo, privs, valSet := getTestMultiSignMainInfo(TxUpdateValidatorsType)
	signBytes, err := GenMultiSignBytes(*mainInfo)
	if err != nil {
		t.Fatalf("ser.EncodeToBytes err:%v", err)
	}

	sigs := make([]ValidatorSign, 0, len(privs))
	for _, priv := range privs {
		sig, err := priv.SignData(signBytes)
		if err != nil {
			t.Fatalf("priv.SignData err:%v", err)
		}
		sigs = append(sigs, ValidatorSign{priv.GetAddress().Bytes(), sig})
	}

	// duplicate signature
	sigs2 := sigs[:1]
	sigs2 = append(sigs2, sigs...)
	mtx1 := NewMultiSignAccountTx(mainInfo, sigs2)
	var ok = false
	err = mtx1.VerifySign(valSet)
	if err != nil {
		ok = true
	}
	assert.Equal(t, true, ok, "multiSignTx verify must err:%v", err)

	// invalid validator
	vals := []*Validator{
		NewValidator(crypto.PubKeyEd25519([32]byte{1}), common.EmptyAddress, 1),
		NewValidator(crypto.PubKeyEd25519([32]byte{2}), common.EmptyAddress, 1),
		NewValidator(crypto.PubKeyEd25519([32]byte{3}), common.EmptyAddress, 1),
		NewValidator(crypto.PubKeyEd25519([32]byte{4}), common.EmptyAddress, 1),
	}
	mtx2 := NewMultiSignAccountTx(mainInfo, sigs)
	err = mtx2.VerifySign(NewValidatorSet(vals))
	if err != nil {
		ok = true
	} else {
		ok = false
	}
	assert.Equal(t, true, ok, "multiSignTx verify must err:%v", err)

	// invalid validator addr
	mtx3 := NewMultiSignAccountTx(mainInfo, []ValidatorSign{ValidatorSign{Addr: []byte{1}, Signature: []byte{2}}})
	err = mtx3.VerifySign(valSet)
	if err != nil {
		ok = true
	} else {
		ok = false
	}
	assert.Equal(t, true, ok, "multiSignTx verify must err:%v", err)

	// unmarshal to crypto.Signature failed
	mtx4 := NewMultiSignAccountTx(mainInfo, []ValidatorSign{ValidatorSign{Addr: sigs[0].Addr, Signature: []byte{2}}})
	err = mtx4.VerifySign(valSet)
	if err != nil {
		ok = true
	} else {
		ok = false
	}
	assert.Equal(t, true, ok, "multiSignTx verify must err:%v", err)

	// invalid signature
	mtx5 := NewMultiSignAccountTx(mainInfo, []ValidatorSign{ValidatorSign{Addr: sigs[0].Addr, Signature: sigs[1].Signature}})
	err = mtx5.VerifySign(valSet)
	if err != nil {
		ok = true
	} else {
		ok = false
	}
	assert.Equal(t, true, ok, "multiSignTx verify must err:%v", err)

	// insufficient voting power
	mtx6 := NewMultiSignAccountTx(mainInfo, sigs[:3])
	err = mtx6.VerifySign(valSet)
	if err != nil {
		ok = true
	} else {
		ok = false
	}
	assert.Equal(t, true, ok, "multiSignTx verify must err:%v", err)

	// ok
	mtx7 := NewMultiSignAccountTx(mainInfo, sigs[0:7])
	err = mtx7.VerifySign(valSet)
	assert.Equal(t, nil, err, "multiSignTx verify err:%v", err)
	fmt.Printf("%v\n", mtx7)
}

func TestContractCreateTxVerify(t *testing.T) {
	fromPriveKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("could not generate key: %v", err)
	}
	mainInfo := &ContractCreateMainInfo{
		FromAddr:     crypto.PubkeyToAddress(fromPriveKey.PublicKey),
		AccountNonce: 1,
		Amount:       big.NewInt(100),
		Payload:      []byte{126},
	}
	var numsigners int32 = 5
	signatures := make([][]byte, 0)
	signersInfo := &SignersInfo{MinSignerPower: numsigners - 1}
	signersInfo.Signers = make([]*SignerEntry, numsigners)
	for i := 0; i < len(signersInfo.Signers); i++ {
		priveKey, err := crypto.GenerateKey()
		if err != nil {
			t.Fatalf("could not generate key: %v", err)
		}

		sigData, err := SignContractCreateTx(priveKey, mainInfo)
		if err != nil {
			t.Fatalf("SignContractCreateTx failed:%v", err)
		}
		signatures = append(signatures, sigData)
		pubAddr := crypto.PubkeyToAddress(priveKey.PublicKey)
		signersInfo.Signers[i] = &SignerEntry{Power: 1, Addr: pubAddr}
	}
	//from addr's signature is not include in signatures
	var ok = false
	tx := CreateContractTx(mainInfo, signatures)
	expectFlag := true
	if tx == nil {
		expectFlag = false
	}
	assert.Equal(t, true, expectFlag, "CreateContractTx failed")
	err = tx.VerifySign(signersInfo)
	if err != nil {
		ok = true
	} else {
		ok = false
	}
	assert.Equal(t, true, ok, "verify err:%v", err)
	//normal test
	sigData, err := SignContractCreateTx(fromPriveKey, mainInfo)
	if err != nil {
		t.Fatalf("SignContractCreateTx failed:%v", err)
	}
	signatures = append(signatures, sigData)
	tx = CreateContractTx(mainInfo, signatures)
	expectFlag = true
	if tx == nil {
		expectFlag = false
	}
	assert.Equal(t, true, expectFlag, "CreateContractTx failed")
	err = tx.VerifySign(signersInfo)
	assert.Equal(t, nil, err, "verify err:%v", err)
	//test len(signatures) > len(savedSignersInfo.Signers)
	signatures2 := append(signatures, signatures[1])
	tx = CreateContractTx(mainInfo, signatures2)
	expectFlag = true
	if tx == nil {
		expectFlag = false
	}
	assert.Equal(t, true, expectFlag, "CreateContractTx failed")
	err = tx.VerifySign(signersInfo)
	if err != nil {
		ok = true
	} else {
		ok = false
	}
	assert.Equal(t, ok, ok, "verify err:%v", err)
	//invalid  signatures
	signatures3 := append(signatures, []byte{12})
	tx = CreateContractTx(mainInfo, signatures3)
	expectFlag = true
	if tx != nil {
		expectFlag = false
	}
	assert.Equal(t, true, expectFlag, "CreateContractTx failed")
	//invalid mainInfo
	changedMainInfo := mainInfo
	changedMainInfo.Amount = big.NewInt(1000000)
	tx = CreateContractTx(changedMainInfo, signatures)
	err = tx.VerifySign(signersInfo)
	if err != nil {
		ok = true
	} else {
		ok = false
	}
	assert.Equal(t, true, ok, "verify err:%v", err)
	// insufficient voting power
	tx = CreateContractTx(changedMainInfo, signatures[:signersInfo.MinSignerPower-2])
	err = tx.VerifySign(signersInfo)
	if err != nil {
		ok = true
	} else {
		ok = false
	}
	assert.Equal(t, true, ok, "verify err:%v", err)
	fmt.Printf("%v\n", tx)
}

func getTestMultiSignMainInfo(txType SupportType) (*MultiSignMainInfo, []PrivValidator, *ValidatorSet) {
	validators := make([]*Validator, 0, 10)
	privs := make([]PrivValidator, 0, 10)
	for i := 0; i < 10; i++ {
		v, p := RandValidator(false, 1)
		validators = append(validators, v)
		privs = append(privs, p)
	}
	valSet := NewValidatorSet(validators)

	mainInfo := &MultiSignMainInfo{
		SupportTxType: txType,
		SignersInfo: SignersInfo{
			MinSignerPower: 20,
			Signers: []*SignerEntry{
				&SignerEntry{
					Power: 10,
					Addr:  common.HexToAddress("0x1"),
				},
				&SignerEntry{
					Power: 10,
					Addr:  common.HexToAddress("0x2"),
				},
				&SignerEntry{
					Power: 10,
					Addr:  common.HexToAddress("0x3"),
				},
			},
		},
	}
	return mainInfo, privs, valSet
}
