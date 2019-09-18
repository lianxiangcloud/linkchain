package rtypes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/types"
)

type specTxArgs interface {
	toTransaction() types.Tx
}

var (
	_ specTxArgs = &sendContractCreateTxArgs{}
)

type SendSpecTxArgs struct {
	Args    json.RawMessage  `json:"args"`
	TxType  string           `json:"type"`
	Signers []common.Address `json:"signers"`
}

func (args SendSpecTxArgs) ToTransaction() (types.Tx, error) {
	var raw interface{}
	switch args.TxType {
	case types.TxContractCreate:
		raw = &sendContractCreateTxArgs{}
	case types.TxContractUpgrade:
		raw = &sendContractUpgradeTxArgs{}
	default:
		return nil, types.ErrParams
	}
	var err error
	if err = json.Unmarshal(args.Args, &raw); err != nil {
		return nil, err
	}

	txArgs, ok := raw.(specTxArgs)
	if !ok {
		return nil, types.ErrParams
	}
	return txArgs.toTransaction(), nil
}

type strVals struct {
	Pubkey string `json:"pubkey"`
	Power  int64  `json:"power"`
}

type sendContractCreateTxArgs struct {
	FromAddr     common.Address  `json:"from"`
	AccountNonce *hexutil.Uint64 `json:"nonce"`
	Amount       *hexutil.Big    `json:"value" `
	Payload      *hexutil.Bytes  `json:"data"`
	GasLimit     *hexutil.Uint64 `json:"gas"`
	Price        *hexutil.Big    `json:"gasPrice"`
}

func (args sendContractCreateTxArgs) toTransaction() types.Tx {
	var input []byte
	input = *args.Payload
	return &types.ContractCreateTx{
		ContractCreateMainInfo: types.ContractCreateMainInfo{
			FromAddr:     args.FromAddr,
			AccountNonce: uint64(*args.AccountNonce),
			Amount:       (*big.Int)(args.Amount),
			Payload:      input,
		},
	}
}

type sendContractUpgradeTxArgs struct {
	FromAddr     common.Address  `json:"from"`
	Recipient    common.Address  `json:"contract"`
	AccountNonce *hexutil.Uint64 `json:"nonce"`
	Payload      *hexutil.Bytes  `json:"data"`
}

func (args sendContractUpgradeTxArgs) toTransaction() types.Tx {
	var input []byte
	input = *args.Payload
	return &types.ContractUpgradeTx{
		ContractUpgradeMainInfo: types.ContractUpgradeMainInfo{
			FromAddr:     args.FromAddr,
			Recipient:    args.Recipient,
			AccountNonce: uint64(*args.AccountNonce),
			Payload:      input,
		},
	}
}

// SendTxArgs represents the arguments to sumbit a new transaction into the transaction pool.
type SendTxArgs struct {
	From         common.Address  `json:"from"`
	TokenAddress common.Address  `json:"tokenAddress"`
	To           *common.Address `json:"to"`
	Gas          *hexutil.Uint64 `json:"gas"`
	GasPrice     *hexutil.Big    `json:"gasPrice"`
	Value        *hexutil.Big    `json:"value"`
	Nonce        *hexutil.Uint64 `json:"nonce"`
	// We accept "data" and "input" for backwards-compatibility reasons. "input" is the
	// newer name and should be preferred by clients.
	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input"`
}

// setDefaults is a helper function that fills in default values for unspecified tx fields.
func (args *SendTxArgs) SetDefaults(ctx context.Context, b Backend) error {
	if args.To == nil && args.TokenAddress != common.EmptyAddress {
		return errors.New("can not create contract with token amount")
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
	if args.Gas == nil {
		args.Gas = new(hexutil.Uint64)
		*(*uint64)(args.Gas) = types.CalNewAmountGas(args.Value.ToInt(), types.EverLiankeFee)
	}
	if args.GasPrice == nil {
		price, err := b.SuggestPrice(ctx)
		if err != nil {
			return err
		}
		args.GasPrice = (*hexutil.Big)(price)
	}
	if args.Nonce == nil {
		nonce, err := b.GetPoolNonce(ctx, args.From)
		if err != nil {
			return err
		}
		args.Nonce = (*hexutil.Uint64)(&nonce)
	}
	if args.Data != nil && args.Input != nil && !bytes.Equal(*args.Data, *args.Input) {
		return errors.New(`Both "data" and "input" are set and not equal. Please use "input" to pass transaction call data.`)
	}
	return nil
}

func (args *SendTxArgs) ToTransaction() types.Tx {
	var input []byte
	if args.Data != nil {
		input = *args.Data
	} else if args.Input != nil {
		input = *args.Input
	}
	if args.TokenAddress != common.EmptyAddress {
		return types.NewTokenTransaction(args.TokenAddress, uint64(*args.Nonce), *args.To, (*big.Int)(args.Value), uint64(*args.Gas), (*big.Int)(args.GasPrice), input)
	}
	if args.To == nil {
		return types.NewContractCreation(uint64(*args.Nonce), (*big.Int)(args.Value), uint64(*args.Gas), (*big.Int)(args.GasPrice), input)
	}
	return types.NewTransaction(uint64(*args.Nonce), *args.To, (*big.Int)(args.Value), uint64(*args.Gas), (*big.Int)(args.GasPrice), input)
}
