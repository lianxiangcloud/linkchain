package ethapi

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/lianxiangcloud/linkchain/app"
	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/math"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/version"
	"github.com/lianxiangcloud/linkchain/vm/evm"
	"golang.org/x/sync/semaphore"
)

// PublicBlockChainAPI provides an API to access the Ethereum blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockChainAPI struct {
	b   Backend
	sem *semaphore.Weighted
}

// NewPublicBlockChainAPI creates a new Ethereum blockchain API.
func NewPublicBlockChainAPI(b Backend) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{
		b: b,
	}
}

func (s *PublicBlockChainAPI) StopTheWorld() bool {
	return s.b.StopTheWorld()
}

func (s *PublicBlockChainAPI) StartTheWorld() bool {
	return s.b.StartTheWorld()
}

func (s *PublicBlockChainAPI) ConsensusState() (*rtypes.ResultConsensusState, error) {
	// Get self round state.
	return s.b.ConsensusState()
}

func (s *PublicBlockChainAPI) DumpConsensusState() (*rtypes.ResultDumpConsensusState, error) {
	return s.b.DumpConsensusState()
}

func (s *PublicBlockChainAPI) Validators(ctx context.Context, number rpc.BlockNumber) (*rtypes.ResultValidators, error) {
	if number == rpc.LatestBlockNumber || number == rpc.PendingBlockNumber {
		return s.b.Validators(nil)
	}
	height := uint64(number.Int64())
	return s.b.Validators(&height)
}

func (s *PublicBlockChainAPI) Status() (*rtypes.ResultStatus, error) {
	return s.b.Status()
}

// BlockNumber returns the block number of the chain head.
func (s *PublicBlockChainAPI) BlockNumber() *big.Int {
	header, _ := s.b.HeaderByNumber(context.Background(), rpc.LatestBlockNumber) // latest header should always be available
	return big.NewInt(int64(header.Height))
}

// GenesisBlockNumber returns the genesis block number
func (s *PublicBlockChainAPI) GenesisBlockNumber(ctx context.Context) hexutil.Uint64 {
	return hexutil.Uint64(types.BlockHeightZero)
}

// Get current black list
func (s *PublicBlockChainAPI) Blacklist() []common.Address {
	return types.BlacklistInstance.GetBlackAddrs()
}

// Check address is black address
func (s *PublicBlockChainAPI) IsBlackAddress(ctx context.Context, address common.Address) bool {
	return types.BlacklistInstance.IsBlackAddress(address)
}

// GetBalance returns the amount of wei for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicBlockChainAPI) GetBalance(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*big.Int, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	b := state.GetBalance(address)
	return b, state.Error()
}

//GetTokenBalance GetBalance of token.
func (s *PublicBlockChainAPI) GetTokenBalance(ctx context.Context, address common.Address, blockNr rpc.BlockNumber, tokenAddress common.Address) (*big.Int, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	b := state.GetTokenBalance(address, tokenAddress)
	return b, state.Error()
}

//GetAllBalances GetBalance of all tokens.
func (s *PublicBlockChainAPI) GetAllBalances(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (map[common.Address]*hexutil.Big, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	b := make(map[common.Address]*hexutil.Big)
	tv := state.GetTokenBalances(address)
	for i := 0; i < len(tv); i++ {
		b[tv[i].TokenAddr] = (*hexutil.Big)(tv[i].Value)
	}
	return b, state.Error()
}

//GetTxsResult get txsResult
func (s *PublicBlockChainAPI) GetTxsResult(ctx context.Context, blockNr uint64) (*types.TxsResult, error) {
	return s.b.GetTxsResult(ctx, blockNr)
}

// GetBlockByNumber returns the requested block. When blockNr is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByNumber(ctx context.Context, blockNr rpc.BlockNumber, fullTx bool) (interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	if block != nil {
		response := rtypes.NewRPCBlock(block, true, fullTx)
		if blockNr == rpc.PendingBlockNumber {
			// Pending blocks need to nil out a few fields
			response.Coinbase = nil
			response.Hash = nil
		}
		return response, err
	}
	return nil, err
}

func (s *PublicBlockChainAPI) GetBlockBalanceRecordsByNumber(ctx context.Context, blockNr rpc.BlockNumber) (interface{}, error) {
	bbr, err := s.b.BalanceRecordByNumber(ctx, blockNr)
	if err != nil {
		return nil, err
	}
	response := rtypes.NewRPCBlockBalanceRecord(bbr)
	if blockNr == rpc.PendingBlockNumber {
		response.BlockHash = common.EmptyHash
	}
	return response, nil
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (interface{}, error) {
	block, err := s.b.GetBlock(ctx, blockHash)
	if block != nil {
		return rtypes.NewRPCBlock(block, true, fullTx), nil
	}
	return nil, err
}

// GetCode returns the code stored at the given address in the state for the given block number.
func (s *PublicBlockChainAPI) GetCode(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(address)
	return code, state.Error()
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta block
// numbers are also allowed.
func (s *PublicBlockChainAPI) GetStorageAt(ctx context.Context, address common.Address, key string, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	res := state.GetState(address, common.HexToHash(key))
	return res[:], state.Error()
}

func (s *PublicBlockChainAPI) GetStorageRoot(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (string, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return "", err
	}
	res := state.GetStorageRoot(address)
	return res.String(), state.Error()
}

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From         common.Address     `json:"from"`
	TokenAddress common.Address     `json:"tokenAddress"`
	To           *common.Address    `json:"to"`
	Gas          hexutil.Uint64     `json:"gas"`
	GasPrice     hexutil.Big        `json:"gasPrice"`
	Value        hexutil.Big        `json:"value"`
	Data         hexutil.Bytes      `json:"data"`
	Nonce        hexutil.Uint64     `json:"nonce"`
	UTXOKind     types.UTXOKind     `json:"utxokind"`
	Outputs      []types.OutputData `json:"outputs"`
}

func (s *PublicBlockChainAPI) doCall(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber, vmCfg evm.Config, timeout time.Duration) ([]byte, uint64, uint64, bool, error) {
	defer func(start time.Time) { log.Debug("Executing VM call finished", "runtime", time.Since(start)) }(time.Now())

	state, header, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, 0, 0, false, err
	}

	if args.To != nil && !state.IsContract(*args.To) {
		var gasFee uint64
		if common.IsLKC(args.TokenAddress) {
			gasFee = types.CalNewAmountGas(args.Value.ToInt(), types.EverLiankeFee)
		} else {
			gasFee = uint64(types.MinGasLimit)
		}
		return nil, gasFee, 0, false, nil
	}

	// Set sender address or use a default if none specified
	addr := args.From
	if addr == common.EmptyAddress {
		if wallets := s.b.AccountManager().Wallets(); len(wallets) > 0 {
			if accounts := wallets[0].Accounts(); len(accounts) > 0 {
				addr = accounts[0].Address
			}
		}
	}
	incrBalance := new(big.Int).Mul(big.NewInt(types.MaxGasLimit), big.NewInt(types.MaxFeeCounts))
	incrBalanceWei := new(big.Int).Mul(incrBalance, big.NewInt(types.GasPrice))
	state.AddBalance(addr, incrBalanceWei)

	// Set default gas & gas price if none were set
	gas, gasPrice := uint64(args.Gas), args.GasPrice.ToInt()
	if gas == 0 {
		gas = math.MaxUint64 / 2
	}
	if gasPrice.Sign() == 0 {
		gasPrice = new(big.Int).SetUint64(defaultGasPrice)
	}
	nonce := uint64(args.Nonce)
	if nonce == 0 {
		nonce = state.GetNonce(addr)
	}

	// Create new call message
	msg := types.NewMessage(addr, args.To, args.TokenAddress, nonce, args.Value.ToInt(), gas, gasPrice, args.Data)
	if args.To == nil {
		msg.SetTxType(types.TxNormal)
	}
	if args.UTXOKind != types.IllKind {
		msg.SetTxType(types.TxUTXO)
		msg.SetUTXOKind(args.UTXOKind)
		msg.SetOutputData(args.Outputs)
	}

	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	// Get a new instance of the EVM.
	vmenv, vmError, err := s.b.GetVM(ctx, msg, state, header, vmCfg)
	if err != nil {
		return nil, 0, 0, false, err
	}

	// Wait for the context to be done and cancel the evm. Even if the
	// EVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		vmenv.Cancel()
	}()

	var (
		res         []byte
		vmerr       error
		byteCodeGas uint64
	)

	vmenv.SetToken(msg.TokenAddress())
	res, gas, byteCodeGas, _, vmerr, err = app.ApplyMessage(vmenv, msg, args.TokenAddress)

	if err := vmError(); err != nil {
		return nil, 0, 0, false, err
	}
	if vmerr != nil || err != nil {
		log.Debug("PublicBlockChainAPI ApplyMessage", "from", args.From, "to", args.To, "gas", args.Gas, "value", args.Value, "err", err, "vmerr", vmerr)
	}
	return res, gas, byteCodeGas, vmerr != nil, err
}

// Call executes the given transaction on the state for the given block number.
// It doesn't make and changes in the state/blockchain and is useful to execute and retrieve values.
func (s *PublicBlockChainAPI) Call(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	if !s.b.EVMAllowed() {
		return nil, types.ErrMempoolIsFull
	}
	result, _, _, _, err := s.doCall(ctx, args, blockNr, evm.Config{}, 5*time.Second)
	return (hexutil.Bytes)(result), err
}

// TokenCall executes the given tokenTransaction on the state for the given block number.
// It doesn't make and changes in the state/blockchain and is useful to execute and retrieve values.
func (s *PublicBlockChainAPI) TokenCall(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	return s.Call(ctx, args, blockNr)
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicBlockChainAPI) EstimateGas(ctx context.Context, args CallArgs) (hexutil.Uint64, error) {
	if !s.b.EVMAllowed() {
		return hexutil.Uint64(0), types.ErrMempoolIsFull
	}

	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		//lo  uint64 = cfg.TxGas - 1
		hi uint64
		//cap uint64
	)
	if uint64(args.Gas) >= cfg.TxGas {
		hi = uint64(args.Gas)
	} else {
		// Retrieve the current pending block to act as the gas ceiling
		args.Gas = hexutil.Uint64(types.MaxGasLimit) * hexutil.Uint64(types.MaxFeeCounts)
		hi = uint64(types.MaxGasLimit) * uint64(types.MaxFeeCounts)
	}
	//cap = hi

	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) (bool, uint64, uint64, error) {
		args.Gas = hexutil.Uint64(gas)

		_, gasUsed, byteCodeGas, failed, err := s.doCall(ctx, args, rpc.PendingBlockNumber, evm.Config{}, 0)
		if err != nil || failed {
			log.Error("estimate gas failed", "err", err)
			return false, gasUsed, byteCodeGas, err
		}
		return true, gasUsed, byteCodeGas, nil
	}
	ok, gasUsed, extraByteCodeGas, err := executable(hi)
	if !ok {
		if err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("gas required exceeds allowance or always failing transaction")
	}
	estimateGas := gasUsed
	if extraByteCodeGas > 0 {
		maxCallGas := 10 * cfg.CallNewAccountGas
		if extraByteCodeGas > maxCallGas {
			estimateGas += maxCallGas
		} else {
			estimateGas += extraByteCodeGas
		}
	}
	log.Debug("EstimateGas", "gasUsed", gasUsed, "extraByteCodeGas", extraByteCodeGas, "estimateGas", estimateGas)
	return hexutil.Uint64(estimateGas), nil
}

// GetMaxOutputIndex get max UTXO output index by token
func (s *PublicBlockChainAPI) GetMaxOutputIndex(ctx context.Context, token common.Address) hexutil.Uint64 {
	return hexutil.Uint64(uint64(s.b.GetMaxOutputIndex(ctx, token)))
}

type OutputArg struct {
	Token common.Address `json:"token"`
	Index hexutil.Uint64 `json:"index"`
}

// GetOutputs get UTXO outputs
func (s *PublicBlockChainAPI) GetOutputs(ctx context.Context, args []OutputArg) ([]*rtypes.RPCOutput, error) {
	outputs := make([]*rtypes.RPCOutput, 0, len(args))
	for i := 0; i < len(args); i++ {
		output, err := s.b.GetOutput(ctx, args[i].Token, uint64(args[i].Index))
		if err != nil {
			log.Warn("GetOutputs fail", "Token", args[i].Token, "idx", args[i].Index)
			return nil, err
		}
		outputs = append(outputs, &rtypes.RPCOutput{
			Out:     rtypes.RPCKey(output.OTAddr),
			Height:  output.Height,
			Commit:  rtypes.RPCKey(output.Commit),
			TokenID: output.TokenID,
		})
	}

	return outputs, nil
}

// GetWhiteValidators get inner validators in whiteList contract
func (s *PublicBlockChainAPI) GetWhiteValidators(ctx context.Context, blockNr rpc.BlockNumber) ([]*types.Validator, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	vals := state.GetWhiteValidators(log.Root())
	return vals, state.Error()
}

// GetAllCandidates get inner CandidateState from contract
func (s *PublicBlockChainAPI) GetAllCandidates(ctx context.Context, blockNr rpc.BlockNumber) ([]*types.CandidateState, error) {
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	cans := state.GetAllCandidates(log.Root())
	return cans, state.Error()
}

// GetChainVersion return chain version
func (s *PublicBlockChainAPI) GetChainVersion(ctx context.Context) (string, error) {
	return version.Version, nil
}

// GetBlockUTXOsByNumber returns the requested block. When blockNr is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockUTXOsByNumber(ctx context.Context, blockNr rpc.BlockNumber, fullTx bool) (interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	tokenOutputSeqs := s.b.GetBlockTokenOutputSeq(ctx, uint64(blockNr))
	if block != nil {
		response := rtypes.NewRPCBlockUTXO(block, true, fullTx, tokenOutputSeqs)
		if blockNr == rpc.PendingBlockNumber {
			// Pending blocks need to nil out a few fields
			response.Coinbase = nil
			response.Hash = nil
		}
		return response, err
	}
	return nil, err
}

// GetUTXOGas return the gas value of UTXO transaction
func (s *PublicBlockChainAPI) GetUTXOGas(ctx context.Context) (hexutil.Uint64, error) {
	return hexutil.Uint64(s.b.GetUTXOGas()), nil
}
