package node

import (
	"bytes"
	"net/http"
	_ "net/http/pprof"
	"time"

	"fmt"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	"github.com/lianxiangcloud/linkchain/app"
	bc "github.com/lianxiangcloud/linkchain/blockchain"
	"github.com/lianxiangcloud/linkchain/bootnode"
	cfg "github.com/lianxiangcloud/linkchain/config"
	cs "github.com/lianxiangcloud/linkchain/consensus"
	"github.com/lianxiangcloud/linkchain/evidence"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p"
	p2pcmn "github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/libs/p2p/sync"
	"github.com/lianxiangcloud/linkchain/libs/txmgr"
	mempl "github.com/lianxiangcloud/linkchain/mempool"
	"github.com/lianxiangcloud/linkchain/metrics"
	"github.com/lianxiangcloud/linkchain/rpc/service"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/utxo"
	"github.com/lianxiangcloud/linkchain/version"
)

//------------------------------------------------------------------------------

// DBContext specifies config information for loading a new DB.
type DBContext struct {
	ID     string
	Config *cfg.Config
}

// DBProvider takes a DBContext and returns an instantiated DB.
type DBProvider func(*DBContext) (dbm.DB, error)

// DefaultDBProvider returns a database using the DBBackend and DBDir
// specified in the ctx.Config.
func DefaultDBProvider(ctx *DBContext) (dbm.DB, error) {
	dbType := dbm.DBBackendType(ctx.Config.DBBackend)
	return dbm.NewDB(ctx.ID, dbType, ctx.Config.DBDir(), ctx.Config.DBCounts), nil
}

// MetricsProvider returns a consensus, p2p and mempool Metrics.
type MetricsProvider func() (*cs.Metrics, *mempl.Metrics)

// NodeProvider takes a config and a logger and returns a ready to go Node.
type NodeProvider func(*cfg.Config, log.Logger) (*Node, error)

// DefaultMetricsProvider returns consensus, p2p and mempool Metrics build
// using Prometheus client library.
func DefaultMetricsProvider() (*cs.Metrics, *mempl.Metrics) {
	return cs.PrometheusMetrics(), mempl.PrometheusMetrics()
}

// DefaultNewNode returns a node with default settings for the
// PrivValidator, and DBProvider.
// It implements NodeProvider.
func DefaultNewNode(config *cfg.Config, logger log.Logger) (*Node, error) {
	return NewNode(config,
		types.LoadOrGenFilePV(config.PrivValidatorFile()),
		DefaultDBProvider,
		DefaultMetricsProvider,
		logger,
	)
}

// NopMetricsProvider returns consensus, p2p and mempool Metrics as no-op.
func NopMetricsProvider() (*cs.Metrics, *mempl.Metrics) {
	return cs.NopMetrics(), mempl.NopMetrics()
}

//------------------------------------------------------------------------------

// Node is the highest level interface to a full node.
// It includes all configuration information and running services.
type Node struct {
	cmn.BaseService

	// config
	config        *cfg.Config
	privValidator types.PrivValidator // local node's validator key

	// network
	p2pmanager *p2p.Switch // p2p connections

	// accounts manager
	accountManager *accounts.Manager

	// services
	eventBus         *types.EventBus // pub/sub for services
	stateDB          dbm.DB
	blockStore       *bc.BlockStore         // store the blockchain to disk
	bcReactor        *bc.BlockchainReactor  // for fast-syncing
	mempoolReactor   *mempl.MempoolReactor  // for gossipping transactions
	consensusState   *cs.ConsensusState     // latest consensus state
	consensusReactor *cs.ConsensusReactor   // for participating in the consensus
	evidencePool     *evidence.EvidencePool // tracking evidence
	syncManager      *sync.SyncHeightManager
	// rpc
	//rpcContext *service.Context
	rpcService *service.Service
}

func makeAccountManager(config *cfg.Config) (*accounts.Manager, error) {
	scryptN := keystore.LightScryptN
	scryptP := keystore.LightScryptP
	keystoreDir := config.KeyStoreDir()

	// Assemble the account manager and supported backends
	backends := []accounts.Backend{
		keystore.NewKeyStore(keystoreDir, scryptN, scryptP),
	}

	return accounts.NewManager(backends...), nil
}

// NewNode returns a new, ready to go.
func NewNode(config *cfg.Config,
	privValidator types.PrivValidator,
	dbProvider DBProvider,
	metricsProvider MetricsProvider,
	logger log.Logger) (*Node, error) {
	var seeds []*p2pcmn.Node
	var localNodeType types.NodeType
	var err error
	if len(config.BootNodeSvr.Addrs) != 0 {
		bootnode.UpdateBootNode(config.BootNodeSvr.Addrs, logger)
	}
	var bootNodeAddr = bootnode.GetBestBootNode()
	if len(bootNodeAddr) != 0 && config.IsTestMode == false {
		seeds, localNodeType, err = bootnode.GetSeeds(bootNodeAddr, privValidator.GetPrikey(), logger)
	}
	if err != nil {
		logger.Error("GetSeeds failed")
		return nil, err
	}
	localPubKeyHex := hexutil.Encode(privValidator.GetPubKey().Bytes())
	for i := 0; i < len(seeds); i++ {
		logger.Info("GetSeedsFromBootSvr", " seeds i", i, "ip", seeds[i].IP.String(), "UDP_Port", seeds[i].UDP_Port, "TCP_Port", seeds[i].TCP_Port)
	}

	logger.Info("GetSeeds:", "BootNodeSvr addr", bootNodeAddr, "localpubky hex", localPubKeyHex, "localNodeType", localNodeType)

	// Get BlockStore
	blockStoreDB, err := dbProvider(&DBContext{"blockstore", config})
	if err != nil {
		return nil, err
	}
	blockStore := bc.NewBlockStore(blockStoreDB)
	initHeight, err := blockStore.LoadInitHeight()
	if err != nil {
		return nil, err
	}
	types.UpdateBlockHeightZero(initHeight)

	// Get Balance Records Store
	balanceRecordStoreDB, err := dbProvider(&DBContext{"balance_record", config})
	if err != nil {
		return nil, err
	}
	balanceRecord := bc.NewBalanceRecordStore(balanceRecordStoreDB, config.SaveBalanceRecord)
	types.SaveBalanceRecord = config.SaveBalanceRecord

	// Get TxService
	txDB, err := dbProvider(&DBContext{"txmgr", config})
	if err != nil {
		return nil, err
	}
	txService := txmgr.NewCrossState(txDB, blockStore)
	txService.SetLogger(logger.With("module", "txmgr"))
	blockStore.SetCrossState(txService)

	// Init Prometheus Metrics
	metrics.PrometheusMetricInstance.Init(config, privValidator.GetPubKey(), logger.With("module", "prometheus_metrics"))
	metrics.PrometheusMetricInstance.SetRole(localNodeType)

	// Get Consensus Status
	statusDB, err := dbProvider(&DBContext{"consensus_state", config})
	if err != nil {
		return nil, err
	}

	status, err := cs.LoadStatus(statusDB)
	if err != nil {
		return nil, err
	}

	for i, v := range status.Validators.Validators {
		logger.Info("current validators", "height", status.LastBlockHeight, "idx", i, "pubKey", fmt.Sprintf("0x%x", v.PubKey.Bytes()), "addr", v.Address)
	}

	// Create Account state DB
	newDB, err := dbProvider(&DBContext{"state", config})

	// need rollback blockStore height
	if config.BaseConfig.RollBack && state.CanRollBackOneBlock(newDB, blockStore.Height()) {
		blockStore.RollBackOneBlock()
		status, err = cs.LoadStatusByHeight(statusDB, blockStore.Height())
		if err != nil {
			return nil, err
		}
	}

	// Create Evidence DB
	evidenceDB, err := dbProvider(&DBContext{"evidence", config})
	if err != nil {
		return nil, err
	}

	// Ensure that the AccountManager method works before the node has started.
	accountManager, err := makeAccountManager(config)
	if err != nil {
		return nil, err
	}

	// Make EventBus
	eventBus := types.NewEventBus()
	eventBus.SetLogger(logger.With("module", "events"))

	// Make Evidence Reactor
	evidenceLogger := logger.With("module", "evidence")
	evidenceStore := evidence.NewEvidenceStore(evidenceDB)
	evidencePool := evidence.NewEvidencePool(statusDB, evidenceStore, status.Copy())
	evidencePool.SetLogger(evidenceLogger)
	evidenceReactor := evidence.NewEvidenceReactor(evidencePool)
	evidenceReactor.SetLogger(evidenceLogger)

	// blacklist use evidence db
	types.BlacklistInstance.Init(evidenceDB)

	// Create Utxo DB
	utxoDB, err := dbProvider(&DBContext{"utxo", config})
	if err != nil {
		return nil, err
	}
	utxoOutputConfig := &cfg.Config{}
	*utxoOutputConfig = *config
	utxoOutputConfig.DBBackend = "bolt"
	utxoOutputDB, err := dbProvider(&DBContext{"utxo_output", utxoOutputConfig})
	if err != nil {
		return nil, err
	}
	utxoOutputTokenDB, err := dbProvider(&DBContext{"utxo_output_token", config})
	if err != nil {
		return nil, err
	}
	utxoStore := utxo.NewUtxoStore(utxoDB, utxoOutputDB, utxoOutputTokenDB)
	utxoStore.SetLogger(logger.With("module", "utxoStore"))

	//create app
	isTrie := config.FullNode
	appHandle, err := app.NewLinkApplication(newDB, blockStore, utxoStore, txService, eventBus, isTrie, balanceRecord, app.SetPoceeds, app.AllocAward)
	if err != nil {
		return nil, err
	}
	appHandle.SetLogger(logger.With("module", "app"))

	// make block executor for update consensus status
	blockExec := cs.NewBlockExecutor(statusDB, logger, evidencePool)

	// Check consensus status with application storage, rebuild status if not consist
	appHeight := appHandle.Height()
	if status.LastBlockHeight+1 == appHeight {
		logger.Warn("rebuild status", "statusHeight", status.LastBlockHeight, "appHeight", appHeight)
		blockMeta := appHandle.LoadBlockMeta(appHeight)
		block := appHandle.LoadBlock(appHeight)
		if blockMeta == nil || block == nil {
			logger.Error("rebuild status", "block", (block != nil), "blockMeta", (blockMeta != nil))
			return nil, types.ErrUnknownBlock
		}
		validators := appHandle.GetValidators(appHeight)
		newStatus, err := blockExec.ApplyBlock(status, blockMeta.BlockID, block, validators)
		if err != nil {
			return nil, err
		}
		status = newStatus.Copy()
	}

	// metrics
	var (
		csMetrics    *cs.Metrics
		memplMetrics *mempl.Metrics
	)
	if config.Instrumentation.Prometheus {
		csMetrics, memplMetrics = metricsProvider()
	} else {
		csMetrics, memplMetrics = NopMetricsProvider()
	}
	//types
	typesLogger := logger.With("module", "types")
	types.SetLogger(typesLogger)
	// Make P2P Manager
	p2pDB, err := dbProvider(&DBContext{"p2p", config})
	if err != nil {
		return nil, err
	}
	p2pLogger := logger.With("module", "p2p")
	localNodeInfo := MakeNodeInfo(status.ChainID, localNodeType, config.Moniker, config.RPC.HTTPEndpoint)
	p2pmanager, err := p2p.NewP2pManager(p2pLogger, privValidator.GetPrikey(), config.P2P,
		localNodeInfo, seeds, p2pDB)
	if err != nil {
		logger.Warn("NewP2pManager failed")
		return nil, err
	}
	//
	syncLogger := logger.With("module", "syncheight")
	var checkInterval time.Duration = time.Duration(config.Consensus.CreateEmptyBlocksInterval) * time.Second
	syncManager := sync.NewSyncHeightManager(p2pmanager, appHandle, checkInterval, syncLogger)
	//
	evidenceReactor.SetP2PManager(p2pmanager)
	// Make MempoolReactor
	mempoolLogger := logger.With("module", "mempool")
	mempool := mempl.NewMempool(
		config.Mempool,
		status.LastBlockHeight,
		p2pmanager,
		mempl.WithMetrics(memplMetrics),
	)
	mempool.SetLogger(mempoolLogger)
	mempool.SetApp(appHandle)
	appHandle.SetMempool(mempool)
	appHandle.SetConm(p2pmanager.GetConManager())
	//mempool.InitWAL() // no need to have the mempool wal during tests
	mempoolReactor := mempl.NewMempoolReactor(config.Mempool, mempool)
	mempoolReactor.SetLogger(mempoolLogger)

	if config.Consensus.WaitForTxs() {
		mempool.EnableTxsAvailable()
	}

	// Decide whether to fast-sync or not
	// We don't fast-sync when the only validator is us.
	fastSync := config.FastSync
	if status.Validators.Size() == 1 {
		addr, _ := status.Validators.GetByIndex(0)
		if bytes.Equal(privValidator.GetAddress(), addr) {
			fastSync = false
			mempool.SetReceiveP2pTx(true)
		}
	}

	// Make BlockchainReactor
	bcReactor := bc.NewBlockchainReactor(status.Copy(), blockExec, appHandle, fastSync, p2pmanager)
	bcReactor.SetLogger(logger.With("module", "blockchain"))
	bcReactor.KeepFastSync(isTrie)

	consensusLogger := logger.With("module", "consensus")
	if status.Validators.HasAddress(privValidator.GetAddress()) {
		consensusLogger.Info("This node is a validator", "addr", privValidator.GetAddress(), "pubKey", privValidator.GetPubKey())
	} else {
		consensusLogger.Info("This node is not a validator", "addr", privValidator.GetAddress(), "pubKey", privValidator.GetPubKey())
	}

	// Make ConsensusReactor
	consensusState := cs.NewConsensusState(
		config.Consensus,
		status.Copy(),
		blockExec,
		appHandle,
		mempool,
		evidencePool,
		cs.WithMetrics(csMetrics),
	)
	consensusState.SetEventBus(eventBus)
	consensusState.SetLogger(consensusLogger)
	if privValidator != nil {
		consensusState.SetPrivValidator(privValidator)
	}
	consensusReactor := cs.NewConsensusReactor(consensusState, fastSync, p2pmanager)
	consensusReactor.SetLogger(consensusLogger)

	consensusReactor.SetReceiveP2pTx(!isTrie)
	p2pmanager.AddReactor("MEMPOOL", mempoolReactor)
	p2pmanager.AddReactor("BLOCKCHAIN", bcReactor)
	p2pmanager.AddReactor("CONSENSUS", consensusReactor)
	p2pmanager.AddReactor("EVIDENCE", evidenceReactor)

	// Filter peers by addr or pubkey with an ABCI query.
	// If the query return code is OK, add peer.
	// XXX: Query format subject to change
	if config.FilterPeers {

	}

	// Make rpc context and service
	rpcContext := service.NewContext()
	rpcContext.SetLogger(logger)
	rpcContext.SetAccountManager(accountManager)
	rpcContext.SetBlockstore(blockStore)
	rpcContext.SetBalanceRecordStore(balanceRecord)
	rpcContext.SetTrieDB(newDB, isTrie)
	rpcContext.SetStateDB(statusDB)
	rpcContext.SetPubKey(privValidator.GetPubKey())
	rpcContext.SetSwitch(p2pmanager)
	rpcContext.SetConsensus(consensusState)
	rpcContext.SetConsensusReactor(consensusReactor)
	rpcContext.SetMempool(mempool)
	rpcContext.SetApp(appHandle)
	rpcContext.SetUTXO(utxoStore)
	rpcContext.SetEventBus(eventBus)
	rpcContext.SetTxService(txService)
	//rpcContext.SetCoinbase(common.HexToAddress(coinbase))
	rpcService := service.New(config.RPC, rpcContext)

	// run the profile server
	profileHost := config.ProfListenAddress
	if profileHost != "" {
		go func() {
			logger.Error("Profile server", "err", http.ListenAndServe(profileHost, nil))
		}()
	}

	node := &Node{
		config:        config,
		privValidator: privValidator,

		p2pmanager: p2pmanager,

		accountManager: accountManager,

		stateDB:          statusDB,
		blockStore:       blockStore,
		bcReactor:        bcReactor,
		mempoolReactor:   mempoolReactor,
		consensusState:   consensusState,
		consensusReactor: consensusReactor,
		evidencePool:     evidencePool,
		eventBus:         eventBus,
		rpcService:       rpcService,
		syncManager:      syncManager,
	}

	node.BaseService = *cmn.NewBaseService(logger, "Node", node)
	return node, nil
}

// OnStart starts the Node. It implements cmn.Service.
func (n *Node) OnStart() error {
	err := n.eventBus.Start()
	if err != nil {
		return err
	}

	if err = n.rpcService.Start(); err != nil {
		n.Logger.Warn("rpc service start fail", "err", err)
		return err
	}

	// Start the switch (the P2P server).
	err = n.p2pmanager.Start()
	if err != nil {
		return err
	}
	n.syncManager.Start()
	if n.config.KeepLatestBlocks > 0 {
		go n.ClearHistoricalData()
	}

	return nil
}

// OnStop stops the Node. It implements cmn.Service.
func (n *Node) OnStop() {
	n.BaseService.OnStop()

	n.Logger.Info("Stopping Node")

	// first stop the non-reactor services
	n.eventBus.Stop()

	// second stop the reactors
	// TODO: gracefully disconnect from peers.
	n.p2pmanager.Stop()
	n.syncManager.Stop()

	n.rpcService.Stop()
}

// RunForever waits for an interrupt signal and stops the node.
func (n *Node) RunForever() {
	// Sleep forever and then...
	cmn.TrapSignal(func() {
		n.Stop()
	})
}

// Switch returns the Node's Switch.
func (n *Node) P2PManager() *p2p.Switch {
	return n.p2pmanager
}

// BlockStore returns the Node's BlockStore.
func (n *Node) BlockStore() *bc.BlockStore {
	return n.blockStore
}

// ConsensusState returns the Node's ConsensusState.
func (n *Node) ConsensusState() *cs.ConsensusState {
	return n.consensusState
}

// ConsensusReactor returns the Node's ConsensusReactor.
func (n *Node) ConsensusReactor() *cs.ConsensusReactor {
	return n.consensusReactor
}

// MempoolReactor returns the Node's MempoolReactor.
func (n *Node) MempoolReactor() *mempl.MempoolReactor {
	return n.mempoolReactor
}

func (n *Node) BlockchainReactor() *bc.BlockchainReactor {
	return n.bcReactor
}

// EvidencePool returns the Node's EvidencePool.
func (n *Node) EvidencePool() *evidence.EvidencePool {
	return n.evidencePool
}

// EventBus returns the Node's EventBus.
func (n *Node) EventBus() *types.EventBus {
	return n.eventBus
}

// PrivValidator returns the Node's PrivValidator.
// XXX: for convenience only!
func (n *Node) PrivValidator() types.PrivValidator {
	return n.privValidator
}

func MakeNodeInfo(chainID string, nodeType types.NodeType, moniker string, httpEndpoint string) p2p.NodeInfo {
	nodeInfo := p2p.NodeInfo{
		Network: chainID,
		Version: version.Version,
		Channels: []byte{
			bc.BlockchainChannel,
			cs.StateChannel, cs.DataChannel, cs.VoteChannel, cs.VoteSetBitsChannel,
			mempl.MempoolChannel,
			evidence.EvidenceChannel,
		},
		Moniker: moniker,
		Other: []string{
			cmn.Fmt("p2p_version=%v", p2p.Version),
			cmn.Fmt("consensus_version=%v", cs.Version),
		},
		Type: nodeType,
	}

	nodeInfo.Other = append(nodeInfo.Other, cmn.Fmt("rpc_addr=%v", httpEndpoint))
	return nodeInfo
}

//------------------------------------------------------------------------------

// NodeInfo returns the Node's Info from the Switch.
func (n *Node) NodeInfo() p2p.NodeInfo {
	return n.p2pmanager.LocalNodeInfo()
}

func (n *Node) ClearHistoricalData() {
	interval := n.config.ClearDataInterval
	if interval < 59 {
		interval = 59
	}
	n.Logger.Info("ClearHistoricalData: start", "interval", interval, "ClearDataInterval", n.config.ClearDataInterval)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)

FOR_LOOP:
	for {
		select {
		case <-ticker.C:
			n.blockStore.DeleteHistoricalData(n.config.KeepLatestBlocks)
			n.consensusState.DeleteHistoricalData(n.config.KeepLatestBlocks)
		case <-n.Quit():
			break FOR_LOOP
		}
	}

	n.Logger.Info("ClearHistoricalData: done")
}
