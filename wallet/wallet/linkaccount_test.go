package wallet

import (
	"fmt"
	"math/big"
	"testing"

	. "github.com/bouk/monkey"
	"github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/wallet/types"
	. "github.com/prashantv/gostub"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
)

var (
	mockEthAddr  = common.HexToAddress("0xa73810e519e1075010678d706533486d8ecc8000")
	mockTokenA   = common.HexToAddress("0x000000000000000000000000000000000000000a")
	mockTokenB   = common.HexToAddress("0x000000000000000000000000000000000000000b")
	utxoAccount0 = "B82MtaMMExz8oPwtstBddpgLoMvT2YmeU79z2A8ZMbf4hxvV2GFUrwPKmT6ko4YgTwMWEmNT1tFDg3DcTSNydftUHKxmEUt"
	utxoAccount1 = "ESuckthZTpjTsqVobB1Yg7SkbU1QUGFThPko8hkFP9VnQZ8WaVaLe4siM1r7tdKkrnFWXkHxZKPuj2gVjd6KZeoo1kWqvG4"
	utxoAccount2 = "EStq1qHQqfp6jjmUuNcDyQAh4tWe6nBdWj3zzzvkvL1vVriaKZFQZCZR73SBwCVBRXLtG8zvC5Uq1LePaD59CeZf85eLFU2"

	mockLinkAccount *LinkAccount
)

func resetMockAccount() {
	var err error

	mockLinkAccount, err = newTestLinkAccount()
	// assert.Equal(err, nil, "newTestLinkAccount fail")
	if err != nil {
		panic("newTestLinkAccount fail")
	}
}

func newTestStateDB() dbm.DB {
	return dbm.NewMemDB()
}

func newTestLogger() log.Logger {
	logger, _ := log.ParseLogLevel("*:error", log.Root(), "info")
	return logger
}

func newTestKeyFile() string {
	return "../tests/UTC--2019-07-08T10-03-04.871669363Z--a73810e519e1075010678d706533486d8ecc8000"
}
func newTestKeyPwd() string {
	return "1234"
}

func newTestLinkAccount() (*LinkAccount, error) {
	return NewLinkAccount(newTestStateDB(), newTestLogger(), newTestKeyFile(), newTestKeyPwd())
}

func TestGetTokenBalanceBySubIndex(t *testing.T) {
	type stest struct {
		bal     map[common.Address]balanceMap
		idx     uint64
		tokenID common.Address
	}
	type testItem struct {
		input  stest
		output *big.Int
	}
	tests := []testItem{
		{
			input: stest{
				bal:     map[common.Address]balanceMap{},
				idx:     0,
				tokenID: LinkToken,
			},
			output: big.NewInt(0),
		},
		{
			input: stest{
				bal:     map[common.Address]balanceMap{LinkToken: balanceMap{0: big.NewInt(100)}},
				idx:     0,
				tokenID: LinkToken,
			},
			output: big.NewInt(100),
		},
		{
			input: stest{
				bal:     map[common.Address]balanceMap{LinkToken: balanceMap{0: big.NewInt(100)}},
				idx:     1,
				tokenID: LinkToken,
			},
			output: big.NewInt(0),
		},
	}
	assert := assert.New(t)
	resetMockAccount()

	for _, test := range tests {
		mockLinkAccount.AccBalance = test.input.bal
		balance := mockLinkAccount.getTokenBalanceBySubIndex(test.input.tokenID, test.input.idx)
		if balance == nil {
			assert.Equal(test.output, balance, "not equal")
			continue
		}
		assert.Equal(int(0), balance.Cmp(test.output), "not equal")
	}
}

func TestAutoRefreshBlockchain(t *testing.T) {
	type testItem struct {
		input  bool
		output bool
		err    error
	}
	tests := []testItem{
		{
			input:  true,
			output: true,
		},
		{
			input:  false,
			output: false,
		},
	}

	assert := assert.New(t)
	resetMockAccount()

	for _, test := range tests {
		err := mockLinkAccount.AutoRefreshBlockchain(test.input)
		assert.Equal(nil, err, "err not nil")
		assert.Equal(test.output, mockLinkAccount.autoRefresh, "autoRefresh not set ok")
	}
}

func TestCreateSubAccount(t *testing.T) {
	type testItem struct {
		input  uint64
		output uint64
		err    error
	}
	tests := []testItem{
		{
			input:  0,
			output: 0, //if only main address,not save,so it is zero
			err:    nil,
		},
		{
			input:  1,
			output: 2,
			err:    nil,
		},
		{
			input:  defaultMaxSubAccount,
			output: defaultMaxSubAccount + 1,
			err:    nil,
		},
		{
			input:  defaultMaxSubAccount + 1,
			output: defaultMaxSubAccount + 1,
			err:    types.ErrSubAccountTooLarge,
		},
	}

	assert := assert.New(t)
	resetMockAccount()

	for _, test := range tests {
		err := mockLinkAccount.CreateSubAccount(test.input)
		assert.Equal(test.err, err, "err not nil")
		accCnt, err := mockLinkAccount.loadAccountSubCnt()
		assert.Equal(nil, err, "err not nil")
		assert.Equal(test.output, uint64(accCnt), "CreateSubAccount count not equal")
	}
}

func TestGetAccountInfo(t *testing.T) {
	balanceExpect := big.NewInt(100)
	nonceExpect := uint64(50)

	balanceFailExpect := big.NewInt(0)
	nonceFailExpect := uint64(0)

	outputExpect := types.GetAccountInfoResult{
		TokenID:      &LinkToken,
		TotalBalance: (*hexutil.Big)(balanceExpect),
		EthAccount: types.EthAccount{
			Address: mockEthAddr,
			Balance: (*hexutil.Big)(balanceExpect),
			Nonce:   (hexutil.Uint64)(nonceExpect),
		},
		UTXOAccounts: []types.UTXOAccount{types.UTXOAccount{Address: utxoAccount0, Index: (hexutil.Uint64)(0), Balance: (*hexutil.Big)(big.NewInt(0))}},
	}
	resetMockAccount()

	Convey("test GetAccountInfo", t, func() {
		Convey("for succ", func() {
			Patch(GetTokenBalance, func(addr common.Address, tokenID common.Address) (*big.Int, error) {
				return balanceExpect, nil
			})
			defer UnpatchAll()

			Patch(EthGetTransactionCount, func(addr common.Address) (*uint64, error) {
				return &nonceExpect, nil
			})

			output, err := mockLinkAccount.GetAccountInfo(&LinkToken)
			So(outputExpect.Equal(output), ShouldEqual, true)
			So(err, ShouldBeNil)
		})
		Convey("for GetTokenBalance fail", func() {
			Patch(GetTokenBalance, func(addr common.Address, tokenID common.Address) (*big.Int, error) {
				return nil, fmt.Errorf("GetTokenBalance fail")
			})
			defer UnpatchAll()

			Patch(EthGetTransactionCount, func(addr common.Address) (*uint64, error) {
				return &nonceExpect, nil
			})

			stubs := Stub(&outputExpect.EthAccount.Balance, (*hexutil.Big)(balanceFailExpect))
			defer stubs.Reset()
			stubs.Stub(&outputExpect.TotalBalance, (*hexutil.Big)(balanceFailExpect))

			output, err := mockLinkAccount.GetAccountInfo(&LinkToken)
			So(outputExpect.Equal(output), ShouldEqual, true)
			So(err, ShouldBeNil)
		})
		Convey("for EthGetTransactionCount fail", func() {
			Patch(GetTokenBalance, func(addr common.Address, tokenID common.Address) (*big.Int, error) {
				return balanceExpect, nil
			})
			defer UnpatchAll()

			Patch(EthGetTransactionCount, func(addr common.Address) (*uint64, error) {
				return nil, fmt.Errorf("EthGetTransactionCount fail")
			})

			stubs := Stub(&outputExpect.EthAccount.Nonce, (hexutil.Uint64)(nonceFailExpect))
			defer stubs.Reset()

			output, err := mockLinkAccount.GetAccountInfo(&LinkToken)
			So(outputExpect.Equal(output), ShouldEqual, true)
			So(err, ShouldBeNil)
		})
		Convey("for EthGetTransactionCount and GetTokenBalance fail", func() {
			Patch(GetTokenBalance, func(addr common.Address, tokenID common.Address) (*big.Int, error) {
				return nil, fmt.Errorf("GetTokenBalance fail")
			})
			defer UnpatchAll()

			Patch(EthGetTransactionCount, func(addr common.Address) (*uint64, error) {
				return nil, fmt.Errorf("EthGetTransactionCount fail")
			})

			stubs := Stub(&outputExpect.EthAccount.Nonce, (hexutil.Uint64)(nonceFailExpect))
			defer stubs.Reset()
			stubs.Stub(&outputExpect.EthAccount.Balance, (*hexutil.Big)(balanceFailExpect))
			stubs.Stub(&outputExpect.TotalBalance, (*hexutil.Big)(balanceFailExpect))

			output, err := mockLinkAccount.GetAccountInfo(&LinkToken)
			So(outputExpect.Equal(output), ShouldEqual, true)
			So(err, ShouldBeNil)
		})
	})
}

func TestGetAddress(t *testing.T) {
	type testItem struct {
		input  uint64
		output string
		err    bool
	}
	tests := []testItem{
		{
			input:  0,
			output: utxoAccount0,
			err:    false,
		},
		{
			input:  1,
			output: utxoAccount1,
			err:    false,
		},
		{
			input:  2,
			output: utxoAccount2,
			err:    false,
		},
		{
			input:  3,
			output: "",
			err:    true,
		},
	}
	resetMockAccount()
	Convey("test GetAddress", t, func() {
		err := mockLinkAccount.CreateSubAccount(2)
		So(err, ShouldBeNil)
		for _, test := range tests {
			addr, err := mockLinkAccount.GetAddress(test.input)
			So(err != nil, ShouldEqual, test.err)
			So(addr, ShouldEqual, test.output)
		}
	})
}

func TestGetBalance(t *testing.T) {
	type testItem struct {
		index   uint64
		token   *common.Address
		balance *big.Int
	}
	tests := []testItem{
		{
			index:   0,
			token:   &LinkToken,
			balance: big.NewInt(100),
		},
		{
			index:   1,
			token:   &LinkToken,
			balance: big.NewInt(50),
		},
		{
			index:   2,
			token:   &LinkToken,
			balance: big.NewInt(0),
		},
		{
			index:   1,
			token:   &mockTokenA,
			balance: big.NewInt(0),
		},
	}

	Convey("test GetBalance", t, func() {
		resetMockAccount()
		err := mockLinkAccount.CreateSubAccount(1)
		So(err, ShouldBeNil)

		for _, test := range tests {
			mockLinkAccount.setTokenBalanceBySubIndex(*test.token, test.index, test.balance)
			balance := mockLinkAccount.GetBalance(test.index, test.token)
			So(balance.String(), ShouldEqual, test.balance.String())
		}
	})
}

func TestGetEthAddress(t *testing.T) {
	resetMockAccount()
	assert := assert.New(t)
	assert.Equal(mockEthAddr, mockLinkAccount.getEthAddress(), "not equal")
}

func TestGetGOutIndex(t *testing.T) {
	type testItem struct {
		token common.Address
		index uint64
	}
	tests := []testItem{
		{
			index: 1,
			token: LinkToken,
		},
		{
			index: 0,
			token: mockTokenA,
		},
		{
			index: 0,
			token: mockTokenB,
		},
	}

	Convey("test GetGOutIndex", t, func() {
		resetMockAccount()
		mockLinkAccount.increaseGOutIndex(LinkToken)
		mockLinkAccount.increaseGOutIndex(LinkToken)
		mockLinkAccount.increaseGOutIndex(mockTokenA)

		for _, test := range tests {
			idx := mockLinkAccount.GetGOutIndex(test.token)
			So(idx, ShouldEqual, test.index)
		}
	})
}

func TestGetHeight(t *testing.T) {
	Convey("test GetHeight", t, func() {
		resetMockAccount()
		expectRemoteHeight := big.NewInt(12)
		mockLinkAccount.remoteHeight.Set(expectRemoteHeight)
		Convey("test localHeight zero", func() {
			localHeightSet := big.NewInt(0)
			mockLinkAccount.localHeight.Set(localHeightSet)
			local, remote := mockLinkAccount.GetHeight()
			So(expectRemoteHeight.Cmp(remote), ShouldEqual, 0)
			So(local.Cmp(localHeightSet), ShouldEqual, 0)
		})
		Convey("test localHeight not zero", func() {
			localHeightSet := big.NewInt(10)
			mockLinkAccount.localHeight.Set(localHeightSet)
			local, remote := mockLinkAccount.GetHeight()
			So(expectRemoteHeight.Cmp(remote), ShouldEqual, 0)
			So(local.Cmp(new(big.Int).Sub(localHeightSet, big.NewInt(1))), ShouldEqual, 0)
		})
	})
}
