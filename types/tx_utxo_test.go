package types

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/common"
	lktypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	//"github.com/lianxiangcloud/linkchain/libs/crypto"
    "github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var skey *ecdsa.PrivateKey
var addr common.Address
var amount = big.NewInt(1e18)

func init() {
	skey, addr = defaultTestKey()
}

func getUTXOTx(t *testing.T) *UTXOTransaction {
	accountSource := &AccountSourceEntry{
		From:   addr,
		Nonce:  1,
		Amount: amount,
	}
    transferGas := CalNewAmountGas(amount, EverLiankeFee)
    transferFee := big.NewInt(0).Mul(big.NewInt(ParGasPrice), big.NewInt(0).SetUint64(transferGas))
	utxoDest := &UTXODestEntry{
		Addr:   lktypes.AccountAddress{},
		Amount: big.NewInt(0).Sub(amount, transferFee),
	}
	dest := []DestEntry{utxoDest}

	utxoTx, _, err := NewAinTransaction(accountSource, dest, common.EmptyAddress, nil)
	require.Nil(t, err)
	err = utxoTx.Sign(GlobalSTDSigner, skey)
	require.Nil(t, err)

	kind := utxoTx.UTXOKind()
	assert.Equal(t, Ain|Uout, kind)
	return utxoTx
}

func TestAsMessage(t *testing.T) {
	accountUTXOTx := getUTXOTx(t)
    log.Debug("TestAsMessage", "accountUTXOTx", accountUTXOTx)
	fromAddr, err := accountUTXOTx.From()
	require.Nil(t, err)
	assert.Equal(t, fromAddr, addr)
	var msg Message
	msg, err = accountUTXOTx.AsMessage()
	require.Nil(t, err)
	state := &MockState{}
	state.On("IsContract", mock.Anything).Return(false)
    msgFrom, err := msg.From(accountUTXOTx, state)
    require.Nil(t, err)
    assert.Equal(t, fromAddr, msgFrom)
	assert.Equal(t, amount, msg.Value())
	assert.Equal(t, accountUTXOTx.TxType(), msg.TxType())
    assert.Equal(t, uint64(1), msg.Nonce())
    assert.Equal(t, 0, len(msg.OutputData()))
}

func TestCheck(t *testing.T) {
	accountUTXOTx := getUTXOTx(t)

	censor := &MockTxCensor{}
	state := &MockState{}
	censor.On("State").Return(state)
	censor.On("LockState").Return()
	censor.On("UnlockState").Return()
	state.On("IsContract", mock.Anything).Return(false)
    
    err := accountUTXOTx.checkTxSemantic(censor)    
    require.Nil(t, err)

    err = accountUTXOTx.CheckBasic(censor)
    require.Nil(t, err)
}
