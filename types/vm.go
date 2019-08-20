package types

import (
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/common"
    "github.com/lianxiangcloud/linkchain/libs/log"
)

// StateDB is an VM database for full state querying.
type StateDB interface {
	CreateAccount(common.Address)

	SubBalance(common.Address, *big.Int)
	AddBalance(common.Address, *big.Int)
	GetBalance(common.Address) *big.Int

	GetNonce(common.Address) uint64
	SetNonce(common.Address, uint64)

	GetCredits(common.Address) uint64
	SetCredits(common.Address, uint64)

	GetCodeHash(common.Address) common.Hash
	GetContractCode([]byte) []byte
	GetCode(common.Address) []byte
	SetCode(common.Address, []byte)
	GetCodeSize(common.Address) int
	IsContract(common.Address) bool

	AddRefund(uint64)
	GetRefund() uint64

	GetState(common.Address, common.Hash) []byte
	SetState(common.Address, common.Hash, []byte)

	Suicide(common.Address) bool
	HasSuicided(common.Address) bool

	// Exist reports whether the given account exists in state.
	// Notably this should also return true for suicided accounts.
	Exist(common.Address) bool
	// Empty returns whether the given account is empty. Empty
	// is defined according to EIP161 (balance = nonce = code = 0).
	Empty(common.Address) bool

	RevertToSnapshot(int)
	Snapshot() int

	AddLog(*Log)
	AddPreimage(common.Hash, []byte)

	ForEachStorage(common.Address, func(common.Hash, []byte) bool)
	TxHash() common.Hash
	Logs() []*Log

	SubTokenBalance(addr common.Address, token common.Address, amount *big.Int)
	AddTokenBalance(addr common.Address, token common.Address, amount *big.Int)
	GetTokenBalance(addr common.Address, token common.Address) *big.Int
	GetTokenBalances(addr common.Address) TokenValues

    GetCoefficient(logger log.Logger) *Coefficient
}

type ChainContext interface {
	// GetHeader returns the hash corresponding to their hash.
	// GetHeader(uint64) *Header
}

type ContractRef interface {
	// Address() common.Address
}

// type OpCode byte

type Contract interface {
	// AsDelegate() Contract
	// GetOp(n uint64) OpCode
	// GetByte(n uint64) byte
	// Caller() common.Address
	// UseGas(gas uint64) (ok bool)
	// Address() common.Address
	// Value() *big.Int
	// SetCode(hash common.Hash, code []byte)
	// SetCallCode(addr *common.Address, hash common.Hash, code []byte)

	// //
	// GetCode() []byte
	// GetCodeHash() common.Hash
	// GetCodeAddr() *common.Address
	// GetInput() []byte
	// SetInput(input []byte)
	// GetGas() uint64
}

type Interpreter interface {
	// Run(contract Contract, input []byte) (ret []byte, err error)
}

type VmConfig interface {
}

// type (
// 	// CanTransferFunc is the signature of a transfer guard function
// 	CanTransferFunc func(StateDB, common.Address, *big.Int) bool
// 	// TransferFunc is the signature of a transfer function
// 	TransferFunc func(StateDB, common.Address, common.Address, *big.Int)
// 	// GetHashFunc returns the nth block hash in the blockchain
// 	// and is used by the BLOCKHASH EVM op code.
// 	GetHashFunc func(uint64) common.Hash
// )

// Context provides the EVM with auxiliary information. Once provided
// it shouldn't be modified.
type Context interface {
	// // CanTransferFunc is the signature of a transfer guard function
	// CanTransferFunc(StateDB, common.Address, *big.Int) bool
	// // TransferFunc is the signature of a transfer function
	// TransferFunc(StateDB, common.Address, common.Address, *big.Int)
	// // GetHashFunc returns the nth block hash in the blockchain
	// // and is used by the BLOCKHASH EVM op code.
	// GetHashFunc(uint64) common.Hash

	// // Message information
	// // Origin   common.Address // Provides information for ORIGIN
	// // GasPrice *big.Int       // Provides information for GASPRICE

	// // // Block information
	// // Coinbase    common.Address // Provides information for COINBASE
	// // GasLimit    uint64         // Provides information for GASLIMIT
	// // BlockNumber *big.Int       // Provides information for NUMBER
	// // Time        *big.Int       // Provides information for TIME
	// // Difficulty  *big.Int       // Provides information for DIFFICULTY
	// GetOrigin() common.Address
	// GetGasPrice() *big.Int
	// GetCoinbase() common.Address
	// GetGasLimit() uint64
	// GetBlockNumber() *big.Int
	// GetTime() *big.Int
	// GetDifficulty() *big.Int
}
