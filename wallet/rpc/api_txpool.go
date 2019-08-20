package rpc

import (
	"context"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	wtypes "github.com/lianxiangcloud/linkchain/wallet/types"
)

// GetBlockTransactionCountByNumber returns the number of transactions in the block with the given block number.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*hexutil.Uint, error) {
	return s.wallet.GetBlockTransactionCountByNumber(blockNr)
}

// GetBlockTransactionCountByHash returns the number of transactions in the block with the given hash.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (*hexutil.Uint, error) {
	return s.wallet.GetBlockTransactionCountByHash(blockHash)
}

// GetTransactionByBlockNumberAndIndex returns the transaction for the given block number and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (interface{}, error) {
	return s.wallet.GetTransactionByBlockNumberAndIndex(blockNr, index)
}

// GetTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (interface{}, error) {
	return s.wallet.GetTransactionByBlockHashAndIndex(blockHash, index)
}

// GetRawTransactionByBlockNumberAndIndex returns the bytes of the transaction for the given block number and index.
func (s *PublicTransactionPoolAPI) GetRawTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (hexutil.Bytes, error) {
	return s.wallet.GetRawTransactionByBlockNumberAndIndex(blockNr, index)
}

// GetRawTransactionByBlockHashAndIndex returns the bytes of the transaction for the given block hash and index.
func (s *PublicTransactionPoolAPI) GetRawTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (hexutil.Bytes, error) {
	return s.wallet.GetRawTransactionByBlockHashAndIndex(blockHash, index)
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number
func (s *PublicTransactionPoolAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint64, error) {
	return s.wallet.GetTransactionCount(address, blockNr)
}

// GetTransactionByHash returns the transaction for the given hash
func (s *PublicTransactionPoolAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (interface{}, error) {
	return s.wallet.GetTransactionByHash(hash)
}

// GetRawTransactionByHash returns the bytes of the transaction for the given hash.
func (s *PublicTransactionPoolAPI) GetRawTransactionByHash(ctx context.Context, hash common.Hash) (hexutil.Bytes, error) {
	return s.wallet.GetRawTransactionByHash(hash)
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (s *PublicTransactionPoolAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	return s.wallet.GetTransactionReceipt(hash)
}

// EstimateGas return gas
func (s *PublicTransactionPoolAPI) EstimateGas(ctx context.Context, args wtypes.CallArgs) (*hexutil.Uint64, error) {
	return s.wallet.EthEstimateGas(args)
}

// func (s *PublicTransactionPoolAPI) SignTransaction(ctx context.Context, args rtypes.SendTxArgs) (*rtypes.SignTransactionResult, error) {

// }

// func (s *PublicTransactionPoolAPI) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {

// }

/*
func (s *PublicTransactionPoolAPI) SignSpecTx(ctx context.Context, args rtypes.SendSpecTxArgs) (*rtypes.SignTransactionResult, error) {
	tx, err := args.ToTransaction()
	if err != nil {
		return nil, err
	}
	for _, addr := range args.Signers {
		if tx, err = s.sign(addr, tx); err != nil {
			return nil, err
		}
	}

	data, err := ser.EncodeToBytes(tx)
	if err != nil {
		return nil, err
	}
	return &rtypes.SignTransactionResult{data, tx}, nil
}
*/
