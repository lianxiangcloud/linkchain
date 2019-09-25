package rpc

import (
	"context"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
	"github.com/lianxiangcloud/linkchain/types"
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

// sign is a helper function that signs a transaction with the private key of the given address.
func (s *PublicTransactionPoolAPI) sign(addr common.Address, tx types.Tx) (types.Tx, error) {
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: addr}

	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return nil, wtypes.ErrAccountNotFound
	}
	// Request the wallet to sign the transaction
	tx1, err := wallet.SignTx(account, tx, types.SignParam)
	if err != nil {
		return nil, wtypes.ErrSignTx
	}
	return tx1, nil
}

func (s *PublicTransactionPoolAPI) SignTransaction(ctx context.Context, args rtypes.SendTxArgs) (*rtypes.SignTransactionResult, error) {
	if args.Gas == nil {
		//return nil, fmt.Errorf("gas not specified")
		return nil, wtypes.ErrArgsInvalid
	}
	if args.GasPrice == nil {
		//return nil, fmt.Errorf("gasPrice not specified")
		return nil, wtypes.ErrArgsInvalid
	}
	if args.Nonce == nil {
		//return nil, fmt.Errorf("nonce not specified")
		return nil, wtypes.ErrArgsInvalid
	}

	tx, err := s.sign(args.From, args.ToTransaction())
	if err != nil {
		return nil, err
	}
	data, err := ser.EncodeToBytes(tx)
	if err != nil {
		return nil, wtypes.ErrInnerServer
	}
	return &rtypes.SignTransactionResult{data, tx}, nil
}

func (s *PublicTransactionPoolAPI) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	return s.wallet.SendRawTransaction(encodedTx)
}

func (s *PublicTransactionPoolAPI) SendRawUTXOTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	return s.wallet.SendRawUTXOTransaction(encodedTx)
}

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
