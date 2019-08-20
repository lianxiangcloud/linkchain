package types

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

func TestContractCreate(t *testing.T) {
	key1, err := crypto.GenerateKey()
	assert.Nil(t, err)
	key2, err := crypto.GenerateKey()
	assert.Nil(t, err)
	addr1 := crypto.PubkeyToAddress(key1.PublicKey)
	addr2 := crypto.PubkeyToAddress(key2.PublicKey)

	nonce := uint64(2)
	amount := big.NewInt(3)
	payload := []byte{5}
	mainInfo := &ContractCreateMainInfo{
		FromAddr:     addr1,
		AccountNonce: nonce,
		Amount:       amount,
		Payload:      payload,
	}

	_, err = SignContractCreateTx(nil, mainInfo)
	assert.Equal(t, err, ErrParams)
	_, err = SignContractCreateTx(key1, mainInfo)
	assert.Nil(t, err)

	tx := CreateContractTx(nil, nil)
	assert.Nil(t, tx)

	tx = CreateContractTx(mainInfo, nil)
	assert.NotNil(t, tx)

	assert.Nil(t, tx.To())

	err = tx.Sign(GlobalSTDSigner, key1)
	assert.Nil(t, err)

	err = tx.Sign(GlobalSTDSigner, key2)
	assert.Nil(t, err)

	addrs, err := tx.Senders()
	assert.Nil(t, err)
	assert.Equal(t, addr1, addrs[0])
	assert.Equal(t, addr2, addrs[1])

	assert.Equal(t, TxContractCreateType, tx.txType())
	assert.Equal(t, payload, tx.Data())

	_, err = tx.AsMessage()
	assert.Nil(t, err)

	bs, err := ser.EncodeToBytes(tx)
	assert.Nil(t, err)

	tx2 := new(ContractCreateTx)
	err = ser.DecodeBytes(bs, tx2)
	assert.Nil(t, err)

	assert.Equal(t, tx.Hash(), tx2.Hash())
}

func TestContractCreateCheckBasic(t *testing.T) {
	censor := &MockTxCensor{}
	txMgr := &MockTxMgr{}
	censor.On("TxMgr").Return(txMgr)

	var tx *ContractCreateTx
	err := tx.CheckBasic(censor)
	assert.Equal(t, ErrTxEmpty, err)

	tx = new(ContractCreateTx)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrParams, err)

	tx.ContractCreateMainInfo.Payload = []byte{1}
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrParams, err)

	tx.ContractCreateMainInfo.Amount = big.NewInt(-1)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrNegativeValue, err)

	tx.ContractCreateMainInfo.Amount = big.NewInt(0)
	tx.ContractCreateMainInfo.FromAddr = common.HexToAddress("0x1")

	txMgr.On("GetMultiSignersInfo", mock.Anything).Return(nil).Once()
	tx.ContractCreateMainInfo.FromAddr = common.HexToAddress("0x0")
	tx = &ContractCreateTx{ContractCreateMainInfo: tx.ContractCreateMainInfo}
	err = tx.CheckBasic(censor)
	assert.NotNil(t, err)

	key1, err := crypto.GenerateKey()
	assert.Nil(t, err)
	key2, err := crypto.GenerateKey()
	assert.Nil(t, err)
	addr1 := crypto.PubkeyToAddress(key1.PublicKey)
	addr2 := crypto.PubkeyToAddress(key2.PublicKey)

	signersInfo := &SignersInfo{
		MinSignerPower: 2,
		Signers: []*SignerEntry{
			&SignerEntry{Power: 1, Addr: addr1},
		},
	}
	err = tx.Sign(GlobalSTDSigner, key1)
	assert.Nil(t, err)
	txMgr.On("GetMultiSignersInfo", mock.Anything).Return(signersInfo).Once()
	err = tx.CheckBasic(censor)
	assert.NotNil(t, err)

	nonce := uint64(2)
	amount := big.NewInt(3)
	payload := []byte{5}
	mainInfo := &ContractCreateMainInfo{
		FromAddr:     addr1,
		AccountNonce: nonce,
		Amount:       amount,
		Payload:      payload,
	}
	tx = CreateContractTx(mainInfo, nil)

	err = tx.Sign(GlobalSTDSigner, key1)
	assert.Nil(t, err)

	err = tx.Sign(GlobalSTDSigner, key2)
	assert.Nil(t, err)

	var tx2 *ContractCreateTx
	err = tx2.VerifySign(signersInfo)
	assert.NotNil(t, err)

	tx2 = new(ContractCreateTx)
	err = tx2.VerifySign(nil)
	assert.NotNil(t, err)

	err = tx2.VerifySign(&SignersInfo{MinSignerPower: 1})
	assert.NotNil(t, err)

	err = tx.VerifySign(signersInfo)
	assert.NotNil(t, err)

	signersInfo.Signers = append(signersInfo.Signers, &SignerEntry{Power: 1, Addr: addr2})
	err = tx.VerifySign(signersInfo)
	assert.Nil(t, err)
}

func TestContractCreateCheckState(t *testing.T) {
	censor := &MockTxCensor{}
	state := &MockState{}
	censor.On("State").Return(state)
	censor.On("LockState").Return()
	censor.On("UnlockState").Return()
	state.On("SetNonce", mock.Anything, mock.Anything).Return()

	addr := common.HexToAddress("0x01")
	nonce := uint64(2)
	amount := big.NewInt(3)
	payload := []byte{5}
	mainInfo := &ContractCreateMainInfo{
		FromAddr:     addr,
		AccountNonce: nonce,
		Amount:       amount,
		Payload:      payload,
	}
	tx := CreateContractTx(mainInfo, nil)

	tx.ContractCreateMainInfo.FromAddr = common.HexToAddress("0x0")
	state.On("GetNonce", mock.Anything).Return(tx.Nonce() + 1).Once()
	err := tx.CheckState(censor)
	assert.Equal(t, ErrNonceTooLow, err)

	state.On("GetNonce", mock.Anything).Return(tx.Nonce() - 1).Once()
	err = tx.CheckState(censor)
	assert.Equal(t, ErrNonceTooHigh, err)

	state.On("GetNonce", mock.Anything).Return(tx.Nonce())
	state.On("SetNonce", mock.Anything, mock.Anything).Return()
	state.On("GetBalance", mock.Anything, mock.Anything).Return(big.NewInt(0).Sub(tx.Cost(), big.NewInt(1))).Once()
	err = tx.CheckState(censor)
	assert.Equal(t, ErrInsufficientFunds, err)

	state.On("GetBalance", mock.Anything).Return(tx.Cost()).Once()
	state.On("SubBalance", mock.Anything, mock.Anything).Return()
	err = tx.CheckState(censor)
	assert.Nil(t, err)
}
