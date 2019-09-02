package wallet

import (
	"math/big"
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"

	// . "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
)

var (
	mockEthAddr = common.HexToAddress("0xa73810e519e1075010678d706533486d8ecc8000")
	mockTokenA  = common.HexToAddress("0x000000000000000000000000000000000000000a")

	mockLinkAccount *LinkAccount
)

func init() {
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
	return log.Test()
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

func TestGetEthAddress(t *testing.T) {
	assert := assert.New(t)
	assert.Equal(mockEthAddr, mockLinkAccount.getEthAddress(), "not equal")
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

	for _, test := range tests {
		mockLinkAccount.AccBalance = test.input.bal
		balance := mockLinkAccount.getTokenBalanceBySubIndex(test.input.tokenID, test.input.idx)
		if balance == nil {
			assert.Equal(balance, test.output, "not equal")
			continue
		}
		assert.Equal(balance.Cmp(test.output), int(0), "not equal")
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

	for _, test := range tests {
		err := mockLinkAccount.AutoRefreshBlockchain(test.input)
		assert.Equal(err, nil, "err not nil")
		assert.Equal(mockLinkAccount.autoRefresh, test.output, "autoRefresh not set ok")
	}
}
