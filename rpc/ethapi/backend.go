// Copyright 2015 The go-ethereum Authors
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

// Package ethapi implements the general Ethereum API functions.
package ethapi

import (
	"context"
	"math/big"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/vm"
	"github.com/lianxiangcloud/linkchain/vm/evm"
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
type Backend interface {
	EVMAllowed() bool
	// General Ethereum API
	ProtocolVersion() string
	SuggestPrice(ctx context.Context) (*big.Int, error)
	AccountManager() *accounts.Manager
	Coinbase() common.Address
	//EventMux() *event.TypeMux

	// Consensus API
	StopTheWorld() bool
	StartTheWorld() bool
	ConsensusState() (*rtypes.ResultConsensusState, error)
	DumpConsensusState() (*rtypes.ResultDumpConsensusState, error)
	Validators(heightPtr *uint64) (*rtypes.ResultValidators, error)
	Status() (*rtypes.ResultStatus, error)

	// BlockChain API
	GetTx(hash common.Hash) (types.Tx, *types.TxEntry)
	GetTransactionReceipt(hash common.Hash) (*types.Receipt, common.Hash, uint64, uint64)
	HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error)
	BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error)
	BalanceRecordByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.BlockBalanceRecords, error)
	StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error)
	GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error)
	GetReceipts(ctx context.Context, blockNr uint64) types.Receipts
	GetVM(ctx context.Context, msg types.Message, state *state.StateDB, header *types.Header, vmCfg evm.Config) (vm.VmInterface, func() error, error)
	Block(heightPtr *uint64) (*rtypes.ResultBlock, error)
	GetMaxOutputIndex(ctx context.Context, token common.Address) int64
	GetBlockTokenOutputSeq(ctx context.Context, blockHeight uint64) map[string]int64
	GetOutput(ctx context.Context, token common.Address, index uint64) (*types.UTXOOutputData, error)
	GetUTXOGas() uint64
	GetTxsResult(ctx context.Context, blockNr uint64) (*types.TxsResult, error)

	// TxPool API
	SendTx(ctx context.Context, signedTx types.Tx) error
	GetPoolTx(txHash common.Hash) types.Tx
	GetPoolTransaction(txHash common.Hash) *types.Transaction
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	Stats() (int, int, int)

	// NetAPI
	NetInfo() (*rtypes.ResultNetInfo, error)
	GetSeeds() []rtypes.Node
	PrometheusMetrics() string
}

func GetAPIs(apiBackend Backend) []rpc.API {
	nonceLock := new(AddrLocker)
	return []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicEthereumAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicBlockChainAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(apiBackend, nonceLock),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicAccountAPI(apiBackend.AccountManager()),
			Public:    true,
		}, {
			Namespace: "personal",
			Version:   "1.0",
			Service:   NewPrivateAccountAPI(apiBackend, nonceLock),
			Public:    false,
		}, {
			Namespace: "txpool",
			Version:   "1.0",
			Service:   NewPublicMempoolAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   NewPublicNetAPI(apiBackend, types.SignParam.Uint64()),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicPrometheusMetricsAPI(apiBackend),
			Public:    true,
		},
	}
}
