// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

// The values in those tests are from the Transaction Tests
// at github.com/ethereum/tests.
var (
	emptyTx = NewTransaction(
		0,
		common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87"),
		big.NewInt(0), 0, big.NewInt(0),
		nil,
	)

	rightvrsTx, _ = NewTransaction(
		3,
		common.HexToAddress("b94f5374fce5edbc8e2a8697c15331677e6ebf0b"),
		big.NewInt(10),
		2000,
		big.NewInt(1),
		common.FromHex("5544"),
	).WithSignature(
		STDHomesteadSigner{},
		common.Hex2Bytes("98ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4a8887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a301"),
	)
)

func TestTransactionSigHash(t *testing.T) {
	var homestead STDHomesteadSigner
	emptyTx.data.GasLimit = 0
	emptyTx.data.Price = big.NewInt(0)
	rightvrsTx.data.GasLimit = 2000
	rightvrsTx.data.Price = big.NewInt(1)

	if homestead.Hash(&emptyTx.data) != common.HexToHash("c775b99e7ad12f50d819fcd602390467e28141316969f4b57f0626f74fe3b386") {
		t.Errorf("empty transaction hash mismatch, got %x", emptyTx.Hash())
	}
	if homestead.Hash(&rightvrsTx.data) != common.HexToHash("fe7a79529ed5f7c3375d06b26b186a8644e0e16c373d7a12be41c62d6042b77a") {
		t.Errorf("RightVRS transaction hash mismatch, got %x", rightvrsTx.Hash())
	}
}

func TestTransactionEncode(t *testing.T) {
	rightvrsTx.data.GasLimit = 2000
	rightvrsTx.data.Price = big.NewInt(1)

	bf := bytes.NewBuffer(nil)
	err := rightvrsTx.EncodeSER(bf)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	txb := bf.Bytes()
	should := common.FromHex("f86103018207d094b94f5374fce5edbc8e2a8697c15331677e6ebf0b0a8255441ca098ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4aa08887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a3")
	if !bytes.Equal(txb, should) {
		t.Errorf("encoded ser mismatch, got %x", txb)
	}
}

func decodeTx(data []byte) (*Transaction, error) {
	var tx = new(Transaction)
	err := tx.DecodeSER(ser.NewStream(bytes.NewReader(data), 0))

	return tx, err
}

func defaultTestKey() (*ecdsa.PrivateKey, common.Address) {
	key, _ := crypto.HexToECDSA("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return key, addr
}

func TestRecipientEmpty(t *testing.T) {
	_, addr := defaultTestKey()
	tx, err := decodeTx(common.Hex2Bytes("f8498080808080011ca09b16de9d5bdee2cf56c28d16275a4da68cd30273e2525f3959f5d62557489921a0372ebd8fb3345f7db7b5a86d42e24d36e983e259b0664ceb8c227ec9af572f3d"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	from, err := tx.Sender(STDHomesteadSigner{})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if addr != from {
		t.Error("derived address doesn't match")
	}
}

func TestRecipientNormal(t *testing.T) {
	_, addr := defaultTestKey()

	tx, err := decodeTx(common.Hex2Bytes("f85d80808094000000000000000000000000000000000000000080011ca0527c0d8f5c63f7b9f41324a7c8a563ee1190bcbf0dac8ab446291bdbf32f5c79a0552c4ef0a09a04395074dab9ed34d3fbfb843c2f2546cc30fe89ec143ca94ca6"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	from, err := tx.Sender(STDHomesteadSigner{})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if addr != from {
		t.Error("derived address doesn't match")
	}
}

// TestTransactionJSON tests serializing/de-serializing to/from JSON.
func TestTransactionJSON(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("could not generate key: %v", err)
	}
	signer := NewSTDEIP155Signer(common.Big1)

	for i := uint64(0); i < 25; i++ {
		var tx *Transaction
		switch i % 2 {
		case 0:
			tx = NewTransaction(i, common.Address{1}, common.Big0, 1, common.Big2, []byte("abcdef"))
		case 1:
			tx = NewContractCreation(i, common.Big0, 1, common.Big2, []byte("abcdef"))
		}

		err := tx.Sign(signer, key)
		if err != nil {
			t.Fatalf("could not sign transaction: %v", err)
		}

		data, err := json.Marshal(tx)
		if err != nil {
			t.Errorf("json.Marshal failed: %v", err)
		}

		var parsedTx *Transaction
		if err := json.Unmarshal(data, &parsedTx); err != nil {
			t.Errorf("json.Unmarshal failed: %v", err)
		}

		// compare nonce, price, gaslimit, recipient, amount, payload, V, R, S
		if tx.Hash() != parsedTx.Hash() {
			t.Errorf("parsed tx differs from original tx, want %v, got %v", tx, parsedTx)
		}
		if tx.SignParam().Cmp(parsedTx.SignParam()) != 0 {
			t.Errorf("invalid chain id, want %d, got %d", tx.SignParam(), parsedTx.SignParam())
		}
	}
}

func TestTransaction(t *testing.T) {
	nonce := uint64(0)
	to := common.HexToAddress("0x1")
	amount := big.NewInt(2)
	gasLimit := uint64(3)
	gasPrice := big.NewInt(4)
	data := []byte{5}
	tx := NewTransaction(nonce, to, amount, gasLimit, gasPrice, data)

	key, err := crypto.GenerateKey()
	assert.Nil(t, err)

	addr := crypto.PubkeyToAddress(key.PublicKey)

	err = tx.Sign(GlobalSTDSigner, key)
	assert.Nil(t, err)

	from, err := tx.From()
	assert.Nil(t, err)
	assert.Equal(t, addr, from)

	assert.Equal(t, GlobalSTDSigner.SignParam().String(), tx.SignParam().String())
	assert.Equal(t, TxNormal, tx.TypeName())

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

	tx4 := new(Transaction)
	err = tx4.DecodeSER(ser.NewStream(bf, 0))
	assert.Nil(t, err)
	assert.Equal(t, tx.Hash(), tx4.Hash())

	bs, err = json.Marshal(tx)
	assert.Nil(t, err)

	tx5 := new(Transaction)
	err = json.Unmarshal(bs, tx5)
	assert.Nil(t, err)
	assert.Equal(t, tx.Hash(), tx5.Hash())

	assert.Equal(t, tx.data, tx.GetTxData())
	assert.Equal(t, common.EmptyAddress, tx.TokenAddress())
	assert.Equal(t, data, tx.Data())
	assert.Equal(t, gasLimit, tx.Gas())
	assert.Equal(t, ParGasPrice, tx.GasPrice().Int64())
	assert.Equal(t, amount.String(), tx.Value().String())
	assert.Equal(t, nonce, tx.Nonce())
	assert.Equal(t, to, *tx.To())

	tx.Size()

	_, err = tx.AsMessage()
	assert.Nil(t, err)

	fields := append(tx.data.signFields(), tx.SignParam(), uint(0), uint(0))
	h := rlpHash(fields)
	sig, err := crypto.Sign(h[:], key)
	assert.Nil(t, err)
	tx6, err := tx.WithSignature(GlobalSTDSigner, sig)
	assert.Nil(t, err)
	from, err = tx6.From()
	assert.Nil(t, err)
	assert.Equal(t, addr, from)

	assert.Equal(t, h, tx.SignHash())

	cost := new(big.Int).Mul(tx.GasPrice(), big.NewInt(int64(tx.Gas())))
	cost = cost.Add(cost, tx.Value())
	assert.Equal(t, cost.String(), tx.Cost().String())

	v, r, s := tx.data.V, tx.data.R, tx.data.S
	v1, r1, s1 := tx.RawSignatureValues()
	assert.Equal(t, v.String(), v1.String())
	assert.Equal(t, r.String(), r1.String())
	assert.Equal(t, s.String(), s1.String())

	_ = tx.String()
}

func TestTransactionCheckBasic(t *testing.T) {
	censor := &MockTxCensor{}
	state := &MockState{}
	censor.On("State").Return(state)
	censor.On("LockState").Return()
	censor.On("UnlockState").Return()
	state.On("IsContract", mock.Anything).Return(false)

	to := common.HexToAddress("0x01")
	var tx *Transaction
	err := tx.CheckBasic(censor)
	assert.Equal(t, ErrTxEmpty, err)

	tx = new(Transaction)
	censor.On("NodeType").Return("")
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

	tx = &Transaction{data: tx.data}
	tx.data.Payload = nil
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrGasLimitOrGasPrice, err)

	tx.data.GasLimit = ParGasLimit
	tx.data.Price = big.NewInt(ParGasPrice)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrGasLimitOrGasPrice, err)

	z1Addr := common.HexToAddress("0x1")
	tx.StoreFrom(z1Addr)
	err = tx.CheckBasic(censor)

	tx = &Transaction{data: tx.data}
	z0Addr := common.HexToAddress("0x0")
	tx.StoreFrom(z0Addr)

	tx.data.Payload = make([]byte, 32000)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrGasLimitOrGasPrice, err)

	tx.data.GasLimit = ParGasLimit - 1
	tx.data.Payload = []byte{0}
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrGasLimitOrGasPrice, err)

	tx.data.GasLimit = ParGasLimit
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrGasLimitOrGasPrice, err)

	tx.data.Payload = nil
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrGasLimitOrGasPrice, err)
}

func TestTransactionCheckState(t *testing.T) {
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
	tx := NewTransaction(nonce, to, amount, gasLimit, gasPrice, data)

	err := tx.CheckState(censor)
	assert.Equal(t, ErrInvalidSender, err)

	key, err := crypto.GenerateKey()
	assert.Nil(t, err)

	err = tx.Sign(GlobalSTDSigner, key)
	assert.Nil(t, err)

	state.On("GetNonce", mock.Anything).Return(tx.Nonce() - 1).Once()
	z1Addr := common.HexToAddress("0x1")
	tx.StoreFrom(z1Addr)
	err = tx.CheckState(censor)

	state.On("GetNonce", mock.Anything).Return(tx.Nonce()).Once()
	state.On("GetBalance", mock.Anything).Return(big.NewInt(1e10)).Once()
	state.On("Exist", mock.Anything).Return(false).Once()
	err = tx.CheckState(censor)
	assert.Equal(t, ErrInsufficientFunds, err)

	state.On("Exist", mock.Anything).Return(true)
	state.On("GetNonce", mock.Anything).Return(tx.Nonce() + 1).Once()
	err = tx.CheckState(censor)
	assert.Equal(t, ErrNonceTooLow, err)

	state.On("GetNonce", mock.Anything).Return(tx.Nonce() - 1).Once()
	err = tx.CheckState(censor)
	assert.Equal(t, ErrNonceTooHigh, err)

	state.On("GetNonce", mock.Anything).Return(tx.Nonce())
	cost := tx.Cost()
	state.On("GetBalance", mock.Anything).Return(big.NewInt(0).Sub(cost, big.NewInt(1))).Once()
	err = tx.CheckState(censor)
	assert.Equal(t, ErrInsufficientFunds, err)

	state.On("GetBalance", mock.Anything).Return(cost)
	state.On("SubBalance", mock.Anything, mock.Anything).Return()
	state.On("SetNonce", mock.Anything, mock.Anything).Return()
	err = tx.CheckState(censor)
	assert.Nil(t, err)
}

func TestMessage(t *testing.T) {
	from := common.HexToAddress("0x1")
	to := common.HexToAddress("0x2")
	tokenAddr := common.HexToAddress("0x3")
	nonce := uint64(4)
	amount := big.NewInt(5)
	gasLimit := uint64(6)
	gasPrice := big.NewInt(7)
	data := []byte{8}
	msg := NewMessage(from, &to, tokenAddr, nonce, amount, gasLimit, gasPrice, data)
	msg.txType = TxNormal

	assert.Equal(t, from, msg.MsgFrom())
	assert.Equal(t, to, *msg.To())
	assert.Equal(t, gasPrice.String(), msg.GasPrice().String())
	casCost := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))
	assert.Equal(t, casCost.String(), msg.GasCost().String())
	assert.Equal(t, amount.String(), msg.Value().String())
	assert.Equal(t, gasLimit, msg.Gas())
	assert.Equal(t, nonce, msg.Nonce())
	assert.Equal(t, data, msg.Data())
	assert.Equal(t, tokenAddr, msg.TokenAddress())
	_, err := msg.AsMessage()
	assert.Nil(t, err)
	assert.Equal(t, TxNormal, msg.TxType())
	msg.SetTxType(TxToken)
	assert.Equal(t, TxToken, msg.TxType())
}
