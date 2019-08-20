package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

func TestContractUpgrade(t *testing.T) {
	key1, err := crypto.GenerateKey()
	assert.Nil(t, err)
	key2, err := crypto.GenerateKey()
	assert.Nil(t, err)
	addr1 := crypto.PubkeyToAddress(key1.PublicKey)
	addr2 := crypto.PubkeyToAddress(key2.PublicKey)

	contractAddr := common.HexToAddress("0x1")
	nonce := uint64(2)
	payload := []byte{5}
	mainInfo := &ContractUpgradeMainInfo{
		FromAddr:     addr1,
		Recipient:    contractAddr,
		AccountNonce: nonce,
		Payload:      payload,
	}

	_, err = SignContractUpgradeTx(nil, mainInfo)
	assert.Equal(t, err, ErrParams)
	_, err = SignContractUpgradeTx(key1, mainInfo)
	assert.Nil(t, err)

	tx := UpgradeContractTx(nil, nil)
	assert.Nil(t, tx)

	tx = UpgradeContractTx(mainInfo, nil)
	assert.NotNil(t, tx)

	from, err := tx.From()
	assert.Nil(t, err)
	assert.Equal(t, addr1, from)

	assert.Equal(t, contractAddr, *tx.To())

	assert.Equal(t, "0", tx.Cost().String())

	err = tx.Sign(GlobalSTDSigner, key1)
	assert.Nil(t, err)

	err = tx.Sign(GlobalSTDSigner, key2)
	assert.Nil(t, err)

	addrs, err := tx.Senders()
	assert.Nil(t, err)
	assert.Equal(t, addr1, addrs[0])
	assert.Equal(t, addr2, addrs[1])

	assert.Equal(t, TxContractUpgrade, tx.TypeName())
	_ = tx.String()

	assert.Equal(t, TxContractCreateType, tx.txType())
	assert.Equal(t, payload, tx.Data())

	imsg, err := tx.AsMessage()
	assert.Nil(t, err)

	imsg, err = imsg.AsMessage()
	assert.Nil(t, err)

	bs, err := ser.EncodeToBytes(tx)
	assert.Nil(t, err)

	tx2 := new(ContractUpgradeTx)
	err = ser.DecodeBytes(bs, tx2)
	assert.Nil(t, err)

	assert.Equal(t, tx.Hash(), tx2.Hash())
}

func TestContractUpgradeCheckBasic(t *testing.T) {
	censor := &MockTxCensor{}
	txMgr := &MockTxMgr{}
	censor.On("TxMgr").Return(txMgr)

	var tx *ContractUpgradeTx
	err := tx.CheckBasic(censor)
	assert.Equal(t, ErrTxEmpty, err)

	tx = new(ContractUpgradeTx)
	err = tx.CheckBasic(censor)
	assert.Equal(t, ErrParams, err)

	tx.ContractUpgradeMainInfo.Payload = []byte{1}
	censor.On("IsWasmContract", mock.Anything).Return(false).Once()
	err = tx.CheckBasic(censor)
	assert.Equal(t, "UpgardeCOntractTx: Payload not wasm", err.Error())

	censor.On("IsWasmContract", mock.Anything).Return(true)
	tx.FromAddr = common.HexToAddress("0x1")

	tx.FromAddr = common.HexToAddress("0x0")
	tx = &ContractUpgradeTx{ContractUpgradeMainInfo: tx.ContractUpgradeMainInfo}

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

	txMgr.On("GetMultiSignersInfo", mock.Anything).Return(signersInfo).Once()
	err = tx.CheckBasic(censor)
	assert.NotNil(t, err)

	txMgr.On("GetMultiSignersInfo", mock.Anything).Return(signersInfo).Once()
	err = tx.CheckBasic(censor)
	assert.NotNil(t, err)

	txMgr.On("GetMultiSignersInfo", mock.Anything).Return(signersInfo).Once()
	tx = &ContractUpgradeTx{ContractUpgradeMainInfo: tx.ContractUpgradeMainInfo}
	err = tx.CheckBasic(censor)
	assert.NotNil(t, err)

	err = tx.Sign(GlobalSTDSigner, key1)
	assert.Nil(t, err)
	txMgr.On("GetMultiSignersInfo", mock.Anything).Return(signersInfo).Once()
	err = tx.CheckBasic(censor)
	assert.NotNil(t, err)

	contractAddr := common.HexToAddress("0x1")
	nonce := uint64(2)
	payload := []byte{5}
	mainInfo := &ContractUpgradeMainInfo{
		FromAddr:     addr1,
		Recipient:    contractAddr,
		AccountNonce: nonce,
		Payload:      payload,
	}
	tx = UpgradeContractTx(mainInfo, nil)

	err = tx.Sign(GlobalSTDSigner, key1)
	assert.Nil(t, err)

	err = tx.Sign(GlobalSTDSigner, key2)
	assert.Nil(t, err)

	var tx2 *ContractUpgradeTx
	err = tx2.VerifySign(signersInfo)
	assert.NotNil(t, err)

	tx2 = new(ContractUpgradeTx)
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

func TestContractUpgradeCheckState(t *testing.T) {
	censor := &MockTxCensor{}
	state := &MockState{}
	censor.On("State").Return(state)
	censor.On("LockState").Return()
	censor.On("UnlockState").Return()

	addr := common.HexToAddress("0x01")
	contractAddr := addr
	nonce := uint64(2)
	payload := []byte{5}
	mainInfo := &ContractUpgradeMainInfo{
		FromAddr:     addr,
		Recipient:    contractAddr,
		AccountNonce: nonce,
		Payload:      payload,
	}
	tx := UpgradeContractTx(mainInfo, nil)

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
