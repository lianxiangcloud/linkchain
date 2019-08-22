package types

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
)

func TestTokenTransaction(t *testing.T) {
	nonce := uint64(0)
	to := common.HexToAddress("0x1")
	amount := big.NewInt(2)
	gasLimit := uint64(3)
	gasPrice := big.NewInt(4)
	data := []byte{5}
	tx := NewTokenTransaction(to, nonce, to, amount, gasLimit, gasPrice, data)

	key, err := crypto.GenerateKey()
	assert.Nil(t, err)

	addr := crypto.PubkeyToAddress(key.PublicKey)

	err = tx.Sign(GlobalSTDSigner, key)
	assert.Nil(t, err)

	fields := append(tx.signFields(), tx.SignParam(), uint(0), uint(0))
	h := rlpHash(fields)
	assert.Equal(t, h, tx.SignHash())

	from, err := tx.From()
	assert.Nil(t, err)
	assert.Equal(t, addr, from)

	assert.Equal(t, GlobalSTDSigner.SignParam().String(), tx.SignParam().String())

	assert.Equal(t, TxToken, tx.TypeName())

	tx.StoreFrom(to)
	nFrom, _ := tx.From()
	assert.Equal(t, to, nFrom)
	tx.StoreFrom(addr)

	jsonData := map[string]string{"key": "val"}
	bs, _ := json.Marshal(jsonData)
	tx3 := NewContractCreation(nonce, amount, gasLimit, gasPrice, bs)

	assert.Equal(t, false, IsContract(tx3.Data()))
	assert.Equal(t, true, IsContract(tx.Data()))

	assert.Equal(t, true, tx3.IllegalGasLimitOrGasPrice(false))
	tx3.data.GasLimit = uint64(1)
	assert.Equal(t, true, tx3.IllegalGasLimitOrGasPrice(false))
	tx3.data.GasLimit = ParGasLimit
	tx3.data.Price = big.NewInt(1)
	assert.Equal(t, true, tx3.IllegalGasLimitOrGasPrice(false))

	bf := bytes.NewBuffer(nil)
	err = tx.EncodeSER(bf)
	assert.Nil(t, err)

	tx4 := new(TokenTransaction)
	err = tx4.DecodeSER(ser.NewStream(bf, 0))
	assert.Nil(t, err)
	assert.Equal(t, tx.Hash(), tx4.Hash())

	bs, err = json.Marshal(tx)
	assert.Nil(t, err)

	tx5 := new(TokenTransaction)
	err = json.Unmarshal(bs, tx5)
	assert.Nil(t, err)
	assert.Equal(t, tx.Hash(), tx5.Hash())

	assert.Equal(t, to, tx.TokenAddress())
	assert.Equal(t, data, tx.Data())
	assert.Equal(t, gasLimit, tx.Gas())
	assert.Equal(t, ParGasPrice, tx.GasPrice().Int64())
	assert.Equal(t, amount.String(), tx.Value().String())
	assert.Equal(t, nonce, tx.Nonce())
	assert.Equal(t, to, *tx.To())

	tx.Size()

	_, err = tx.AsMessage()
	assert.Nil(t, err)

	gasCost := new(big.Int).Mul(tx.GasPrice(), big.NewInt(int64(tx.Gas())))
	cost := new(big.Int).Add(gasCost, tx.Value())
	assert.Equal(t, cost.String(), tx.Cost().String())
	assert.Equal(t, gasCost.String(), tx.GasCost().String())

	v, r, s := tx.data.Signdata.V, tx.data.Signdata.R, tx.data.Signdata.S
	v1, r1, s1 := tx.RawSignatureValues()
	assert.Equal(t, v.String(), v1.String())
	assert.Equal(t, r.String(), r1.String())
	assert.Equal(t, s.String(), s1.String())

	_ = tx.String()
}

func TestTokenTransactionCheckBasic(t *testing.T) {
	censor := &MockTxCensor{}
	state := &MockState{}
	censor.On("State").Return(state)
	censor.On("LockState").Return()
	censor.On("UnlockState").Return()

	to := common.HexToAddress("0x01")
	var tx *TokenTransaction
	err := tx.CheckBasic(censor)
	assert.Equal(t, ErrTxEmpty, err)

	tx = new(TokenTransaction)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrInvalidReceiver, err)

	tx.data.Recipient = &to
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrParams, err)

	tx.data.Amount = big.NewInt(-1)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrParams, err)

	tx.data.Price = big.NewInt(0)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrNegativeValue, err)

	tx.data.Amount = big.NewInt(0)
	tx.data.Payload = make([]byte, 32768)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrOversizedData, err)

	tx = &TokenTransaction{data: tx.data}
	tx.data.Payload = nil
	state.On("IsContract", mock.Anything).Return(true)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrGasLimitOrGasPrice, err)

	tx.data.GasLimit = ParGasLimit
	tx.data.Price = big.NewInt(ParGasPrice)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrInvalidSender, err)

	z1Addr := common.HexToAddress("0x1")
	tx.StoreFrom(z1Addr)

	tx = &TokenTransaction{data: tx.data}
	z0Addr := common.HexToAddress("0x0")
	tx.StoreFrom(z0Addr)

	tx.data.Payload = make([]byte, 32000)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrIntrinsicGas, err)

	tx.data.GasLimit = ParGasLimit - 1
	tx.data.Payload = []byte{0}

	err = tx.CheckBasic(censor)
	assert.Nil(t, err)

	tx.data.Payload = nil
	err = tx.CheckBasic(censor)
	assert.Nil(t, err)
}

func TestTokenTransactionCheckState(t *testing.T) {
	censor := &MockTxCensor{}
	state := &MockState{}
	censor.On("State").Return(state)
	censor.On("LockState").Return()
	censor.On("UnlockState").Return()

	nonce := uint64(1)
	to := common.HexToAddress("0x1")
	amount := big.NewInt(2)
	gasLimit := uint64(3)
	gasPrice := big.NewInt(4)
	data := []byte{5}
	tx := NewTokenTransaction(to, nonce, to, amount, gasLimit, gasPrice, data)

	censor.On("Block").Return(&Block{
		Header: &Header{
			GasLimit: 0,
		},
	}).Once()

	err := tx.CheckState(censor)
	assert.Equal(t, ErrGasLimit, err)

	censor.On("Block", mock.Anything).Return(&Block{
		Header: &Header{
			GasLimit: ParGasLimit,
		},
	})

	err = tx.CheckState(censor)
	assert.Equal(t, ErrInvalidSender, err)

	key, err := crypto.GenerateKey()
	assert.Nil(t, err)

	err = tx.Sign(GlobalSTDSigner, key)
	assert.Nil(t, err)

	z0Addr := common.HexToAddress("0x0")
	tx.StoreFrom(z0Addr)

	state.On("Exist", mock.Anything).Return(false).Once()
	state.On("GetNonce", mock.Anything).Return(tx.Nonce()).Once()
	state.On("GetTokenBalance", mock.Anything, mock.Anything).Return(big.NewInt(1e12)).Once()
	state.On("GetBalance", mock.Anything, mock.Anything).Return(big.NewInt(1e12)).Once()

	state.On("SubBalance", mock.Anything, mock.Anything).Return()
	state.On("SubTokenBalance", mock.Anything, mock.Anything, mock.Anything).Return()
	state.On("SetNonce", mock.Anything, mock.Anything).Return()

	err = tx.CheckState(censor)

	state.On("Exist", mock.Anything).Return(true)
	state.On("GetNonce", mock.Anything).Return(tx.Nonce() + 1).Once()
	err = tx.CheckState(censor)
	assert.Equal(t, ErrNonceTooLow, err)

	state.On("GetNonce", mock.Anything).Return(tx.Nonce() - 1).Once()
	err = tx.CheckState(censor)
	assert.Equal(t, ErrNonceTooHigh, err)

	state.On("GetNonce", mock.Anything).Return(tx.Nonce())
	state.On("GetTokenBalance", mock.Anything, mock.Anything).Return(big.NewInt(0).Sub(tx.Value(), big.NewInt(1))).Once()
	err = tx.CheckState(censor)
	assert.Equal(t, ErrInsufficientFunds, err)

	state.On("GetTokenBalance", mock.Anything, mock.Anything).Return(tx.Value())
	state.On("GetBalance", mock.Anything).Return(new(big.Int).Sub(tx.GasCost(), big.NewInt(1))).Once()
	err = tx.CheckState(censor)
	assert.Equal(t, ErrInsufficientFunds, err)

	state.On("GetBalance", mock.Anything).Return(tx.GasCost()).Once()
	err = tx.CheckState(censor)
	assert.Nil(t, err)
}
