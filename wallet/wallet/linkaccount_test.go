package wallet

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	. "github.com/bouk/monkey"
	"github.com/lianxiangcloud/linkchain/libs/common"
	lkctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
	"github.com/lianxiangcloud/linkchain/wallet/types"
	. "github.com/prashantv/gostub"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
)

var (
	mockEthAddr  = common.HexToAddress("0xa73810e519e1075010678d706533486d8ecc8000")
	mockTokenA   = common.HexToAddress("0x000000000000000000000000000000000000000a")
	mockTokenB   = common.HexToAddress("0x000000000000000000000000000000000000000b")
	utxoAccount0 = "bwcJ9V3z7uW1fbYm2L6HuCGiuaTSVr5dq7ir49ViFBNeYYQqMuUM6S16aWKz4HmGRFGDb5RnfVBv7uGeySjkzUkmEbGKRN"
	utxoAccount1 = "oRWauC7hnjcupPN2UMzT3fTVpv99jv2ZEmr4QV6kYjG8XaeWHTNBRXeQq2yeEHgbm85Zqu6DBjTLYZxPEJUwRugrgeUkyY"
	utxoAccount2 = "oRTryuuZqwvd7kyWX8vPjKYyUD7M2Vmt82gvgqb5DzHme1tWSpwdenqoV9X72ExsSRAYfr1k471tmUWG9cFPRbDiZcQHmr"

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
			err:    types.ErrSubAccountOverLimit,
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

func TestOnStart(t *testing.T) {
	remoteHeightExpect := big.NewInt(7)
	blocks := [][]byte{
		[]byte("{\"number\":\"0x0\",\"hash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"miner\":\"0x0000000000000000000000000000000000000000\",\"timestamp\":\"0x59de4000\",\"parentHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"transactionsRoot\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"stateRoot\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"receiptsRoot\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"gasLimit\":\"0x12a05f200\",\"gasUsed\":\"0x0\",\"logsBloom\":\"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\",\"transactions\":[],\"token_output_seqs\":null}"),
		[]byte("{\"number\":\"0x1\",\"hash\":\"0xb51d17bcb8d455723b142d1f0a5fd57144fd10838780bbecb790c7ac15c63e8d\",\"miner\":\"0x0000000000000000000000000000000000000000\",\"timestamp\":\"0x5d774466\",\"parentHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"transactionsRoot\":\"0x6126a588508eaeb6a8bb0fc04705f4f924b419924cbeb03e519a39d9a5147151\",\"stateRoot\":\"0x6122e854e252324f744ef1f679bdb3d244cafe10d97072e5eee49937124d04f7\",\"receiptsRoot\":\"0x146fb59d99447d0778eba55f8e56d0871e75baf96535fff9f1d3f9d1a90e6af0\",\"gasLimit\":\"0x12a05f200\",\"gasUsed\":\"0x1dcd6500\",\"logsBloom\":\"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\",\"transactions\":[{\"type\":\"rpctx\",\"value\":{\"txType\":\"utx\",\"txHash\":\"0x6126a588508eaeb6a8bb0fc04705f4f924b419924cbeb03e519a39d9a5147151\",\"from\":\"0xa73810e519e1075010678d706533486d8ecc8000\",\"tx\":{\"type\":\"utx\",\"value\":{\"inputs\":[{\"type\":\"AccountInput\",\"value\":{\"nonce\":\"0\",\"amount\":10050000000000000000000,\"cf\":\"yFB/YDXIBfChBKKwjU2rUyKdjpMuq9vMyD/EDHmXKQQ=\",\"commit\":\"5GYYWOfPCiOxgq8WGUKHyeaMiXuDSjbmMEFYvm97gkY=\"}}],\"outputs\":[{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"2nsi28WUKH3+GWdTL2pzAu6Nx2GjtOLFqkp5FMqUbHk=\",\"amount\":0,\"remark\":\"aByYDCaRiRMp3q0KoxWJMjPcrdbGzaCHFIhBA9G9224=\"}}],\"token_id\":\"0x0000000000000000000000000000000000000000\",\"r_key\":\"RiH9tiO0BI8Wc3JasCKhU8EY1cx4Al55cRfc6EhaEPg=\",\"add_keys\":[\"qqMBkjCX2VWWTzjLbDeFFh+SoMO5cmCNE+AcFuY7eWA=\"],\"fee\":50000000000000000000,\"extra\":null,\"signature\":{\"v\":58342,\"r\":10416621822990522327237098928979253963796477615164519070266070931196189365450,\"s\":36677495745897540434694730160412092472807973074830974516504767352714418589585},\"rct_signature\":{\"RctSigBase\":{\"Type\":0,\"PseudoOuts\":[],\"EcdhInfo\":[{\"Mask\":\"HVvMrN2SroYYcZqs/JrJKvSxjEeyWYq1G32u1Ftu1AU=\",\"Amount\":\"RuEZaJlgvU/VyCAM215W7IqPqm1e6fhw/Y5zjrn0XA8=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"OutPk\":[{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"X6JIgorHvKAkEA4U0VZXUdcb1Bvx+nJKatvp50LOJgQ=\"}],\"TxnFee\":\"0\"},\"P\":{\"RangeSigs\":[],\"Bulletproofs\":[{\"A\":\"Hu6S9DLI6FBSqTaog0oUfMhd7ztlnkQpPH1KB/+4pkI=\",\"S\":\"DtPSkGI7/9FK8P/8Fo1rkpgz9LY2+QcMpJ6ZxW6Wr5E=\",\"T1\":\"eO91huvgMmIHBT6QRIWiTNRsueQl35gPCmLnjDioRdE=\",\"T2\":\"g2QU4Y9+Wh9jYlnMPX13nTWyPoaf0c7GX6UxoBMRofI=\",\"Taux\":\"os8ZGMxt52DeIWg1JGAf5B3Ud/nhxKfCo9R68l6pVgs=\",\"Mu\":\"uj2S2+YfgZaWcrFG/50Euxuy0FS5X+6pcqTyPVLq6gc=\",\"L\":[\"MXCG+hcO6Sw4MsDeDLGlo7nZVIhkPo2zkbjuCXi9N3E=\",\"4iK6hRFbdP2ARQBu4WqiyGKBxljDizX/EwnoIFw7ZLs=\",\"TEWCgMKCkw3SCXqpS9AT5ONz+LdoIk0ja8Swmqp74qw=\",\"JXTMnqrEemzHLIsFV+xm1CKBlz5tx/83WTWSPWv8Txc=\",\"5SuHAySPTUeEqtN90aLJfjY6hPXPRSUU26mmzfH2DgQ=\",\"tRfqr3KTWXAPfK3brzcDZFkSJGqSyBkgWhqBCWCmxTo=\"],\"R\":[\"hEZzHaPDktFHxNxBCbrtqRKxFaTVgr0qqjivhHngEpI=\",\"MRAX65uXYWZjHKjO5/fgfkh9j8tiRpq+BcCJhV4Yuuw=\",\"rozVjNizD3Tp1ajJ72xBEtnOMRHTWNsc+SGq9V/9Ons=\",\"C7bujufx6G6WRR5kixGNtwyS9G4hi91V9yKUPM1pl4c=\",\"GKZwDhf8Lp33KhvWcRdl3Xui0AAOIqLzu/aOX6FBX3g=\",\"vmdm0fcBIXA+b1MZHNcSvc9Za3Usw3GdNNzpmI+cEXo=\"],\"Aa\":\"ea2lf/HeoZHdIb6jiE/nRLwoSPWmAnPwQ1claoBfPAc=\",\"B\":\"uH5/iLTIsA3sU0WW1yYmBClkwqW0sOiM8jYAF7u7fA4=\",\"T\":\"K5jsWXxXhc9s2AXaGGtSSadXeK8A2sLtutjijgjaYgk=\"}],\"MGs\":[],\"PseudoOuts\":[],\"Ss\":[]}}}}}}],\"token_output_seqs\":{\"0x0000000000000000000000000000000000000000\":-1}}"),
		[]byte("{\"number\":\"0x2\",\"hash\":\"0x6c284b22630b778336ed53d3b9cfde140e8386ec8dcb41df9b2894df6dd8462c\",\"miner\":\"0x0000000000000000000000000000000000000000\",\"timestamp\":\"0x5d774467\",\"parentHash\":\"0xb51d17bcb8d455723b142d1f0a5fd57144fd10838780bbecb790c7ac15c63e8d\",\"transactionsRoot\":\"0xd2d82869b49f65befcaa3032a44f8eabd0766fe7cb7069aa99334051350d9617\",\"stateRoot\":\"0x2707865deb1dc060f34a38f253b0abfc82ee0af9c189460e186a8b7459aefc0d\",\"receiptsRoot\":\"0xebffdeefe43b005028c0bed2a65879f90bf09bb176ba8329a72ee0626a96ae60\",\"gasLimit\":\"0x12a05f200\",\"gasUsed\":\"0x3b9aca00\",\"logsBloom\":\"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\",\"transactions\":[{\"type\":\"rpctx\",\"value\":{\"txType\":\"utx\",\"txHash\":\"0xd2d82869b49f65befcaa3032a44f8eabd0766fe7cb7069aa99334051350d9617\",\"from\":\"0xa73810e519e1075010678d706533486d8ecc8000\",\"tx\":{\"type\":\"utx\",\"value\":{\"inputs\":[{\"type\":\"AccountInput\",\"value\":{\"nonce\":\"1\",\"amount\":20100000000000000000000,\"cf\":\"tvbCjlXN+6N2M0pdatBslIfhEd8GsWZH2OnAjpctVAY=\",\"commit\":\"vtaxV+7x3+3OSz6JMCsP2NHHLEhkflvGLB/qWRfVHG4=\"}}],\"outputs\":[{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"vCBVhOllNunmpm0RYvkJHhhIbrZk7xhdTVCzFrkJFsw=\",\"amount\":0,\"remark\":\"O8fjVrRZITczAAM8QyrD4PZznmxOCgPS1icpB0z9r48=\"}},{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"t/sm7bNEmmsOh8tyHUidF77BN0MH8pEDtjlgjuH0SgM=\",\"amount\":0,\"remark\":\"gtl0duxkXZ4mmDn3JcRY30NGK6EJs10BaIwWmcvlOXQ=\"}}],\"token_id\":\"0x0000000000000000000000000000000000000000\",\"r_key\":\"SIpdHq816hm7T7tj91RB7MM+OUDR5I8w0Ii2U6+xzKQ=\",\"add_keys\":[\"VOs1hCfCQEMYtsVeKmqkGGvJh2XFT7mMZNy7QVxcAzI=\",\"TIU8uwOUN6QQ4jpjH53YI9bEc8DPtXQRi7qM6xpJbVk=\"],\"fee\":100000000000000000000,\"extra\":null,\"signature\":{\"v\":58341,\"r\":7438111402584847116381203454381232403597925286796096803279778412438903191433,\"s\":54611048478830286525029265055848591239510409057555981074719311771422441193142},\"rct_signature\":{\"RctSigBase\":{\"Type\":0,\"PseudoOuts\":[],\"EcdhInfo\":[{\"Mask\":\"XJgGZ6NKZMyEDKI+S+PONPQ0xoHSCLGKMUNeVEhsVgU=\",\"Amount\":\"kyL+XjwVrtEY1mzFftnabJBr2soFOGdDXWqBgqYcaQc=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"},{\"Mask\":\"EqC6SH7gmlmD+hUSU6WpIWBkCCQzyyJB5JyZ3IXsDQw=\",\"Amount\":\"qYJ0Z67DDf3Mz9PNEb/scPX5H00xAhYAZWits+652QU=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"OutPk\":[{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"NMsbPEBd+tH3Yhg0BZX/K0WSOeDK8tbQP6mY86jADQU=\"},{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"9kU6CZflZjrApmknVAj3kcCPTh1QgUvV3iJFEAywAn4=\"}],\"TxnFee\":\"0\"},\"P\":{\"RangeSigs\":[],\"Bulletproofs\":[{\"A\":\"I6HooaZALsSv6xXpD40gOhKUBS64BduPOjebgq4qg8c=\",\"S\":\"eBbNSkqIkgA5FU8CbYA+8uz6ttC/WtJT/P2MLJW60/I=\",\"T1\":\"TwdUESy0rN8M4W8wsnFtUUpQICdtM902msK/DN9cwvQ=\",\"T2\":\"tfcRI7QbbtYZ9NKvc34ohvu4bmH3Jlhb7fQVJWQTALk=\",\"Taux\":\"EYUrm1P6kxxoVrpo0cT1oFfbmcFhdXPPdCFMiv7ehAQ=\",\"Mu\":\"XGHk8P4oooUxZ5Lxaqy3A5cMoH/054IgzMlO34BplQo=\",\"L\":[\"ZRm9GC0rMcaKAjRo3Clpdd3ErgqWLXysPgmT/RqffRg=\",\"4UVOUi9yTcjMiuRk+cXaE1U9jy9cdonEDbE14ELAsN8=\",\"M2OQCSd+lm6RF8/wAkLUGzQx/gu/71jSzhuYRcxMQmU=\",\"ejrKco0ZmCA+NETDREKQxuUm2VOCbak8zxZ20hgVLX0=\",\"uxNW/4D7Nb+yOdBoIJ9ezTgOXAUm1q6IbUvdWDI2zRM=\",\"zg1Qkm5qjcydn5T71WbDucrcOuDOgW5YQt/OaR1pmWA=\",\"Vm9mQckP8FYERafhlzVEzgqoKs349xQrjXSa7qA5fE8=\"],\"R\":[\"AXJyNC8tQywDXQAM5+O9w+hj3M8rOwNi/nx81sbtSoo=\",\"k95iOZzlMAvwVwqcPUDtzwUmZxt59oERsQ1V6eUCFhg=\",\"BFwdEWXbWr6z0HFidEwsBCzgXIxM/ZqeLJkJIQFpNn4=\",\"beLSrxLvO+ifpvDeb5DKN6CbnD9xXNxZ6lnpmYQRMLg=\",\"lDG9gR7yKoq/UkvbDplH577BtXyocUb7nM5ADqnddac=\",\"SiEyt+lbaKhrIDnVIhGODtR+fIMwgzSn/X/NR/0oicc=\",\"vJZP3amy81SEscL41sWsbvw1EnIojB0egNXwBic3iog=\"],\"Aa\":\"bTMFdwR7DaNHUUoQb/ecHC7NoWYVH0SeJuvzPLguMA8=\",\"B\":\"UWWW72FvkUQG89P5OVyvH6T6XzLS47Fcqessc3uTxQA=\",\"T\":\"8zfH3513K0mony8e+jq2hh4OwezQlWnH7iftmbD3LQw=\"}],\"MGs\":[],\"PseudoOuts\":[],\"Ss\":[]}}}}}}],\"token_output_seqs\":{\"0x0000000000000000000000000000000000000000\":0}}"),
		[]byte("{\"number\":\"0x3\",\"hash\":\"0xe36b8540c17e95bf228ea20afd985a1c883d20067c4497afbb3bded6a995a782\",\"miner\":\"0x0000000000000000000000000000000000000000\",\"timestamp\":\"0x5d774478\",\"parentHash\":\"0x6c284b22630b778336ed53d3b9cfde140e8386ec8dcb41df9b2894df6dd8462c\",\"transactionsRoot\":\"0x6982d558bed953be1db9df9486886db125a8bb22029a5d2660bd790cf063aa74\",\"stateRoot\":\"0x43a1983dd7d51410090e7e0f5d0f4b168586c35d0174386655a1acb10126cd7f\",\"receiptsRoot\":\"0xe9c2ab8f0b5aa12f3a8b7507d4c090bb37527dd00ae5e574fa3e75b65b7fda1b\",\"gasLimit\":\"0x12a05f200\",\"gasUsed\":\"0x1dcd6500\",\"logsBloom\":\"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\",\"transactions\":[{\"type\":\"rpctx\",\"value\":{\"txType\":\"utx\",\"txHash\":\"0x6982d558bed953be1db9df9486886db125a8bb22029a5d2660bd790cf063aa74\",\"tx\":{\"type\":\"utx\",\"value\":{\"inputs\":[{\"type\":\"UTXOInput\",\"value\":{\"key_offset\":[\"0\"],\"key_image\":\"ETaYg3l0pJMp4nycvSuwopbakf2lhHtu4uWZEAiFHr0=\"}}],\"outputs\":[{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"JtSdSJRtKLakClZpV+YWIPPuzvZB+DpPuk7lWl5Puxo=\",\"amount\":0,\"remark\":\"0PFW2lbvT8lzn7mYbUV/TXzyUCmT8rcOOXCxrUcShkk=\"}},{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"CxaHXz1BW2OtgwFLO47LzeYmAKOtHe5wr8rYl5MaB3c=\",\"amount\":0,\"remark\":\"4dHrmC8RTy8TSRetPco8teXuVQFbzqV/lA0LKRL3J4w=\"}}],\"token_id\":\"0x0000000000000000000000000000000000000000\",\"r_key\":\"ZlVsxILJi97Y88JDAKLmD8BRzEb4OHZIqn+NcFJyTJ4=\",\"add_keys\":[\"/9bgD20CWNccMKv04zFGoTNhjtWbMWBXQDs8MMiJxO4=\",\"xQVNhihGJNiPGeiv7KtzqENOOuLmC2+f4nZ6b2Nuj9E=\"],\"fee\":50000000000000000000,\"extra\":null,\"signature\":{\"v\":0,\"r\":0,\"s\":0},\"rct_signature\":{\"RctSigBase\":{\"Type\":3,\"PseudoOuts\":[],\"EcdhInfo\":[{\"Mask\":\"fqkxEy/G+MVDxgCQk/QdyaDrVpu7mW9A29XAGq/TEA8=\",\"Amount\":\"6Zl7tKGHCaofFsS1rSGbFaYoklIwaY36jcr2XUrOzgY=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"},{\"Mask\":\"88bzMmW76mq/8jg6ybbselEJTeJPKtTwbjCSedWyEQE=\",\"Amount\":\"EiIxGnYS22T+ysF71vV+M+A/QcxPnJmgUL/cLLLxvAk=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"OutPk\":[{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"GqYLLB1H/3AyPkzJY6BMEO340567JvToG8K3XdDC2cY=\"},{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"amiG39OUuP8bB7acdrm0JJUZ43oGK/Dbq7h9u5soZAM=\"}],\"TxnFee\":\"0\"},\"P\":{\"RangeSigs\":[],\"Bulletproofs\":[{\"A\":\"vHG7Fej12opwBFGr/bwi7b9MLVQIPZ4vGw8X7rvyL4E=\",\"S\":\"YWYfAtdfd5L3c8BjyVKM63XDJAMYHL/wVyNZJkp9Zug=\",\"T1\":\"Wr8GIJU8qfM5FxXcUBrNPruyWjFZ6hyC5dVlBWGSx/U=\",\"T2\":\"18szk17m4ZvhQfcwvVt76USqZ54pJ5ESeUholZAl/Cw=\",\"Taux\":\"rMpa9vN6cAM8v79xagIhI9t8gzABoe0pBa5TeekVeQI=\",\"Mu\":\"f7v5I5BJ8nBJHcsKlL5nGWmhj9M1BAWOSfRA6gFn/wY=\",\"L\":[\"/gKHR1kAtf7/srjQtXxeKQuzncAXotV6eI9lJBvRAd4=\",\"0MmwEoKFYmqNPBuFEqWHv5PDxGB98qeHGtDyyDyKqBE=\",\"54bwryM3mlJx4L+2J4MlXCjMto+CWZZ87xxznAfuzj0=\",\"p7qQFeI3SNyTJA8si4eKaX1TmFtqPgaRjy6A8NisffU=\",\"e/HrRuX62k/vZt57JqDDrJ2oWj8EVq4RyyAk3oLoGw0=\",\"t7IZ1T7nfCDfpa72gvEQJiajro8Co/CMK5nj/rzt20A=\",\"G+SSQIfVRkCjsDphQGW+ZnBIvvHHdASR23LqixIR3Eg=\"],\"R\":[\"TkMA/Nhipn5AqTlnML1ClbRU71whET5VN6Rsg10e6HQ=\",\"+yD1GU0aLQks+/Vfe2Z77JeLGHvWKpanXDmsaKZjNKY=\",\"Ghx1AVFuaKoeY1BiYsGGfQ6XOxU4/wKwwQITNVGJAng=\",\"Q3MnKvIZLHhhTNFiOAM6+FZVav45kTCxeJNi+AfGyEk=\",\"dtIx0XmBPQEov1083uaC0GLuFWSGR47uX4ig/8plqBs=\",\"pmGTuuZiSEiY5PhRVWwOUmbLTamrzal79D6YhSiLZP0=\",\"9Lj5HtlkluyPz1pMbAXDREQss0RMyCqlgZyoEHH5ihw=\"],\"Aa\":\"yKbz5pRCAD7pnb+I8zkKldIErFJgl/8GT0wGZxoUjQg=\",\"B\":\"qqkM+rj4agiDlbQHOrzBzDGiJjHAGKN47PFOfxHPuQo=\",\"T\":\"ygXynvtZRD1Ie+iYia0YokXKAxBEdb/m/FCD/QGTpQo=\"}],\"MGs\":[{\"Ss\":[],\"Cc\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"PseudoOuts\":[\"deyH6oKoKOqew6cC+ezi0gOCJTa4tqCQbi6+JmIk0TY=\"],\"Ss\":[{\"C\":\"ZKSQ7Kkt9d9C6Fhg2t7Qg+7DVYi2+wI9SrrfSO0xvAA=\",\"R\":\"Zzb7mJFiBmUlgseqJNNRap33/F+cORT9bpsYPdksBwY=\"}]}}}}}}],\"token_output_seqs\":{\"0x0000000000000000000000000000000000000000\":2}}"),
		[]byte("{\"number\":\"0x4\",\"hash\":\"0x1e626732214e468d4070f3eaac509f033116edcb7644b350ef8172fce15c8e13\",\"miner\":\"0x0000000000000000000000000000000000000000\",\"timestamp\":\"0x5d77447f\",\"parentHash\":\"0xe36b8540c17e95bf228ea20afd985a1c883d20067c4497afbb3bded6a995a782\",\"transactionsRoot\":\"0x20a3d3d40c4b0ee7d1bfa4994aa876f6929376cd5eefb5c5cb8a95c61693ff5a\",\"stateRoot\":\"0x6231efee7751c1c1fb7a30dd71548c919b56dabd6140cdbef71783d7c5c062c4\",\"receiptsRoot\":\"0x68d4a9a63873b28a50a31aa2b75ffab6bc7ba2743bb7a7a327566f58a9eeaed9\",\"gasLimit\":\"0x12a05f200\",\"gasUsed\":\"0x1dcd6500\",\"logsBloom\":\"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\",\"transactions\":[{\"type\":\"rpctx\",\"value\":{\"txType\":\"utx\",\"txHash\":\"0x20a3d3d40c4b0ee7d1bfa4994aa876f6929376cd5eefb5c5cb8a95c61693ff5a\",\"tx\":{\"type\":\"utx\",\"value\":{\"inputs\":[{\"type\":\"UTXOInput\",\"value\":{\"key_offset\":[\"1\"],\"key_image\":\"AnRwPQNB6VdRg/rjBoJGtrnDZ0I65R79cB7IvkEsJiY=\"}}],\"outputs\":[{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"cG2w3l+LOedFBFEZSUItKVoHKS1Nkax51ZWWmR4XHOM=\",\"amount\":0,\"remark\":\"U9kZ6UUE5H1PMgESQK5YieOyOFcKudv8mlsgVgx8m1w=\"}},{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"4epPOMBLx3UvStv1jV77RXe5PA6+N85AW1gYnNUMazk=\",\"amount\":0,\"remark\":\"2e8/nDIEZeiuLF6PryDCDrZ6rs/xm7bNxD0MnkfC3ak=\"}}],\"token_id\":\"0x0000000000000000000000000000000000000000\",\"r_key\":\"+JsToIM6cceHu1bKxQVMwpqEen6zKs8/nF6W1yQeKBw=\",\"add_keys\":[\"WC+0VopVbLAEbYKzEFNTK/gI48Pyh3n92Hrk2kEIcYM=\",\"GVVwk8ERJfWSVW7GIH8rKRVNhpKHOe/gdVCKO+iI/Jo=\"],\"fee\":50000000000000000000,\"extra\":null,\"signature\":{\"v\":0,\"r\":0,\"s\":0},\"rct_signature\":{\"RctSigBase\":{\"Type\":3,\"PseudoOuts\":[],\"EcdhInfo\":[{\"Mask\":\"gLgYfzOJh/HI4RFQ7IF/PdsEe4yLZjb9y/xpp5ERvw4=\",\"Amount\":\"y9mwhcH0isW3wdqqIPFyIxTIuISJKw/UDI4nI08xOgU=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"},{\"Mask\":\"pB6NGfBfeGLnznPXubXwYx/x+KRmNSkQ86sAoUY4Hw0=\",\"Amount\":\"ZiTOQq63rkutuMaiWmNIM890mqFn3BAe9/dFzkTbrAA=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"OutPk\":[{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"YVJFnqLzNaJSxbIWbHgzmpL9b015D9vxGBOrVDvFHUQ=\"},{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"/PXNCyjiETpIcZHjrwudghQ8TfPQvB0IpeDPwkY/Kps=\"}],\"TxnFee\":\"0\"},\"P\":{\"RangeSigs\":[],\"Bulletproofs\":[{\"A\":\"o3dJrOC5KZHheI7Je4xVvupdZg0YBa5rE6gengLleMw=\",\"S\":\"aYsIVTi/4ZCkDHBducMOIrjirLT2MRHuYlGU0cgQvWM=\",\"T1\":\"4H2EQAVLwChZEZVVdMnwXkelPKkGWWaw2YOl4X/FbS8=\",\"T2\":\"z8zrcQIbP08FbVBCQJBY6V5kq9UmezXbDnGUwonZZHM=\",\"Taux\":\"D2LIHRWPQ3qjzDfJkp/eRAtDgcPxekkV96hK8edpYAA=\",\"Mu\":\"DzaXQlpinmgCuclOzteJrva6X9/vVF3T7qEQXbZa+w8=\",\"L\":[\"osdYIuJmKmsVy2ZTy4vfLFE2/OpJsPEhLJYPovttAMU=\",\"he2xNgLR2qCZAko7NXIVQVhyu3klchHV3agnwMLpAbs=\",\"QxVcJ1ZO+w9yZYUbWBI2RFzTRDTuS854OgRY36ONMzU=\",\"tWiPn8TdxLxfkyKVbIPySmJGMv1wDTG+CYcE4puY1lc=\",\"TbOBZ+fgsn6/31+2j1FShPJ8seWdNCA+drwNhJxbNGk=\",\"FRYzHJcK9YKVQKDYxvbt03+fu9R/goOf90iX4L/pgMI=\",\"6lNLzGohNUpsiNiVrP84S554hXh+dojo54Yrqt7Pkn0=\"],\"R\":[\"KPlOSf/ptWY4zyP78Tndq4KA1Q76XR8h3sH23O4Hg+o=\",\"aaKaEmTnVTy13o1q6/jHAa7KqW5WB8bszWGJxKiTlKA=\",\"nE6fRnIYDv6O4KDGuzHKzAq73KITUvllyVETPy7MNaM=\",\"CODHLWbbU5g87fn4GIB/OCMF4DtdjIy7Jn3OPQzJjXM=\",\"tk7npJgwCC89Sdqk6jswfyrw8I4FGWG6qsBeq/kH/9k=\",\"55P6NYZcbYWX3ST6Yr+EY2m8pO9Tkq8T7nCKUNF4ir0=\",\"HryhiafTW1w9ghI0ZtFpmuTLS43YP23t2y2mP6Wn7Pc=\"],\"Aa\":\"94G1EhxkF/88H5pTQgfJDUr1ru+GK7XvWAgT4FFXfQ0=\",\"B\":\"9OCoYkccy3FnRWVssv45q2gn4VWIsO/KUpVHXUQnDAM=\",\"T\":\"p1IsMp/oMJoCsBmaCrZW6OGcjeZ+ytboyF1XuI5cOAM=\"}],\"MGs\":[{\"Ss\":[],\"Cc\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"PseudoOuts\":[\"00ZxjKYajkyMCL4wWEnWZ3IanrpFP6dlP0APuxPvfdI=\"],\"Ss\":[{\"C\":\"HNAwSgMyIUn+mBCm08e9m4aELQlCrdma23Xx0kUTdws=\",\"R\":\"W8o03/2hfWkrq2NFaJlMRIzHqlHILEZkE6lqTkPKWQs=\"}]}}}}}}],\"token_output_seqs\":{\"0x0000000000000000000000000000000000000000\":4}}"),
		[]byte("{\"number\":\"0x5\",\"hash\":\"0xdd4574ae50135ebce00aecc771875a9127949af817b72efc08145edfa72f4533\",\"miner\":\"0x0000000000000000000000000000000000000000\",\"timestamp\":\"0x5d774483\",\"parentHash\":\"0x1e626732214e468d4070f3eaac509f033116edcb7644b350ef8172fce15c8e13\",\"transactionsRoot\":\"0xfacf22f4f9e1b87b6a1cab7cbacb4152341fd27b2b2ba62c0953f7ebc23083bc\",\"stateRoot\":\"0x0a2a639c24ebbcedd1f82b683b096ff4f971fd9b8b5818c057dab8335d61ce00\",\"receiptsRoot\":\"0xf6f66a029a4e98e4eab3918445e9ba367d9f01e526072352c25a7336bb784a3f\",\"gasLimit\":\"0x12a05f200\",\"gasUsed\":\"0x1dd50620\",\"logsBloom\":\"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\",\"transactions\":[{\"type\":\"rpctx\",\"value\":{\"txType\":\"utx\",\"txHash\":\"0xfacf22f4f9e1b87b6a1cab7cbacb4152341fd27b2b2ba62c0953f7ebc23083bc\",\"tx\":{\"type\":\"utx\",\"value\":{\"inputs\":[{\"type\":\"UTXOInput\",\"value\":{\"key_offset\":[\"4\"],\"key_image\":\"8josOCAZqQJwfpXYqhn3psJ/RUwnI/59gsSNr6eX+fM=\"}}],\"outputs\":[{\"type\":\"AccountOutput\",\"value\":{\"to\":\"0xe50ab035b1cc691b84e415ff0931867f6a71b091\",\"amount\":5000000000000000000,\"data\":\"EjNE\",\"commit\":\"OU5C09tqX4i7YCEKEJqFmAzp5gFakBgSHwCysG+ImCw=\"}},{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"DOCZ2SoFgpBZoRYP52+OS0NG2B7u49iHkIBHyxbuQII=\",\"amount\":0,\"remark\":\"gDHlVKuevQ0FdkxC0qt0vgkIMA85rwA2jcoFowYAZwQ=\"}}],\"token_id\":\"0x0000000000000000000000000000000000000000\",\"r_key\":\"ZKnmCd6iZH25PrIrfMeNTWk36NW9PMQOfmRgO8SDdqM=\",\"add_keys\":[\"MqrxHMOQriPMLuscWCozU32vjaGtNQoKPFYcE2AMPE0=\",\"S7yuHRP4aLXKNbKnsDMwxkQZIiyff0WpKZrRgdQRAeU=\"],\"fee\":50050000000000000000,\"extra\":null,\"signature\":{\"v\":0,\"r\":0,\"s\":0},\"rct_signature\":{\"RctSigBase\":{\"Type\":3,\"PseudoOuts\":[],\"EcdhInfo\":[{\"Mask\":\"nqvVOpLCIT7EwMIvMLiqfcUcjQUwYX7qKwLykxuJPQc=\",\"Amount\":\"3rs6zoLr8VqWuOFLoU7fCIohzzs5icGg+0i/3hg1ZAc=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"OutPk\":[{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"MtO01K8ZIaDyXBCf+1bTbJJZlVcnMntJVYitBmks1Jg=\"}],\"TxnFee\":\"0\"},\"P\":{\"RangeSigs\":[],\"Bulletproofs\":[{\"A\":\"AbxGQNN190E5qXN+zvSzsCJ07tHFGixvuWKEv6OufRI=\",\"S\":\"wZjy+J2xv8V6ZYw39j0CgcvkU440b/smF/cAV63Y8Ng=\",\"T1\":\"tCDMvp26Fq7TXLtKAQf52FjHwJJc5rWNeoVISmO4iwU=\",\"T2\":\"FbsjaPCjvghxwrn0SI8xGYxQBOJm81tTF0asoryoEDY=\",\"Taux\":\"Hax3hWkUV5b1VGm1dCyC9s96fGygWau1Z8hHMDzAkQw=\",\"Mu\":\"KlXokDBpnFghV7lLCg1Zt2elB1ZTvUjASVRs9FJS/w4=\",\"L\":[\"Z1wwX7oPf+TKbJf1ibyXB/0nSw0JjeG1pmvNi/oUFUQ=\",\"5W+rmUgJyEBi5Gdnq8Jlxh5GVUtw2p6n/bKARbeTkF0=\",\"rCbyzSKCiB+wrPY9nzz7hqO8oSJ3rnuwpGLsNGvxTd8=\",\"iD8q6aZ3+S/psDy2D1FIKw2XQUHRyzq7kVTEYdM2Gc0=\",\"r6N0NJci5Vu9nIkc06aTmq+1e9daYXWF4sgluOCVgaY=\",\"dXx3FM36UunOoUvh4htUr5I898AMxvFk2NbcWFfyy5Y=\"],\"R\":[\"V5mbYI99NhTdso6bJ7iOe7B/3/QeNsbIsKVqJ2Vr6j8=\",\"nTqY5K8dtBvV928ZlNkn65ECera2++TlYLR7Dh0Vmw4=\",\"Bzd2gBYqdMIpHK3iB2kTvqGwk0z9JHtt+C0gUf0I8zE=\",\"Beb1LIQiwAcWqI16sP75mdQFq3THz3ZnqYab6BqEMj0=\",\"gN5q0SSnLUpNf7GMU+EF+TA4wk2dedVhnGOed2KOXFY=\",\"fvy2b4YmOlqWOGVXWGof6lBkKfqqHmL6xptPD6z1/4c=\"],\"Aa\":\"o1QwgazABSWu0s62Oe1pJxYkIyLnkcRpEYxjnQEExw8=\",\"B\":\"g45nZ02Gp+2634hO5vqtdNU6Nv+wHH1cW1cJKCI2Dws=\",\"T\":\"fGRNZAG5sQAHh5BXGKYj+qQEbKhr5/InN1XLokIbwQk=\"}],\"MGs\":[{\"Ss\":[],\"Cc\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"PseudoOuts\":[\"qQ6KifhAibsypLC+nQ2+hSxuVqCx2djQfIOheFsvjek=\"],\"Ss\":[{\"C\":\"tPkScFU4KK3uVPXp12xgzfrmUqW2+zUnNp65E6W38wE=\",\"R\":\"S3a/pqUY+RKg1FjtZftiB39t+jB0CDNqFxUQE0laRAk=\"}]}}}}}}],\"token_output_seqs\":{\"0x0000000000000000000000000000000000000000\":6}}"),
		[]byte("{\"number\":\"0x6\",\"hash\":\"0xf53b9b4e1c5d1147b6bda2f50ea5a345563cf86211d53d7d30af82ad83f69dac\",\"miner\":\"0x0000000000000000000000000000000000000000\",\"timestamp\":\"0x5d774489\",\"parentHash\":\"0xdd4574ae50135ebce00aecc771875a9127949af817b72efc08145edfa72f4533\",\"transactionsRoot\":\"0x7ce39b3a355db0efc4a17b1bc12c2f23ca87adaef0c3b8d96a7c689d2fcf5042\",\"stateRoot\":\"0xb005996fc0cd3fbd8d6b71c371f5f3ef087f0cfb56133f954ec8526d411eecee\",\"receiptsRoot\":\"0xf294e0d58b433da7e1a89cf5570c8f1cb2380ce362da470209352b2395f7c2c9\",\"gasLimit\":\"0x12a05f200\",\"gasUsed\":\"0x1dd50620\",\"logsBloom\":\"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\",\"transactions\":[{\"type\":\"rpctx\",\"value\":{\"txType\":\"utx\",\"txHash\":\"0x7ce39b3a355db0efc4a17b1bc12c2f23ca87adaef0c3b8d96a7c689d2fcf5042\",\"tx\":{\"type\":\"utx\",\"value\":{\"inputs\":[{\"type\":\"UTXOInput\",\"value\":{\"key_offset\":[\"6\"],\"key_image\":\"BqfhPSORM5cYbz6xqT/X/cddqav4p0TcwK3WaclPG2M=\"}}],\"outputs\":[{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"6EDC0lgwuI/W6Ri0+2zWUUe2HxQVN+cN13KShkUkHvU=\",\"amount\":0,\"remark\":\"NpyQ6XJ8Hv6TBvUUc737/ZH4sa1OfL0HQHZpkNoIqPE=\"}},{\"type\":\"AccountOutput\",\"value\":{\"to\":\"0xe50ab035b1cc691b84e415ff0931867f6a71b091\",\"amount\":5000000000000000000,\"data\":\"EjNE\",\"commit\":\"OU5C09tqX4i7YCEKEJqFmAzp5gFakBgSHwCysG+ImCw=\"}},{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"5fxXxPx6SE7YNF+lZjwxNXBN5rJRo/VwXijbNeBK0qE=\",\"amount\":0,\"remark\":\"ea6SelCjkP+urMLCoX7OhljZEsBKouJehVnzDhUlGdk=\"}}],\"token_id\":\"0x0000000000000000000000000000000000000000\",\"r_key\":\"thvHhdlzbEjSQN5B3y536fijxu4PQPZEPY0uHVn/ovY=\",\"add_keys\":[\"u0Xm3YIT2zs0kdcRb73nLqNEefvjU6UOHTqyH6JS850=\",\"SAVL7fQ4jMLFGJgc2pCKuZ6m+IqhozRzF2TbfjRc8SU=\",\"/Z8zJ7X6SKbBEU7FkK81+nUdPXFt2YTErx3tWKvfjSA=\"],\"fee\":50050000000000000000,\"extra\":null,\"signature\":{\"v\":0,\"r\":0,\"s\":0},\"rct_signature\":{\"RctSigBase\":{\"Type\":3,\"PseudoOuts\":[],\"EcdhInfo\":[{\"Mask\":\"bYKZlLqofLqUD31VJ8vUkyV8bTRigcX4p01soqLsFwI=\",\"Amount\":\"wrBJ8bQVA0yjrBTAxtxahLtnJblA8OXr33f1Pn0vFwo=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"},{\"Mask\":\"sCQ+uPSHzk+DQeSZZjlUmZjEUk75S//EHBZqHfPydgc=\",\"Amount\":\"h760dJsJm8FANz7x8o8Z272MBQB4Hdx9GmlE/z6h9AU=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"OutPk\":[{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"xea3JF8+/xAKKJi0zqZn1m470DyiBzZxRFmqwytLQCM=\"},{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"qZ33PFtWqkj5FEIXpFbL7q4HH5EVR6pxdNyHf9B+XVM=\"}],\"TxnFee\":\"0\"},\"P\":{\"RangeSigs\":[],\"Bulletproofs\":[{\"A\":\"MqqR+QdubnffYXPz9rKPktO3qVnDPp009J/owqAMzko=\",\"S\":\"TJzWTZNkoJSdYWmPZ6pECakqT5pUQkMlfWylPnLI90Q=\",\"T1\":\"J0QgBFmjaAto7XziKIeG7lmAeqe8YApklBORt3qJNb0=\",\"T2\":\"S/xkGQYotDqtXVDDj0BhkFh/guTBVrY6DDO2tb5B2mw=\",\"Taux\":\"PGte4pEs2gOuhcZ3/GvzE0z4qFhvxSUYMFgx9l45uQk=\",\"Mu\":\"2ztfMoZNya8zsgzJozrUGogeG0y9xggSxYIXByBR5Ak=\",\"L\":[\"CDqCA9z1uejcPwFzOmS7cGD4vRXsG75WvFFKxRBiIbw=\",\"R35xDwd5ZwjltayfaZw5d0l0QZ9Z6qzyQJUCCTpoZBw=\",\"MDrUwJ2AFoKCB2EjPvDYzuIkB9lII0SBc3ijstV6cbg=\",\"+Ym8DIqPguS3kqw+2ftxLW3ygJGR7+0kjcXYR7Y5opY=\",\"z/JECkhaUowi+bMTMbjYHCZOAbJIEwa5qX7j64DQA9Q=\",\"r9Cw8gQiqIGsW0UJcL2x4effooop43RJAA7QT1bvKq0=\",\"JEAH+fVxaBjjjAejDG6venp7/jzwv2/4qBvvKPiQX3A=\"],\"R\":[\"FgptLoP8NzI//FxEMD8qWe4o95dK+Z2QA6Tn/dlYmbw=\",\"u0UBZaxwVihSh+YKZ7EPNMCPY0WFpspooR848Bl5hzg=\",\"qbDBfEM19udMYG8KsrHZ8crn2bfxnpPpgt7UQxbaRtc=\",\"dWHj9BnMcFTZORFHGqFsT98nHTdjcbXbldfgXEDVIng=\",\"d7+O8sOV7uHnXVLOjURAsiN4odg7UwL6aepExkbjY2Y=\",\"U+0oUOOGrsxcqG07TCDJFulChXuhAmMxYEKy9EfQvJI=\",\"IG+Q/nPdcC1rm8E3z+TYLp74RpQ7zxHqcr1awCiJ+wU=\"],\"Aa\":\"oNv+jNzG+n1YQrzS5R3Z0G+uVTBi0HpbwAnt7LkvtAQ=\",\"B\":\"sKp7qREdQovgFFvmZ8dgy48bSV24rLozFsmoh93osgo=\",\"T\":\"Nuq2v1ABvX5LLnHbQsZg9QRZTpYgBO7tVZsN+wVwhgU=\"}],\"MGs\":[{\"Ss\":[],\"Cc\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"PseudoOuts\":[\"jhD7CROFen1fBUIMN8/7jCD93/1Lk4IBYYt2TEhbf60=\"],\"Ss\":[{\"C\":\"kLLOGDkHrOn7lkOVO2N9LmFTe4lAVjR2PJ5MJCvOQQk=\",\"R\":\"eZ13+x/cgNb7hF34U2d82kTuhaGyqaSYdsd7nzZMGw0=\"}]}}}}}}],\"token_output_seqs\":{\"0x0000000000000000000000000000000000000000\":7}}"),
		[]byte("{\"number\":\"0x7\",\"hash\":\"0x8a2d952dfbc8b31edc720d8169bcf14211ad2be596821024faf19973e0e51af1\",\"miner\":\"0x0000000000000000000000000000000000000000\",\"timestamp\":\"0x5d77448e\",\"parentHash\":\"0xf53b9b4e1c5d1147b6bda2f50ea5a345563cf86211d53d7d30af82ad83f69dac\",\"transactionsRoot\":\"0x85d6f6abf07d7009eebcc442d5e83a9153056057312c2a58593da67fa5969f97\",\"stateRoot\":\"0x4f4af42f588e27cd056476fc5005f81f24eb13763aeccb00ae51421543d69a0a\",\"receiptsRoot\":\"0xc9966ff2cc7d4d89d27ad9659670ee7a95c6fb58289706f949aff99065ef8cc6\",\"gasLimit\":\"0x12a05f200\",\"gasUsed\":\"0x1dcd6500\",\"logsBloom\":\"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\",\"transactions\":[{\"type\":\"rpctx\",\"value\":{\"txType\":\"utx\",\"txHash\":\"0x85d6f6abf07d7009eebcc442d5e83a9153056057312c2a58593da67fa5969f97\",\"tx\":{\"type\":\"utx\",\"value\":{\"inputs\":[{\"type\":\"UTXOInput\",\"value\":{\"key_offset\":[\"7\"],\"key_image\":\"1j31N0VGXDLJnAr8923NxG5s0taW9NzvMRdpi/YXDtE=\"}}],\"outputs\":[{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"EJDJlVFJnvX/MkAu7qXpcwOKo5bnrozXj5bNatWMEuc=\",\"amount\":0,\"remark\":\"cCXxdoxShmyRSIXawn1b8J0zLFqkr7VPJdFHCEjlTAA=\"}},{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"/FuANrJ200l+hk4rU04AE5krWwAAH/2+vn9AUxdIfhw=\",\"amount\":0,\"remark\":\"pdnxk0Nx8kF2Zm3E3xQuRAERmWpuAKBr15UwXXQrzUo=\"}},{\"type\":\"UTXOOutput\",\"value\":{\"otaddr\":\"KMj15oUKkszufxQisum1uYPTrnxMvs6Ma+AtII1S7BM=\",\"amount\":0,\"remark\":\"nw7afd0lDi10mcLiFbbj73VyetFBrDYxg8QFZEaFro8=\"}}],\"token_id\":\"0x0000000000000000000000000000000000000000\",\"r_key\":\"x+2J3a63K1JXnR/j1KZ171Z95JrCUNfyGwsxk6xKF4I=\",\"add_keys\":[\"6plKSaZMiN+Z5A5NkH27O62uBYrMH0B8b4QDUdVVMtM=\",\"1CuAZ7UrKZXmkOT93jTiwMVwPjepkXTxwPwb6+q3vkk=\",\"1CuAZ7UrKZXmkOT93jTiwMVwPjepkXTxwPwb6+q3vkk=\"],\"fee\":50000000000000000000,\"extra\":null,\"signature\":{\"v\":0,\"r\":0,\"s\":0},\"rct_signature\":{\"RctSigBase\":{\"Type\":3,\"PseudoOuts\":[],\"EcdhInfo\":[{\"Mask\":\"ZIDHUe7ey+0McwBDjWrXHBlxzB3TlYOdJ03DjGRBlAo=\",\"Amount\":\"g/VFXXsuR3nX0HlJoDMC6vT2LqV9M9dkiCJIn0agMwA=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"},{\"Mask\":\"QsLDyzPiSje8b8ad7zNxi3nQYxeyrPytI29Vvxdr2QQ=\",\"Amount\":\"m24sPVqapQpTYJFl8RizTjmmjIu6D9zxCbvcvjxjOgA=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"},{\"Mask\":\"igyw5j3+bFx7puYiMCifA/rEJB/ithxdY+cgsOOTtQg=\",\"Amount\":\"3CK6olmkCKeFMhRjSo2AgHLDSH9yl8Zo9S2cfg/UiQ8=\",\"SenderPK\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"OutPk\":[{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"YumAW01fag0CIM+7sDqnnFzfyzjJrJW6P7K4ttXnGjo=\"},{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"7BunseLFc9LULoN19Ff2kQthyiF6JukN1nal6d2/esg=\"},{\"Dest\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"Mask\":\"Kyh78g2UJO3JZzIS8MkyGO4Cxyf7fQ3L/DFPTWavVwY=\"}],\"TxnFee\":\"0\"},\"P\":{\"RangeSigs\":[],\"Bulletproofs\":[{\"A\":\"OUB2DpCoDUAbx2QwKjRqy5E7hwc1+a2o0g1ZO4K5Sp8=\",\"S\":\"u3ba9Tb9Z6F6ZdLNbkwNz5l2SfKQdGHMyZ7vNZ61YkU=\",\"T1\":\"lEG32rlcwi9SU1yUm+TzWOBAwgYdKGI+oQRTD7+lNjo=\",\"T2\":\"Cd1IWHukLoqmi4QXNWgvxzeiv/vOGMnK/y5IQt5PHtw=\",\"Taux\":\"p/I73QZh0BvjZftK0dbNL/VFb9pBMvNngv6nVyO3qwc=\",\"Mu\":\"Qxm/6nJnlLcuX2j0yj6FyFMUazIw2h4WYrxxRJwfow4=\",\"L\":[\"DzeIr+nsnprJElZev8xECj6/78RuTPAVwPvjub9czhI=\",\"hfMWM/d1oGDqgGhPgYm6n+XOyNyJNLqk6NSygebvBpw=\",\"232I0IiLwSdDfDej4xH+KGiWgxSnSFsU/cC/h+Krxj0=\",\"ufYgLkqRbLAVzAeis5qBoGJRjYPgMdNawXIXVeJGr1E=\",\"pfkPCl99lj8JQB0rK/SR4aucZFHy8UoR/4/0RIgrkaY=\",\"64pmU1olotjsjVcgfmHHcuDudslrmt+Aq0blYVt4fIo=\",\"O6GTfk9StQzXuOzBFg1L5D6aAAnk07exrU2bWJe1skA=\",\"Jzq3C/Ng3YIeR7d9+4qmgqKVytL9bxXn/TmR2wCXniM=\"],\"R\":[\"fIsIU7LB2hgtGDnH9NdXi15cBYn78hYbBqYDz7bdchQ=\",\"Kkxks7b6c/TcNArMSJrjDHvjU7CP70M4g98gKaUqevw=\",\"noM87elelXV8wAh9ZvG3KwuJ/PhFalUhpeU7R9I5Iuc=\",\"+2FbShSs8g5GedP1ijVycluUSh+aBCi6+RsrmjG6Guk=\",\"80qILxEsqpnbK2WAyyubIM8X4fRegC6m0bbQdUfmoOs=\",\"q+H/5NNXBh+PSgF+Rkj3JvUJr+owBeSmYOG2ZO8HEE0=\",\"2ncdmsq3RePtKgwE/WQfw9yq38EGumRZEE+OQLld7SU=\",\"Zz4FudUmZR7DS3s4JgX1k47NnBgQZPv+bQWWu1r5UXg=\"],\"Aa\":\"oM9auQfG8ySjilhq+Np5fABVSsnqiaTLS4r6GZ8mywc=\",\"B\":\"CkG0oG3HqhMKFEndV4eiY3a9pPBG7C8mrmC/jr15Ew8=\",\"T\":\"2a9TXlCOfR2YLmnVkU1L4wCCsOCi6pnO5rDq/e2B1gk=\"}],\"MGs\":[{\"Ss\":[],\"Cc\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"}],\"PseudoOuts\":[\"L0vpVNz45XJR5sHY8CRD2xF8b1MOYZBQY8F07w9gUSs=\"],\"Ss\":[{\"C\":\"GXfulk45Ibz3uuofXoIqE2AXmO8Dhnz1epb25OXdYQg=\",\"R\":\"5w26mdX2grGvCViqsAIpVcZASAMJPGRrjkaa3PMQmwQ=\"}]}}}}}}],\"token_output_seqs\":{\"0x0000000000000000000000000000000000000000\":9}}"),
		// []byte(""),
	}
	localHeightExpect := big.NewInt(7)

	type tokenBalanceItem struct {
		token   common.Address
		index   uint64
		balance *big.Int
	}
	b0, _ := new(big.Int).SetString("0x42d0496797df1960000", 0)
	b1, _ := new(big.Int).SetString("0x21f2f6f0fc3c6100000", 0)
	balanceTests := []tokenBalanceItem{
		{
			index:   0,
			token:   LinkToken,
			balance: b0,
		},
		{
			index:   1,
			token:   LinkToken,
			balance: b1,
		},
		{
			index:   2,
			token:   LinkToken,
			balance: big.NewInt(0),
		},
		{
			index:   0,
			token:   mockTokenA,
			balance: big.NewInt(0),
		},
	}

	type outIndexItem struct {
		token common.Address
		index uint64
	}
	indexTests := []outIndexItem{
		{
			token: LinkToken,
			index: 12,
		},
		{
			token: mockTokenA,
			index: 0,
		},
	}

	type keyimageItem struct {
		key   lkctypes.Key
		index uint64
	}
	keyimageTests := []keyimageItem{
		{
			key:   lkctypes.HexToKey("113698837974a49329e27c9cbd2bb0a296da91fda5847b6ee2e5991008851ebd"),
			index: 0,
		},
		{
			key:   lkctypes.HexToKey("0274703d0341e9575183fae3068246b6b9c367423ae51efd701ec8be412c2626"),
			index: 1,
		},
		{
			key:   lkctypes.HexToKey("c198f90b009952d2e7090d24d015fecd5c2e58216776215998ec99eb200b9b36"),
			index: 2,
		},
		{
			key:   lkctypes.HexToKey("3f7f0d1abe4b6a85622f116bc7b914a828110044f6c493ae30c719d2c94e06e5"),
			index: 3,
		},
		{
			key:   lkctypes.HexToKey("f23a2c382019a902707e95d8aa19f7a6c27f454c2723fe7d82c48dafa797f9f3"),
			index: 4,
		},
		{
			key:   lkctypes.HexToKey("bb24639be5b336bad5e8b33c98697b344b03e8c72c3ffe8325a80ba29ea96ccb"),
			index: 5,
		},
		{
			key:   lkctypes.HexToKey("06a7e13d23913397186f3eb1a93fd7fdc75da9abf8a744dcc0add669c94f1b63"),
			index: 6,
		},
		{
			key:   lkctypes.HexToKey("d63df53745465c32c99c0afcf76dcdc46e6cd2d696f4dcef3117698bf6170ed1"),
			index: 7,
		},
		{
			key:   lkctypes.HexToKey("cdac5a422ebe658958cd7b70ba84c31c2c12c6f5544f82f5d3275e566c887568"),
			index: 8,
		},
		{
			key:   lkctypes.HexToKey("544433ccf85f5f6d176f76f37b74d23824f78335a74b570e3686c766f6d39a58"),
			index: 9,
		},
		{
			key:   lkctypes.HexToKey("47b7d3cc9a129b776c8bb610cae34528d8abf32e67577b39096943186fc70a30"),
			index: 10,
		},
		{
			key:   lkctypes.HexToKey("b893e76b50800aa463f2ca8fc00bf23b403bab3401da6c2917be22956dd9980a"),
			index: 11,
		},
		{
			key:   lkctypes.HexToKey("c48a3afc48054aa6689185c1492ed0c607218754bd93942e0f59324cdada97fc"),
			index: 12,
		},
	}

	chainVersionExpect := "0.1.1"

	Convey("test OnStart", t, func() {
		Convey("for RefreshMaxBlock succ", func() {
			resetMockAccount()

			Patch(RefreshMaxBlock, func() (*big.Int, error) {
				return remoteHeightExpect, nil
			})
			defer UnpatchAll()

			Patch(GetBlockUTXOsByNumber, func(height *big.Int) (*rtypes.RPCBlock, error) {
				h := int(height.Int64())
				if h >= len(blocks) {
					return nil, fmt.Errorf("GetBlockUTXOsByNumber fail")
				}
				var block rtypes.RPCBlock
				if err := json.Unmarshal(blocks[h], &block); err != nil {
					return nil, err
				}

				return &block, nil
			})

			Patch(GetChainVersion, func() (string, error) {
				return chainVersionExpect, nil
			})

			err := mockLinkAccount.CreateSubAccount(2)
			So(err, ShouldBeNil)

			mockLinkAccount.OnStart()
			time.Sleep(10 * time.Second)

			lh, rh := mockLinkAccount.GetHeight()
			So(lh.Cmp(localHeightExpect), ShouldEqual, 0)
			So(rh.Cmp(remoteHeightExpect), ShouldEqual, 0)

			for _, test := range balanceTests {
				b := mockLinkAccount.GetBalance(test.index, &test.token)
				So(test.balance.Cmp(b), ShouldEqual, 0)
			}

			for _, test := range indexTests {
				idx := mockLinkAccount.GetGOutIndex(test.token)
				So(idx, ShouldEqual, test.index)
			}

			for _, test := range keyimageTests {
				idx, ok := mockLinkAccount.keyImages[test.key]
				So(ok, ShouldEqual, true)
				So(idx, ShouldEqual, test.index)
			}
			So(len(mockLinkAccount.keyImages), ShouldEqual, len(keyimageTests))

			// Transfers            transferContainer

			status := mockLinkAccount.Status()
			So(status.LocalHeight.ToInt().Cmp(localHeightExpect), ShouldEqual, 0)
			So(status.RemoteHeight.ToInt().Cmp(remoteHeightExpect), ShouldEqual, 0)
			So(status.WalletOpen, ShouldEqual, true)
			So(status.AutoRefresh, ShouldEqual, true)
			So(status.WalletVersion, ShouldEqual, WalletVersion)
			So(status.EthAddress, ShouldEqual, mockLinkAccount.account.EthAddress)
			So(status.ChainVersion, ShouldEqual, chainVersionExpect)

			mockLinkAccount.OnStop()
			So(mockLinkAccount.walletOpen, ShouldEqual, false)
			So(mockLinkAccount.autoRefresh, ShouldEqual, false)
			mainKey := mockLinkAccount.account.GetKeys()
			So(mainKey.SpendSKey, ShouldEqual, lkctypes.SecretKey{})
			So(mainKey.ViewSKey, ShouldEqual, lkctypes.SecretKey{})
		})
	})
}
