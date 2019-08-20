package ethapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/crypto"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	"github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)


func TestGetTransactionBy(t *testing.T) {
	b := &MockBackend{}
	s := NewPublicTransactionPoolAPI(b, nil)

	assert := assert.New(t)

	txs, txsEntry := getTestTxs()

	block := &types.Block{
		Header: &types.Header{
			Height:         1,
			ValidatorsHash: common.HexToHash("0x1"),
		},
		Data: &types.Data{
			Txs: txs,
		},
		LastCommit: &types.Commit{},
	}

	for idx, tx := range txs {
		entry := txsEntry[idx]
		b.On("GetTx", mock.Anything).Return(tx, &entry).Once()
		v := s.GetTransactionByHash(nil, tx.Hash())
		rpcTx := v.(*rtypes.RPCTx)
		assert.Equal(rpcTx.TxHash, tx.Hash(), "not equal")

		b.On("GetBlock", mock.Anything, mock.Anything).Return(block, nil).Once()
		v = s.GetTransactionByBlockHashAndIndex(nil, block.Hash(), hexutil.Uint(uint(idx)))
		rpcTx = v.(*rtypes.RPCTx)
		assert.Equal(rpcTx.TxHash, tx.Hash(), "not equal")

		b.On("BlockByNumber", mock.Anything, mock.Anything).Return(block, nil).Once()
		v = s.GetTransactionByBlockNumberAndIndex(nil, rpc.BlockNumber(block.Height), hexutil.Uint(uint(idx)))
		rpcTx = v.(*rtypes.RPCTx)
		assert.Equal(rpcTx.TxHash, tx.Hash(), "not equal")

		if tx.TypeName() == types.TxNormal || tx.TypeName() == types.TxToken {
			serBytes, _ := ser.EncodeToBytes(tx)
			txBytes := hexutil.Bytes(serBytes)

			b.On("GetTx", mock.Anything).Return(tx, &entry).Once()
			bs, err := s.GetRawTransactionByHash(nil, tx.Hash())
			assert.Nil(err, "error")
			assert.Equal(bs, txBytes, "not equal")

			b.On("GetBlock", mock.Anything, mock.Anything).Return(block, nil).Once()
			bs = s.GetRawTransactionByBlockHashAndIndex(nil, block.Hash(), hexutil.Uint(uint(idx)))
			assert.Equal(bs, txBytes, "not equal")

			b.On("BlockByNumber", mock.Anything, mock.Anything).Return(block, nil).Once()
			bs = s.GetRawTransactionByBlockNumberAndIndex(nil, rpc.BlockNumber(block.Height), hexutil.Uint(uint(idx)))
			assert.Equal(bs, txBytes, "not equal")
		}

		// bs, err := json.Marshal(v)
		// if err != nil {
		// 	panic(err)
		// }
		// fmt.Printf("%s,\n", bs)
	}

	b.On("BlockByNumber", mock.Anything, mock.Anything).Return(block, nil).Once()
	count := s.GetBlockTransactionCountByNumber(nil, rpc.BlockNumber(block.Height))
	assert.Equal(count.String(), hexutil.Uint(uint(len(txs))).String(), "not equal")

	b.On("GetBlock", mock.Anything, mock.Anything).Return(block, nil).Once()
	count = s.GetBlockTransactionCountByHash(nil, block.Hash())
	assert.Equal(count.String(), hexutil.Uint(uint(len(txs))).String(), "not equal")
}

func TestSignTransaction(t *testing.T) {
	b := &MockBackend{}
	s := NewPublicTransactionPoolAPI(b, nil)

	dir, err := ioutil.TempDir("", "eth-keystore-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	am, addrs := testAccountManager(dir)
	a := addrs[0]
	b.On("AccountManager").Return(am)

	assert := assert.New(t)

	ret, err := s.SignTransaction(nil, rtypes.SendTxArgs{})
	assert.Nil(ret, "not nil")
	assert.Equal(err.Error(), "gas not specified", "not equal")

	uint1 := hexutil.Uint64(1)
	ret, err = s.SignTransaction(nil, rtypes.SendTxArgs{Gas: &uint1})
	assert.Nil(ret, "not nil")
	assert.Equal(err.Error(), "gasPrice not specified", "not equal")

	ret, err = s.SignTransaction(nil, rtypes.SendTxArgs{Gas: &uint1, GasPrice: (*hexutil.Big)(big.NewInt(1))})
	assert.Nil(ret, "not nil")
	assert.Equal(err.Error(), "nonce not specified", "not equal")

	ret, err = s.SignTransaction(nil, rtypes.SendTxArgs{Gas: &uint1, GasPrice: (*hexutil.Big)(big.NewInt(1)), Nonce: &uint1})
	assert.Nil(ret, "not nil")
	assert.Equal(err, accounts.ErrUnknownAccount)

	ret, err = s.SignTransaction(nil, rtypes.SendTxArgs{Gas: &uint1, GasPrice: (*hexutil.Big)(big.NewInt(1)), Nonce: &uint1, To: &common.EmptyAddress})
	assert.Nil(ret, "not nil")
	assert.Equal(err, accounts.ErrUnknownAccount)

	sp := NewPrivateAccountAPI(b, new(AddrLocker))
	seconds := uint64(3600)
	sp.UnlockAccount(a.Address, "1234", &seconds)

	ret, err = s.SignTransaction(nil, rtypes.SendTxArgs{From: a.Address, Gas: &uint1, GasPrice: (*hexutil.Big)(big.NewInt(1)), Nonce: &uint1, To: &common.EmptyAddress, Value: (*hexutil.Big)(big.NewInt(-1))})
	assert.Nil(ret, "not nil")
	assert.Equal(err.Error(), "rlp: cannot encode negative *big.Int")

	ret, err = s.SignTransaction(nil, rtypes.SendTxArgs{From: a.Address, Gas: &uint1, GasPrice: (*hexutil.Big)(big.NewInt(1)), Nonce: &uint1, To: &common.EmptyAddress})
	assert.NotNil(ret, "not nil")
	assert.Nil(err, "not nil")
}

func TestSignSpecTx(t *testing.T) {
	b := &MockBackend{}
	s := NewPublicTransactionPoolAPI(b, nil)

	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "eth-keystore-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	am, addrs := testAccountManager(dir)
	b.On("AccountManager").Return(am)
	a1 := addrs[0].Address
	a2 := addrs[1].Address
	a3 := addrs[2].Address

	sp := NewPrivateAccountAPI(b, new(AddrLocker))
	seconds := uint64(3600)
	sp.UnlockAccount(a1, "1234", &seconds)
	sp.UnlockAccount(a2, "1234", &seconds)
	sp.UnlockAccount(a3, "1234", &seconds)

	signersInfo := &types.SignersInfo{MinSignerPower: 3}
	signers := make([]*types.SignerEntry, 0, 3)
	for _, addr := range []common.Address{a1, a2, a3} {
		signers = append(signers, &types.SignerEntry{Power: 10, Addr: addr})
	}
	signersInfo.Signers = signers
	type specTx interface {
		VerifySign(signersInfo *types.SignersInfo) error
	}

	strArgs := []string{
		`{"type":"cct","signers":["%s","%s","%s"],"args":{ "from":"%s", "nonce":"0x3", "value":"0x4", "data":"0x48656c6c6f20576f726c6421", "gas":"0x7", "gasPrice":"0x8"} }`,
		`{"type":"cut","signers":["%s","%s","%s"],"args":{ "from":"%s", "contract":"0x0000000000000000000000000000000000000004","nonce":"0x3", "value":"0x4", "data":"0x48656c6c6f20576f726c6421"} }`,
	}
	for _, str := range strArgs {
		str = fmt.Sprintf(str, a1.Hex(), a2.Hex(), a3.Hex(), a1.Hex())
		args := rtypes.SendSpecTxArgs{}
		if err := json.Unmarshal([]byte(str), &args); err != nil {
			t.Fatalf("json.Unmarshal err:%v", err)
		}

		ret, err := s.SignSpecTx(nil, args)
		assert.NotNil(ret, "not nil")
		assert.Nil(err, "not nil")
		stx, ok := ret.Tx.(specTx)
		if !ok {
			t.Fatalf("ret is not a VerifySign tx:%v", ret)
		}
		err = stx.VerifySign(signersInfo)
		assert.Nil(err, "not nil")
	}

}

func TestSign(t *testing.T) {
	b := &MockBackend{}
	s := NewPublicTransactionPoolAPI(b, nil)

	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "eth-keystore-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	am, addrs := testAccountManager(dir)
	a := addrs[0]
	b.On("AccountManager").Return(am)
	ret, err := s.Sign(common.EmptyAddress, []byte{})
	assert.Nil(ret, "not nil")
	assert.Equal(err, accounts.ErrUnknownAccount)

	sp := NewPrivateAccountAPI(b, new(AddrLocker))
	seconds := uint64(3600)
	sp.UnlockAccount(a.Address, "1234", &seconds)
	ret, err = s.Sign(a.Address, common.EmptyHash.Bytes())
	assert.NotNil(ret, "not nil")
	assert.Nil(err, "not nil")
}

func TestGetTransactionCount(t *testing.T) {
	b := &MockBackend{}
	s := NewPublicTransactionPoolAPI(b, nil)

	assert := assert.New(t)

	b.On("StateAndHeaderByNumber", mock.Anything, mock.Anything).Return(func(ctx context.Context, blockNr rpc.BlockNumber) *state.StateDB {
		return nil
	}, nil, errors.New("err")).Once()
	ret, err := s.GetTransactionCount(nil, common.EmptyAddress, rpc.BlockNumber(1))
	assert.NotNil(err, "not nil")
	assert.Nil(ret, "not nil")

	b.On("StateAndHeaderByNumber", mock.Anything, mock.Anything).Return(func(ctx context.Context, blockNr rpc.BlockNumber) *state.StateDB {
		s, _ := state.New(common.EmptyHash, state.NewDatabase(dbm.NewMemDB()))
		s.SetNonce(common.EmptyAddress, 1)
		return s
	}, nil, nil)
	ret, err = s.GetTransactionCount(nil, common.EmptyAddress, rpc.BlockNumber(1))
	assert.Nil(err, "not nil")
	assert.Equal(ret.String(), hexutil.Uint64(1).String(), "not equal")
}

func TestGetTransactionReceipt(t *testing.T) {
	b := &MockBackend{}
	s := NewPublicTransactionPoolAPI(b, nil)

	assert := assert.New(t)

	b.On("GetTx", mock.Anything).Return(nil, nil).Once()
	ret, err := s.GetTransactionReceipt(nil, common.EmptyHash)
	assert.Nil(ret, "not nil")
	assert.Nil(err, "not nil")

	b.On("GetTx", mock.Anything).Return(&types.Transaction{}, &types.TxEntry{}).Once()
	b.On("GetReceipts", mock.Anything, mock.Anything).Return(types.Receipts{}).Once()
	ret, err = s.GetTransactionReceipt(nil, common.EmptyHash)
	assert.Nil(ret, "not nil")
	assert.Nil(err, "not nil")

	key, _ := crypto.GenerateKey()
	tokenTx := types.NewTokenTransaction(common.EmptyAddress, 0, common.EmptyAddress, big.NewInt(0), 0, big.NewInt(0), nil)
	err = tokenTx.Sign(types.GlobalSTDSigner, key)
	if err != nil {
		panic(err)
	}

	receipts := types.Receipts{&types.Receipt{PostState: []byte{}}}
	b.On("GetTx", mock.Anything).Return(tokenTx, &types.TxEntry{Index: 0}).Once()
	b.On("GetReceipts", mock.Anything, mock.Anything).Return(receipts).Once()
	ret, err = s.GetTransactionReceipt(nil, common.EmptyHash)
	assert.Equal(ret["to"], &common.EmptyAddress, "not nil")
	assert.Nil(err, "not nil")
}

func TestSendRawTokenTransaction(t *testing.T) {
	assert := assert.New(t)
	assert.Equal(nil, nil)

	b := &MockBackend{}
	s := NewPublicTransactionPoolAPI(b, nil)

	key, _ := crypto.GenerateKey()
	tokenTx := types.NewTokenTransaction(common.EmptyAddress, 0, common.EmptyAddress, big.NewInt(0), 0, big.NewInt(0), nil)
	err := tokenTx.Sign(types.GlobalSTDSigner, key)
	if err != nil {
		panic(err)
	}

	hash := tokenTx.Hash()
	raw, _ := ser.EncodeToBytes(tokenTx)

	b.On("SendTx", mock.Anything, mock.Anything).Return(nil).Once()
	ret, err := s.SendRawTx(nil, raw, types.TxToken)
	assert.Equal(hash, ret)
	assert.Equal(nil, err)

	testErr := errors.New("test err")
	b.On("SendTx", mock.Anything, mock.Anything).Return(testErr).Once()
	ret, err = s.SendRawTx(nil, raw, types.TxToken)
	assert.Equal(common.EmptyHash, ret)
	assert.Equal(testErr, err)
}

func TestSendTokenTransaction(t *testing.T) {
	assert := assert.New(t)
	assert.Equal(nil, nil)

	b := &MockBackend{}
	s := NewPublicTransactionPoolAPI(b, nil)

	dir, err := ioutil.TempDir("", "eth-keystore-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	am, addrs := testAccountManager(dir)
	a := addrs[0]
	to := common.HexToAddress("0x01")
	b.On("AccountManager").Return(am).Times(10)
	b.On("SendTx", mock.Anything, mock.Anything).Return(nil)

	tokenAddress := common.HexToAddress("0x08")
	tx := rtypes.SendTxArgs{
		From:         a.Address,
		To:           &to,
		Gas:          (*hexutil.Uint64)(new(uint64)),
		GasPrice:     (*hexutil.Big)(big.NewInt(10)),
		Value:        (*hexutil.Big)(big.NewInt(11)),
		Nonce:        (*hexutil.Uint64)(new(uint64)),
		TokenAddress: tokenAddress,
	}

	_, err = s.SendTransaction(nil, tx)
	assert.Equal(keystore.ErrLocked, err)

	sp := NewPrivateAccountAPI(b, new(AddrLocker))
	seconds := uint64(3600)
	sp.UnlockAccount(a.Address, "1234", &seconds)
	_, err = s.SendTransaction(nil, tx)
	assert.Equal(nil, err)
}

func testAccountManager(dir string) (*accounts.Manager, []accounts.Account) {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP

	os.MkdirAll(dir, 0777)
	ks := keystore.NewKeyStore(dir, scryptN, scryptP)
	as := ks.Accounts()

	for i := len(as); i < 3; i++ {
		_, err := ks.NewAccount("1234", "cuetest")
		if err != nil {
			panic(err)
		}
	}

	as = ks.Accounts()
	backends := []accounts.Backend{ks}
	return accounts.NewManager(backends...), as
}

// GetTransactionByBlockHashAndIndex
// GetRawTransactionByBlockNumberAndIndex
// GetRawTransactionByBlockHashAndIndex
// GetTransactionByHash
// GetRawTransactionByHash
// GetTransactionReceipt
