package ethapi

import (
	"context"
	"fmt"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
	"github.com/lianxiangcloud/linkchain/types"
)

// PublicTransactionPoolAPI exposes methods for the RPC interface
type PublicTransactionPoolAPI struct {
	b         Backend
	nonceLock *AddrLocker
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewPublicTransactionPoolAPI(b Backend, nonceLock *AddrLocker) *PublicTransactionPoolAPI {
	return &PublicTransactionPoolAPI{b, nonceLock}
}

// GetBlockTransactionCountByNumber returns the number of transactions in the block with the given block number.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) *hexutil.Uint {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		n := hexutil.Uint(len(block.Txs))
		return &n
	}
	return nil
}

// GetBlockTransactionCountByHash returns the number of transactions in the block with the given hash.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) *hexutil.Uint {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		n := hexutil.Uint(len(block.Txs))
		return &n
	}
	return nil
}

// newRPCTransactionFromBlockIndex returns a transaction that will serialize to the RPC representation.
func newRPCTransactionFromBlockIndex(b *types.Block, index uint64) interface{} {
	var tx types.Tx
	txs := b.Txs
	if index >= uint64(len(txs)) {
		log.Warn("ethapi: invalid index", "index", index, "len", len(txs))
	} else {
		tx = txs[index]
	}

	if tx == nil {
		return nil
	}

	entry := &types.TxEntry{
		BlockHash:   b.Hash(),
		BlockHeight: b.HeightU64(),
		Index:       index,
	}
	return rtypes.NewRPCTx(tx, entry)
}

// newRPCRawTransactionFromBlockIndex returns the bytes of a transaction given a block and a transaction index.
func newRPCRawTransactionFromBlockIndex(b *types.Block, index uint64) hexutil.Bytes {
	var tx types.Tx

	txs := b.Txs
	if index < uint64(len(txs)) {
		tx = txs[index]
	}

	if tx == nil {
		return nil
	}

	blob, _ := ser.EncodeToBytes(tx)
	return blob
}

// GetTransactionByBlockNumberAndIndex returns the transaction for the given block number and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) interface{} {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		return newRPCTransactionFromBlockIndex(block, uint64(index))
	}
	return nil
}

// GetTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) interface{} {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		return newRPCTransactionFromBlockIndex(block, uint64(index))
	}
	return nil
}

// GetRawTransactionByBlockNumberAndIndex returns the bytes of the transaction for the given block number and index.
func (s *PublicTransactionPoolAPI) GetRawTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) hexutil.Bytes {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		return newRPCRawTransactionFromBlockIndex(block, uint64(index))
	}
	return nil
}

// GetRawTransactionByBlockHashAndIndex returns the bytes of the transaction for the given block hash and index.
func (s *PublicTransactionPoolAPI) GetRawTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) hexutil.Bytes {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		return newRPCRawTransactionFromBlockIndex(block, uint64(index))
	}
	return nil
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number
func (s *PublicTransactionPoolAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint64, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	nonce := state.GetNonce(address)
	return (*hexutil.Uint64)(&nonce), state.Error()
}

// GetTransactionByHash returns the transaction for the given hash
func (s *PublicTransactionPoolAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) interface{} {
	// Try to return an already finalized transaction
	var (
		tx      types.Tx
		txentry *types.TxEntry
	)
	//tx, blockHash, blockNumber, index = s.b.GetTx(hash)
	tx, txentry = s.b.GetTx(hash)
	if tx == nil {
		tx = s.b.GetPoolTx(hash)
	}
	if tx == nil {
		return nil
	}
	return rtypes.NewRPCTx(tx, txentry)
}

// GetRawTransactionByHash returns the bytes of the transaction for the given hash.
func (s *PublicTransactionPoolAPI) GetRawTransactionByHash(ctx context.Context, hash common.Hash) (hexutil.Bytes, error) {
	var tx types.Tx

	// Retrieve a finalized transaction, or a pooled otherwise
	if tx, _ = s.b.GetTx(hash); tx == nil {
		if tx = s.b.GetPoolTx(hash); tx == nil {
			// Transaction not found anywhere, abort
			return nil, nil
		}
	}

	// Serialize to SER and return
	return ser.EncodeToBytes(tx)
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (s *PublicTransactionPoolAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	tx, txentry := s.b.GetTx(hash)
	if tx == nil {
		log.Warn("GetTx nil", "hash", hash)
		return nil, nil
	}

	receipts := s.b.GetReceipts(ctx, txentry.BlockHeight)
	if txentry.Index >= uint64(len(receipts)) {
		log.Warn("GetReceipt nil", "hash", hash, "txentry", *txentry)
		return nil, nil
	}
	receipt := receipts[txentry.Index]

	fields := map[string]interface{}{
		"blockHash":         txentry.BlockHash,
		"blockNumber":       hexutil.Uint64(txentry.BlockHeight),
		"transactionHash":   hash,
		"transactionIndex":  hexutil.Uint64(txentry.Index),
		"gasUsed":           hexutil.Uint64(receipt.GasUsed),
		"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
		"logs":              receipt.Logs,
		"logsBloom":         receipt.Bloom,
		"status":            hexutil.Uint(receipt.Status),
	}

	if receipt.VMErr != "" {
		fields["vmerr"] = receipt.VMErr
	}

	switch itx := tx.(type) {
	case *types.UTXOTransaction:
		fields["from"], _ = itx.From()
		fields["to"] = itx.ToAddrs()
		fields["tokenAddress"] = itx.TokenAddress()
	case types.RegularTx:
		fields["from"], _ = itx.From()
		fields["to"] = itx.To()
		fields["tokenAddress"] = itx.TokenAddress()
	default:
		log.Warn("GetTransactionReceipt tx no receipt", "hash", hash)
		return nil, nil
	}

	// Assign receipt status or post state.
	if len(receipt.PostState) > 0 {
		fields["root"] = hexutil.Bytes(receipt.PostState)
	}

	if receipt.Logs == nil {
		fields["logs"] = [][]*types.Log{}
	}
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != common.EmptyAddress {
		fields["contractAddress"] = receipt.ContractAddress
	}
	return fields, nil
}

// sign is a helper function that signs a transaction with the private key of the given address.
func (s *PublicTransactionPoolAPI) sign(addr common.Address, tx types.Tx) (types.Tx, error) {
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: addr}

	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return nil, err
	}
	// Request the wallet to sign the transaction
	return wallet.SignTx(account, tx, types.SignParam)
}

// submitTransaction is a helper function that submits tx to txPool and logs a message.
func submitTransaction(ctx context.Context, b Backend, tx types.Tx) (common.Hash, error) {
	if err := b.SendTx(ctx, tx); err != nil {
		log.Info("ethapi: Submit transaction failed", "fullhash", tx.Hash().Hex(), "err", err)
		return common.EmptyHash, err
	}
	log.Trace("ethapi: Submitted transaction", "fullhash", tx.Hash().Hex())
	return tx.Hash(), nil
}

// SendTransaction creates a transaction for the given argument, sign it and submit it to the
// transaction pool.
func (s *PublicTransactionPoolAPI) SendTransaction(ctx context.Context, args rtypes.SendTxArgs) (common.Hash, error) {
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: args.From}

	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return common.EmptyHash, err
	}

	if args.Nonce == nil {
		// Hold the addresse's mutex around signing to prevent concurrent assignment of
		// the same nonce to multiple accounts.
		s.nonceLock.LockAddr(args.From)
		defer s.nonceLock.UnlockAddr(args.From)
	}

	// Set some sanity defaults and terminate on failure
	if err := args.SetDefaults(ctx, s.b); err != nil {
		return common.EmptyHash, err
	}
	// Assemble the transaction and sign with the wallet
	tx := args.ToTransaction()
	signed, err := wallet.SignTx(account, tx, types.SignParam)
	if err != nil {
		return common.EmptyHash, err
	}
	return submitTransaction(ctx, s.b, signed)
}

// SendRawTokenTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicTransactionPoolAPI) SendRawTokenTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	return s.SendRawTx(ctx, encodedTx, types.TxToken)
}

// SendRawTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicTransactionPoolAPI) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	return s.SendRawTx(ctx, encodedTx, types.TxNormal)
}

// SendRawUTXOTransaction add the signed UTXO transaction to the transaction pool.
func (s *PublicTransactionPoolAPI) SendRawUTXOTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	return s.SendRawTx(ctx, encodedTx, types.TxUTXO)
}

// SendRawTx will add the signed tx to the transaction pool.
func (s *PublicTransactionPoolAPI) SendRawTx(ctx context.Context, encodedTx hexutil.Bytes, txType string) (common.Hash, error) {
	var tx types.Tx
	switch txType {
	case types.TxNormal:
		tx = new(types.Transaction)
	case types.TxToken:
		tx = new(types.TokenTransaction)
	case types.TxContractCreate:
		tx = new(types.ContractCreateTx)
	case types.TxContractUpgrade:
		tx = new(types.ContractUpgradeTx)
	case types.TxMultiSignAccount:
		tx = new(types.MultiSignAccountTx)
	case types.TxUTXO:
		tx = new(types.UTXOTransaction)
	default:
		return common.EmptyHash, types.ErrTxNotSupport
	}

	if err := ser.DecodeBytes(encodedTx, tx); err != nil {
		log.Info("SendRawTx", "err", err)
		return common.EmptyHash, err
	}

	return submitTransaction(ctx, s.b, tx)
}

// @Todo: implement it
func signHash(d []byte) []byte {
	return d
}

// Sign calculates an ECDSA signature for:
// keccack256("\x19Ethereum Signed Message:\n" + len(message) + message).
//
// Note, the produced signature conforms to the secp256k1 curve R, S and V values,
// where the V value will be 27 or 28 for legacy reasons.
//
// The account associated with addr must be unlocked.
//
// https://github.com/ethereum/wiki/wiki/JSON-RPC#eth_sign
func (s *PublicTransactionPoolAPI) Sign(addr common.Address, data hexutil.Bytes) (hexutil.Bytes, error) {
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: addr}

	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return nil, err
	}
	// Sign the requested hash with the wallet
	signature, err := wallet.SignHash(account, signHash(data))
	if err == nil {
		signature[64] += 27 // Transform V from 0/1 to 27/28 according to the yellow paper
	}
	return signature, err
}

// SignTransaction will sign the given transaction with the from account.
// The node needs to have the private key of the account corresponding with
// the given from address and it needs to be unlocked.
func (s *PublicTransactionPoolAPI) SignTransaction(ctx context.Context, args rtypes.SendTxArgs) (*rtypes.SignTransactionResult, error) {
	if args.Gas == nil {
		return nil, fmt.Errorf("gas not specified")
	}
	if args.GasPrice == nil {
		return nil, fmt.Errorf("gasPrice not specified")
	}
	if args.Nonce == nil {
		return nil, fmt.Errorf("nonce not specified")
	}
	if err := args.SetDefaults(ctx, s.b); err != nil {
		return nil, err
	}
	tx, err := s.sign(args.From, args.ToTransaction())
	if err != nil {
		return nil, err
	}
	data, err := ser.EncodeToBytes(tx)
	if err != nil {
		return nil, err
	}
	return &rtypes.SignTransactionResult{data, tx}, nil
}

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
