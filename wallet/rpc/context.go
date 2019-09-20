//go:generate mockgen -destination mock_wallet.go -package rpc -self_package github.com/lianxiangcloud/linkchain/wallet/rpc github.com/lianxiangcloud/linkchain/wallet/rpc Wallet

package rpc

import (
	"math/big"
	"time"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/libs/common"
	lkctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/wallet/config"
	wtypes "github.com/lianxiangcloud/linkchain/wallet/types"
)

// Context RPC context
type Context struct {
	cfg    *config.Config
	wallet Wallet
	logger log.Logger

	accManager *accounts.Manager
}

func NewContext() *Context {
	return &Context{}
}

func (c *Context) SetAccountManager(am *accounts.Manager) {
	c.accManager = am
}
func (c *Context) SetWallet(w Wallet) {
	c.wallet = w
}

func (c *Context) GetWallet() Wallet {
	return c.wallet
}

func (c *Context) SetLogger(logger log.Logger) {
	c.logger = logger.With("module", "rpc.service")
}

// Wallet wallet
type Wallet interface {
	CreateUTXOTransaction(from common.Address, nonce uint64, subaddrs []uint64, dests []types.DestEntry,
		tokenID common.Address, refundAddr common.Address, extra []byte) ([]*types.UTXOTransaction, error)
	GetBalance(index uint64, token *common.Address, addr *common.Address) (*big.Int, error)
	GetHeight(addr *common.Address) (localHeight *big.Int, remoteHeight *big.Int)
	GetAddress(index uint64, addr *common.Address) (string, error)
	Transfer(txs []string) (ret []wtypes.SendTxRet)
	OpenWallet(walletfile string, password string) error
	CreateSubAccount(maxSub uint64, addr *common.Address) error
	AutoRefreshBlockchain(autoRefresh bool, addr *common.Address) error
	GetAccountInfo(tokenID *common.Address, addr *common.Address) (*wtypes.GetAccountInfoResult, error)
	RescanBlockchain(addr *common.Address) error
	GetWalletEthAddress() (*common.Address, error)
	Status(addr *common.Address) *wtypes.StatusResult
	GetTxKey(hash *common.Hash, addr *common.Address) (*lkctypes.Key, error)
	GetMaxOutput(tokenID common.Address, addr *common.Address) (*hexutil.Uint64, error)
	GetUTXOTx(hash common.Hash, addr *common.Address) (*types.UTXOTransaction, error)
	SelectAddress(addr common.Address) error
	SetRefreshBlockInterval(interval time.Duration, addr *common.Address) error
	LockAccount(addr common.Address) error
	// CheckTxKey(hash *common.Hash, txKey *lkctypes.Key, destAddr string) (*hexutil.Uint64, *hexutil.Big, error)
	//
	GetBlockTransactionCountByNumber(blockNr rpc.BlockNumber) (*hexutil.Uint, error)
	GetBlockTransactionCountByHash(blockHash common.Hash) (*hexutil.Uint, error)
	GetTransactionByBlockNumberAndIndex(blockNr rpc.BlockNumber, index hexutil.Uint) (r interface{}, err error)
	GetTransactionByBlockHashAndIndex(blockHash common.Hash, index hexutil.Uint) (r interface{}, err error)
	GetRawTransactionByBlockNumberAndIndex(blockNr rpc.BlockNumber, index hexutil.Uint) (r hexutil.Bytes, err error)
	GetRawTransactionByBlockHashAndIndex(blockHash common.Hash, index hexutil.Uint) (r hexutil.Bytes, err error)
	GetTransactionCount(address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint64, error)
	GetTransactionByHash(hash common.Hash) (r interface{}, err error)
	GetRawTransactionByHash(hash common.Hash) (r hexutil.Bytes, err error)
	GetTransactionReceipt(hash common.Hash) (r map[string]interface{}, err error)
	//
	EthEstimateGas(args wtypes.CallArgs) (*hexutil.Uint64, error)
	SendRawTransaction(encodedTx hexutil.Bytes) (common.Hash, error)
	SendRawUTXOTransaction(encodedTx hexutil.Bytes) (common.Hash, error)
	GetLocalUTXOTxsByHeight(height *big.Int, addr *common.Address) (*wtypes.UTXOBlock, error)
	GetLocalOutputs(ids []hexutil.Uint64, addr *common.Address) ([]wtypes.UTXOOutputDetail, error)
	GetUTXOAddInfo(hash common.Hash) (*wtypes.UTXOAddInfo, error)
	DelUTXOAddInfo(hash common.Hash) error
}
