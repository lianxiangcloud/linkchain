package service

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/bloombits"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/config"
	cs "github.com/lianxiangcloud/linkchain/consensus"
	"github.com/lianxiangcloud/linkchain/libs/common"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/math"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/metrics"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/vm"
	"github.com/lianxiangcloud/linkchain/vm/evm"
	"github.com/lianxiangcloud/linkchain/vm/wasm"
	"github.com/pkg/errors"
)

type ApiBackend struct {
	s             *Service
	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
}

func (b *ApiBackend) context() *Context {
	return b.s.context()
}

func (b *ApiBackend) bloomService() *BloomService {
	return b.s.bloom
}

func NewApiBackend(s *Service) *ApiBackend {
	return &ApiBackend{
		s:             s,
		bloomRequests: make(chan chan *bloombits.Retrieval),
	}
}

// implement backend methods.
func (b *ApiBackend) EVMAllowed() bool {
	return b.s.evmLimit.Allow()
}

// General Ethereum API
func (b *ApiBackend) ProtocolVersion() string {
	return "v1.0.0"
}

func (b *ApiBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return big.NewInt(types.ParGasPrice), nil
}

func (b *ApiBackend) AccountManager() *accounts.Manager {
	return b.context().accManager
}

func (b *ApiBackend) Coinbase() common.Address {
	return b.context().GetCoinBase()
}

// Consensus API
func (b *ApiBackend) StopTheWorld() bool {
	return b.context().consensusReactor.StopTheWorld()
}

func (b *ApiBackend) StartTheWorld() bool {
	return b.context().consensusReactor.StartTheWorld()
}

func (b *ApiBackend) ConsensusState() (*rtypes.ResultConsensusState, error) {
	// Get self round state.
	bz, err := b.context().consensusState.GetRoundStateSimpleJSON()
	return &rtypes.ResultConsensusState{bz}, err
}

func (b *ApiBackend) DumpConsensusState() (*rtypes.ResultDumpConsensusState, error) {
	// Get Peer consensus states.
	peers := b.context().p2pSwitch.Peers().List()
	peerStates := make([]rtypes.PeerStateInfo, len(peers))
	for i, peer := range peers {
		peerState := peer.Get(types.PeerStateKey).(*cs.PeerState)
		peerStateJSON, err := peerState.ToJSON()
		if err != nil {
			return nil, err
		}
		peerStates[i] = rtypes.PeerStateInfo{
			// Peer basic info.
			NodeAddress: peer.ID(),
			// Peer consensus state.
			PeerState: peerStateJSON,
		}
	}
	// Get self round state.
	roundState, err := b.context().consensusState.GetRoundStateJSON()
	if err != nil {
		return nil, err
	}
	return &rtypes.ResultDumpConsensusState{roundState, peerStates}, nil
}

func (b *ApiBackend) Validators(heightPtr *uint64) (*rtypes.ResultValidators, error) {
	storeHeight := b.context().blockStore.Height()
	height, err := getHeight(storeHeight, heightPtr)
	if err != nil {
		return nil, err
	}

	validators, lastHeightChanged, err := cs.LoadValidators(b.context().stateDB, height)
	if err != nil {
		return nil, err
	}
	return &rtypes.ResultValidators{height, lastHeightChanged, validators.Validators}, nil
}

func (b *ApiBackend) Block(heightPtr *uint64) (*rtypes.ResultBlock, error) {
	storeHeight := b.context().blockStore.Height()
	height, err := getHeight(storeHeight, heightPtr)
	if err != nil {
		return nil, err
	}

	blockMeta := b.context().blockStore.LoadBlockMeta(height)
	block := b.context().blockStore.LoadBlock(height)
	return &rtypes.ResultBlock{blockMeta, block}, nil
}

func getHeight(storeHeight uint64, heightPtr *uint64) (uint64, error) {
	if heightPtr != nil {
		height := *heightPtr
		if height <= 0 {
			return 0, fmt.Errorf("Height must be greater than 0")
		}
		if height > storeHeight {
			return 0, fmt.Errorf("Height must be less than or equal to the current blockchain height")
		}
		return height, nil
	}
	return storeHeight, nil
}

func (b *ApiBackend) NetInfo() (*rtypes.ResultNetInfo, error) {
	peers := []rtypes.Peer{}
	for _, peer := range b.context().p2pSwitch.Peers().List() {
		peers = append(peers, rtypes.Peer{
			NodeInfo:         peer.NodeInfo(),
			IsOutbound:       peer.IsOutbound(),
			ConnectionStatus: peer.Status(),
		})
	}
	// TODO: Should we include PersistentPeers and Seeds in here?
	// PRO: useful info
	// CON: privacy
	return &rtypes.ResultNetInfo{
		NPeers: len(peers),
		Peers:  peers,
	}, nil
}

func (b *ApiBackend) Status() (*rtypes.ResultStatus, error) {
	latestHeight := b.context().blockStore.Height()
	var (
		latestBlockMeta     *types.BlockMeta
		latestBlockHash     cmn.HexBytes
		latestAppHash       cmn.HexBytes
		latestBlockTimeNano int64
	)
	if latestHeight != 0 {
		latestBlockMeta = b.context().blockStore.LoadBlockMeta(latestHeight)
		latestBlockHash = latestBlockMeta.BlockID.Hash.Bytes()
		latestAppHash = latestBlockMeta.Header.ParentHash.Bytes()
		latestBlockTimeNano = int64(latestBlockMeta.Header.Time)
	}

	latestBlockTime := time.Unix(0, latestBlockTimeNano)

	var votingPower int64
	if val := b.validatorAtHeight(latestHeight); val != nil {
		votingPower = val.VotingPower
	}

	result := &rtypes.ResultStatus{
		NodeInfo: b.context().p2pSwitch.LocalNodeInfo(),
		SyncInfo: rtypes.SyncInfo{
			LatestBlockHash:   latestBlockHash,
			LatestAppHash:     latestAppHash,
			LatestBlockHeight: latestHeight,
			LatestBlockTime:   latestBlockTime,
			CatchingUp:        b.context().consensusReactor.FastSync(),
		},
		ValidatorInfo: rtypes.ValidatorInfo{
			Address:     b.context().pubKey.Address(),
			PubKey:      b.context().pubKey,
			VotingPower: votingPower,
		},
	}

	return result, nil
}

func (b *ApiBackend) validatorAtHeight(h uint64) *types.Validator {
	privValAddress := b.context().pubKey.Address()

	lastBlockHeight, vals := b.context().consensusState.GetValidators()

	// if we're still at height h, search in the current validator set
	if lastBlockHeight == h {
		for _, val := range vals {
			if bytes.Equal(val.Address, privValAddress) {
				return val
			}
		}
	}

	// if we've moved to the next height, retrieve the validator set from DB
	if lastBlockHeight > h {
		vals, _, err := cs.LoadValidators(b.context().stateDB, h)
		if err != nil {
			// should not happen
			return nil
		}
		_, val := vals.GetByAddress(privValAddress)
		return val
	}

	return nil
}

// BlockChain API
func (b *ApiBackend) GetTx(hash common.Hash) (types.Tx, *types.TxEntry) {
	bc := b.context().blockStore
	return bc.GetTx(hash)
}

func (b *ApiBackend) GetTransactionReceipt(hash common.Hash) (*types.Receipt, common.Hash, uint64, uint64) {
	bc := b.context().blockStore
	return bc.GetTransactionReceipt(hash)
}

func (b *ApiBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	if blockNr == rpc.PendingBlockNumber {
		block := b.context().app.GetPendingBlock()
		return block.Head(), nil
	}

	var height, latest uint64
	bc := b.context().blockStore
	latest = bc.Height()
	if blockNr == rpc.LatestBlockNumber {
		height = latest
	} else {
		height = uint64(blockNr)
	}
	height++
	if height > (latest + 1) {
		b.s.logger.Debug("ApiBackend StateAndHeaderByNumber: invalid block_number", "req_num", height, "latest_num", latest)
		return nil, fmt.Errorf("invalid block_number")
	}

	if height == (latest + 1) {
		return b.context().app.GetPendingBlock().Head(), nil
	}
	meta := bc.LoadBlockMeta(height)
	if meta == nil {
		b.s.logger.Warn("ApiBackend HeaderByNumber: blockstore LoadBlockMeta fail", "height", height)
		return nil, fmt.Errorf("LoadblockMeta fail")
	}

	return meta.Header, nil
}

func (b *ApiBackend) HeaderByHeight(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	if blockNr == rpc.PendingBlockNumber {
		block := b.context().app.GetPendingBlock()
		return block.Head(), nil
	}

	var height, latest uint64
	bc := b.context().blockStore
	latest = bc.Height()
	if blockNr == rpc.LatestBlockNumber {
		height = latest
	} else {
		height = uint64(blockNr)
	}
	if height > (latest + 1) {
		b.s.logger.Debug("ApiBackend StateAndHeaderByNumber: invalid block_number", "req_num", height, "latest_num", latest)
		return nil, fmt.Errorf("invalid block_number")
	}

	if height == (latest + 1) {
		return b.context().app.GetPendingBlock().Head(), nil
	}
	meta := bc.LoadBlockMeta(height)
	if meta == nil {
		b.s.logger.Warn("ApiBackend HeaderByNumber: blockstore LoadBlockMeta fail", "height", height)
		return nil, fmt.Errorf("LoadblockMeta fail")
	}

	return meta.Header, nil
}

func (b *ApiBackend) Height() uint64 {
	return b.context().blockStore.Height()
}

func (b *ApiBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	if blockNr == rpc.PendingBlockNumber {
		return b.context().app.GetPendingBlock(), nil
	}

	var height uint64
	bc := b.context().blockStore
	if blockNr == rpc.LatestBlockNumber {
		height = bc.Height()
	} else {
		height = uint64(blockNr)
	}

	latest := bc.Height()
	if height > latest {
		b.s.logger.Debug("ApiBackend BlockByNumber: invalid block_number", "req_num", height, "latest_num", latest)
		return nil, fmt.Errorf("invalid block_number")
	}

	block := bc.LoadBlock(height)
	if block == nil {
		b.s.logger.Warn("ApiBackend BlockByNumber: blockstore LoadBlock fail", "height", height)
		return nil, fmt.Errorf("LoadBlock fail")
	}

	return block, nil
}

func (b *ApiBackend) BalanceRecordByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.BlockBalanceRecords, error) {
	height := uint64(blockNr)
	brs := b.context().brs
	bbr := brs.Get(height)
	if bbr == nil {
		return nil, errors.New("balance record not found")
	}
	return bbr, nil
}

func (b *ApiBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	if blockNr == rpc.PendingBlockNumber {
		// @Todo: lack of header
		return b.context().app.GetPendingStateDB(), b.context().app.GetPendingBlock().Head(), nil
	}

	var height uint64
	bc := b.context().blockStore
	latest := bc.Height()

	if blockNr == rpc.LatestBlockNumber {
		height = latest
	} else {
		height = uint64(blockNr)
	}
	// @Note: state-root always saved at next Block
	height++

	if height > (latest + 1) {
		b.s.logger.Debug("ApiBackend StateAndHeaderByNumber: invalid block_number", "req_num", height, "latest_num", latest)
		return nil, nil, fmt.Errorf("invalid block_number")
	}

	if height == (latest + 1) {
		return b.context().app.GetLatestStateDB(), b.context().app.GetPendingBlock().Head(), nil
	}

	meta := bc.LoadBlockMeta(height)
	if meta == nil {
		b.s.logger.Warn("ApiBackend StateAndHeaderByNumber: blockstore LoadBlockMeta fail", "height", height)
		return nil, nil, fmt.Errorf("LoadBlockMeta fail")
	}

	txsResult, err := bc.LoadTxsResult(height - 1)
	if err != nil {
		b.s.logger.Warn("ApiBackend StateAndHeaderByNumber: blockstore LoadTxsResult fail", "height", height-1)
		return nil, nil, fmt.Errorf("LoadTxsResult fail")
	}

	if txsResult.TrieRoot == common.EmptyHash {
		return nil, nil, fmt.Errorf("there is no historical state in light weight node")
	}

	statedb, err := state.New(txsResult.TrieRoot, b.context().triedb)
	if err != nil {
		b.s.logger.Warn("ApiBackend StateAndHeaderByNumber: state.New fail", "state_hash", meta.Header.StateHash.String(), "height", meta.Header.Height, "block_hash", meta.Header.Hash().String())
		return nil, nil, err
	}

	return statedb, meta.Header, nil
}

func (b *ApiBackend) GetVM(ctx context.Context, msg types.Message, state *state.StateDB, header *types.Header,
	vmCfg evm.Config) (vm.VmInterface, func() error, error) {
	state.SetBalance(msg.MsgFrom(), math.MaxBig256)
	vmError := func() error { return nil }

	vmenv := vm.NewVM()
	evmGasRate := config.EvmGasRate
	contextEvm := evm.NewEVMContext(header, b.context().blockStore, nil, evmGasRate)
	vmenv.AddVm(&contextEvm, state, vmCfg)
	wasmGasRate := config.WasmGasRate
	contextWasm := wasm.NewWASMContext(header, b.context().blockStore, nil, wasmGasRate)
	vmenv.AddVm(&contextWasm, state, vmCfg)

	// realvm := vmenv.GetRealVm(state, *msg.To())
	msgcode := msg.Data()
	if msg.To() != nil && state.IsContract(*msg.To()) {
		msgcode = state.GetCode(*(msg.To()))
	}
	realvm := vmenv.GetRealVm(msgcode, msg.To())
	realvm.Reset(msg)

	return realvm, vmError, nil
}

func (b *ApiBackend) GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error) {
	bc := b.context().blockStore
	block := bc.LoadBlockByHash(blockHash)
	if block == nil {
		b.s.logger.Info("ApiBackend GetBlock: blockstore LoadBlockByHash fail", "hash", blockHash.String())
		return nil, nil
	}
	return block, nil
}

func (b *ApiBackend) GetReceipts(ctx context.Context, blockNr uint64) types.Receipts {
	bc := b.context().blockStore

	receipts := bc.GetReceipts(blockNr)
	if receipts == nil {
		b.s.logger.Warn("ApiBackend GetReceipts: blockstore GetReceipts nil", "height", blockNr)
		return types.Receipts{}
	}
	return *receipts
}

func (b *ApiBackend) GetBlockBalanceRecords(height uint64) *types.BlockBalanceRecords {
	brs := b.context().brs
	return brs.Get(height)
}

// GetMaxOutputIndex get max UTXO output index by token
func (b *ApiBackend) GetMaxOutputIndex(ctx context.Context, token common.Address) int64 {
	return b.context().utxo.GetMaxUtxoOutputSeq(token)
}

func (b *ApiBackend) GetBlockTokenOutputSeq(ctx context.Context, blockHeight uint64) map[string]int64 {
	return b.context().utxo.GetBlockTokenUtxoOutputSeq(blockHeight)
}

// GetOutput get UTXO output
func (b *ApiBackend) GetOutput(ctx context.Context, token common.Address, index uint64) (*types.UTXOOutputData, error) {
	return b.context().utxo.GetUtxoOutput(token, index)
}

// TxPool API
func (b *ApiBackend) SendTx(ctx context.Context, signedTx types.Tx) error {
	return b.context().mempool.AddTx("", signedTx)
}

func (b *ApiBackend) GetPoolTx(txHash common.Hash) types.Tx {
	b.s.logger.Warn("ApiBackend GetPoolTx: Not Support")
	return nil
}

func (b *ApiBackend) GetPoolTransaction(txHash common.Hash) *types.Transaction {
	b.s.logger.Warn("ApiBackend GetPoolTransaction: Not Support")
	return nil
}

func (b *ApiBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.context().app.GetNonce(addr), nil
}

// bloom filter
func (b *ApiBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.bloomService().chainIndexer.Sections()
	return types.BloomBitsBlocks, sections
}

func (b *ApiBackend) EventBus() *types.EventBus {
	return b.context().eventBus
}

func (b *ApiBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.bloomRequests)
	}
}

func (b *ApiBackend) Stats() (int, int, int) {
	return b.context().mempool.Stats()
}

func (b *ApiBackend) PrometheusMetrics() string {
	return metrics.PrometheusMetricInstance().GetMetrics()
}

func (b *ApiBackend) GetUTXOGas() uint64 {
	return b.context().app.GetUTXOGas()
}
