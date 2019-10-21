package types

import (
	"errors"
)

type mockError struct {
	code    int
	message string
}

func (e *mockError) ErrorCode() int { return e.code }

func (e *mockError) Error() string { return e.message }

func NewMockError(errCode int, msg string) *mockError {
	return &mockError{code: errCode, message: msg}
}

var (
	// ErrInvalidSignParam is returned if the transaction signed with error param.
	ErrInvalidSignParam = errors.New("invalid sign param for signer")

	ErrVtxEmptyPubkey             = NewMockError(-40003, "validator pubkey empty")
	ErrVtxAddressPubkeyNotMatched = NewMockError(-40004, "validator pubkey and address not matched")
	ErrVtxNegativePower           = NewMockError(-40005, "validator negative power")

	ErrParams      = NewMockError(-3001, "invalid params")
	ErrTxEmpty     = NewMockError(-3002, "tx is empty")
	ErrTxSign      = NewMockError(-3003, "tx sign error")
	ErrTxDuplicate = NewMockError(-3013, "tx duplicate cached")

	ErrMempoolIsFull = NewMockError(-3014, "temporary unavailable")

	// ErrInvalidSender is returned if the transaction contains an invalid signature.
	ErrInvalidSender = NewMockError(-3020, "invalid sender")

	// ErrNonceTooHigh is returned if the nonce of a transaction is higher than the
	// next one expected based on the local chain.
	ErrNonceTooHigh = NewMockError(-3011, "nonce too high")

	// ErrNonceTooLow is returned if the nonce of a transaction is lower than the
	// one present in the local chain.
	ErrNonceTooLow = NewMockError(-3010, "nonce too low")

	// ErrUnderpriced is returned if a transaction's gas price is below the minimum
	// configured for the transaction pool.
	ErrUnderpriced = NewMockError(-3023, "transaction underpriced")

	// ErrReplaceUnderpriced is returned if a transaction is attempted to be replaced
	// with a different one without the required price bump.
	ErrReplaceUnderpriced = NewMockError(-3024, "replacement transaction underpriced")

	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrInsufficientFunds = NewMockError(-3012, "insufficient funds for gas * price + value")

	// ErrIntrinsicGas is returned if the transaction is specified to use less gas
	// than required to start the invocation.
	ErrIntrinsicGas = NewMockError(-3025, "intrinsic gas too low")

	// ErrGasLimit is returned if a transaction's requested gas limit exceeds the
	// maximum allowance of the current block.
	ErrGasLimit = NewMockError(-3026, "exceeds block gas limit")
	ErrOutOfGas = NewMockError(-3026, "out of gas")

	// ErrNegativeValue is a sanity error to ensure noone is able to specify a
	// transaction with a negative value.
	ErrNegativeValue = NewMockError(-3027, "negative value")

	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = NewMockError(-3039, "oversized data")

	ErrGasLimitOrGasPrice = NewMockError(-3040, "illegal gasLimit or gasPrice")

	ErrTxNotSupport = NewMockError(-3039, "tx not support")

	ErrGasUsedMismatch = errors.New("block gas used mismatched")
	ErrHashMismatch    = errors.New("block root hash mismatched")

	// ErrGasLimitReached is returned by the gas pool if the amount of gas required
	// by a transaction is higher than what's left in the block.
	ErrGasLimitReached = errors.New("gas limit reached")

	ErrUnknownBlock    = errors.New("unknown Block")
	ErrGetTxsResult    = errors.New("get TxsResult failed")
	ErrInvalidReceiver = errors.New("invalid receiver")
	ErrLoadValidators  = errors.New("load validators failed")
	ErrNotFound        = errors.New("not found")
	ExecutionReverted  = errors.New("vm: execution reverted")

	ErrUtxoTxFeeTooLow     = errors.New("fee too low")
	ErrUtxoTxFeeIllegal    = errors.New("fee illegal")
	ErrUtxoTxInvalidInput  = errors.New("invalid input")
	ErrUtxoTxInvalidOutput = errors.New("invalid output")
	ErrUtxoTxDoubleSpend   = errors.New("double spend")

	ErrBlacklistAddress           = errors.New("blacklist address")
	ErrGenerateProcessTransaction = errors.New("generate process transaction")
)
