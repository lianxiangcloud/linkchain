package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/log"
)

const (
	// FuzzModeDrop is a mode in which we randomly drop reads/writes, connections or sleep
	FuzzModeDrop = iota
	// FuzzModeDelay is a mode in which we randomly sleep
	FuzzModeDelay
)

// NOTE: Most of the structs & relevant comments + the
// default configuration options were used to manually
// generate the config.toml. Please reflect any changes
// made here in the defaultConfigTemplate constant in
// config/toml.go
// NOTE: tmlibs/cli must know to look in the config dir!
var (
	DefaultChainDir    = "linkchain"
	defaultConfigDir   = "config"
	defaultDataDir     = "data"
	defaultLogDir      = "log"
	defaultKeyStoreDir = "keystore"

	defaultLogFileName     = "linkchain.log"
	defaultConfigFileName  = "config.toml"
	defaultGenesisJSONName = "genesis.json"

	defaultPrivValName  = "priv_validator.json"
	defaultAddrBookName = "addrbook.json"

	defaultConfigFilePath  = filepath.Join(defaultConfigDir, defaultConfigFileName)
	defaultGenesisJSONPath = filepath.Join(defaultConfigDir, defaultGenesisJSONName)
	defaultPrivValPath     = filepath.Join(defaultConfigDir, defaultPrivValName)
	defaultAddrBookPath    = filepath.Join(defaultConfigDir, defaultAddrBookName)

	WasmGasRate = uint64(1)
	EvmGasRate  = uint64(1)
)

// Config defines the top level configuration for a node
type Config struct {
	// Top level options use an anonymous struct
	BaseConfig `mapstructure:",squash"`

	// Options for services
	Log             *log.RotateConfig      `mapstructure:"log"`
	RPC             *RPCConfig             `mapstructure:"rpc"`
	P2P             *P2PConfig             `mapstructure:"p2p"`
	Mempool         *MempoolConfig         `mapstructure:"mempool"`
	Consensus       *ConsensusConfig       `mapstructure:"consensus"`
	Instrumentation *InstrumentationConfig `mapstructure:"instrumentation"`
	BootNodeSvr     *BootNodeConfig        `mapstructure:"bootnode"`
}

// DefaultConfig returns a default configuration for a node
func DefaultConfig() *Config {
	return &Config{
		BaseConfig:      DefaultBaseConfig(),
		Log:             DefaultRotateConfig(),
		RPC:             DefaultRPCConfig(),
		P2P:             DefaultP2PConfig(),
		Mempool:         DefaultMempoolConfig(),
		Consensus:       DefaultConsensusConfig(),
		Instrumentation: DefaultInstrumentationConfig(),
		BootNodeSvr:     DefaultBootNodeConfig(),
	}
}

// TestConfig returns a configuration that can be used for testing
func TestConfig() *Config {
	return &Config{
		BaseConfig:      TestBaseConfig(),
		Log:             TestRotateConfig(),
		RPC:             TestRPCConfig(),
		P2P:             TestP2PConfig(),
		Mempool:         TestMempoolConfig(),
		Consensus:       TestConsensusConfig(),
		Instrumentation: TestInstrumentationConfig(),
	}
}

// SetRoot sets the RootDir for all Config structs
func (cfg *Config) SetRoot(root string) *Config {
	cfg.BaseConfig.RootDir = root
	cfg.Log.RootDir = root
	cfg.RPC.RootDir = root
	cfg.Mempool.RootDir = root
	cfg.Consensus.RootDir = root
	return cfg
}

//-----------------------------------------------------------------------------
// BaseConfig

// BaseConfig defines the base configuration for a node
type BaseConfig struct {

	// The ID of the chain to join (should be signed with every transaction and vote)
	ChainID string `mapstructure:"chain_id"`

	// The chain initial height
	InitHeight uint64 `mapstructure:"init_height"`

	// The root directory for all data.
	// This should be set in viper so it can unmarshal into this struct
	RootDir string `mapstructure:"home"`

	// Path to the JSON file containing the initial validator set and other meta data
	Genesis string `mapstructure:"genesis_file"`

	// Path to the JSON file containing the private key to use as a validator in the consensus protocol
	PrivValidator string `mapstructure:"priv_validator_file"`

	// A custom human readable name for this node
	Moniker string `mapstructure:"moniker"` //nodetype_hostname

	// Output level for logging
	LogLevel string `mapstructure:"log_level"`

	// Output file for rotate logging
	LogFile string `mapstructure:"log_file"`

	// TCP or UNIX socket address for the profiling server to listen on
	ProfListenAddress string `mapstructure:"pprof"`

	// If this node is many blocks behind the tip of the chain, FastSync
	// allows them to catchup quickly by downloading blocks in parallel
	// and verifying their commits
	FastSync bool `mapstructure:"fast_sync"`

	// If true, the app can decide a new peer if we should keep the connection or not
	FilterPeers bool `mapstructure:"filter_peers"` // false

	// Database backend: leveldb | memdb
	DBBackend string `mapstructure:"db_backend"`

	// Database directory
	DBPath string `mapstructure:"db_path"`

	// Database split counts
	DBCounts uint64 `mapstructure:"db_counts"`

	// LogPath directory
	LogPath string `mapstructure:"log_dir"`

	// KeyStore directory
	KeyStorePath string `mapstructure:"keystore_dir"`

	// if false, the node init with default account's state
	OnLine bool `mapstructure:"on_line"`

	InfoAddr string `mapstructure:"info_addr"`

	InfoPrefix string `mapstructure:"info_prefix"`

	// jspath
	ExecFlag string `mapstructure:"exec"`

	// wasm gas rate
	WasmGasRate uint64 `mapstructure:"wasm_gas_rate"`

	RollBack bool `mapstructure:"roll_back"`

	FullNode bool `mapstructure:"full_node"`

	InitStateRoot string `mapstructure:"init_state_root"`

	KeepLatestBlocks  uint64 `mapstructure:"keep_latest_blocks"`
	ClearDataInterval uint64 `mapstructure:"clear_data_interval"`

	SaveBalanceRecord bool `mapstructure:"save_balance_record"`

	IsTestMode bool `mapstructure:"is_test_mode"`

	TestNet bool `mapstructure:"test_net"`
}

// DefaultBaseConfig returns a default base configuration for a node
func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		ChainID:           "chainID",
		Genesis:           defaultGenesisJSONPath,
		PrivValidator:     defaultPrivValPath,
		Moniker:           defaultMoniker,
		LogLevel:          DefaultPackageLogLevels(),
		ProfListenAddress: "",
		FastSync:          true,
		FilterPeers:       false,
		DBBackend:         "leveldb",
		DBPath:            defaultDataDir,
		LogPath:           defaultLogDir,
		KeyStorePath:      defaultKeyStoreDir,
		OnLine:            false,
		InfoAddr:          ":40001",
		InfoPrefix:        "o_blockchain_data",
		ExecFlag:          "",
		WasmGasRate:       1,
		RollBack:          false,
		FullNode:          false,
		KeepLatestBlocks:  0,
		ClearDataInterval: 300,
		SaveBalanceRecord: false,
		IsTestMode:        false,
	}
}

// TestBaseConfig returns a base configuration for testing a node
func TestBaseConfig() BaseConfig {
	cfg := DefaultBaseConfig()
	cfg.ChainID = "blockchain_test"
	cfg.FastSync = false
	cfg.DBBackend = "memdb"
	return cfg
}

// GenesisFile returns the full path to the genesis.json file
func (cfg BaseConfig) GenesisFile() string {
	return rootify(cfg.Genesis, cfg.RootDir)
}

// PrivValidatorFile returns the full path to the priv_validator.json file
func (cfg BaseConfig) PrivValidatorFile() string {
	return rootify(cfg.PrivValidator, cfg.RootDir)
}

// DBDir returns the full path to the database directory
func (cfg BaseConfig) DBDir() string {
	return rootify(cfg.DBPath, cfg.RootDir)
}

// LogDir returns the full path to the log directory
func (cfg BaseConfig) LogDir() string {
	return rootify(cfg.LogPath, cfg.RootDir)
}

// KeyStoreDir returns the full path to the keystore directory
func (cfg BaseConfig) KeyStoreDir() string {
	return rootify(cfg.KeyStorePath, cfg.RootDir)
}

// DefaultLogLevel returns a default log level of "info"
func DefaultLogLevel() string {
	return "info"
}

// DefaultPackageLogLevels returns a default log level setting so all packages
// log at "error", while the `state` and `main` packages log at "info"
func DefaultPackageLogLevels() string {
	return fmt.Sprintf("main:info,state:info,*:%s", DefaultLogLevel())
}

//-----------------------------------------------------------------------------
// RotateConfig

// RotateConfig defines the configuration options for the Log
//-----------------------------------------------------------------------------
func DefaultRotateConfig() *log.RotateConfig {
	return &log.RotateConfig{
		Filename:   defaultLogFileName,
		Daily:      true,
		MaxDays:    7,
		Rotate:     true,
		RotatePerm: "0444",
		Perm:       "0664",
	}
}

func TestRotateConfig() *log.RotateConfig {
	return &log.RotateConfig{
		Filename:   defaultLogFileName,
		Hourly:     true,
		MaxDays:    7,
		Rotate:     true,
		RotatePerm: "0444",
		Perm:       "0664",
	}
}

// RPCConfig

// RPCConfig defines the configuration options for the RPC server
type RPCConfig struct {
	RootDir      string        `mapstructure:"home"`
	IpcEndpoint  string        `mapstructure:"ipc_endpoint"`
	HTTPEndpoint string        `mapstructure:"http_endpoint"`
	HTTPModules  []string      `mapstructure:"http_modules"`
	HTTPCores    []string      `mapstructure:"http_cores"`
	VHosts       []string      `mapstructure:"vhosts"`
	WSEndpoint   string        `mapstructure:"ws_endpoint"`
	WSModules    []string      `mapstructure:"ws_modules"`
	WSOrigins    []string      `mapsturcture:"ws_origins"`
	WSExposeAll  bool          `mapstructure:"ws_expose_all"`
	EVMInterval  time.Duration `mapstructure:"evm_interval"`
	EVMMax       int           `mapstructure:"evm_max"`
}

// DefaultRPCConfig returns a default configuration for the RPC server
func DefaultRPCConfig() *RPCConfig {
	return &RPCConfig{
		IpcEndpoint:  "linkchain.ipc",
		HTTPEndpoint: "127.0.0.1:16000",
		HTTPModules:  []string{"web3", "eth", "personal", "debug", "txpool", "net"},
		HTTPCores:    []string{"*"},
		VHosts:       []string{"*"},
		WSEndpoint:   "127.0.0.1:18000",
		WSModules:    []string{"web3", "eth", "personal", "debug", "txpool", "net", "lk"},
		WSExposeAll:  true,
		WSOrigins:    []string{"*"},
		EVMInterval:  500 * time.Millisecond,
		EVMMax:       100,
	}
}

// TestRPCConfig returns a configuration for testing the RPC server
func TestRPCConfig() *RPCConfig {
	cfg := DefaultRPCConfig()
	cfg.HTTPEndpoint = "127.0.0.1:46000"
	cfg.WSEndpoint = "127.0.0.1:48000"
	return cfg
}

func (cfg *RPCConfig) IPCFile() string {
	return rootify(cfg.IpcEndpoint, cfg.RootDir)
}

//-----------------------------------------------------------------------------
// P2PConfig defines the configuration options for the peer-to-peer networking layer
type P2PConfig struct {
	// Address to listen for incoming connections
	ListenAddress string `mapstructure:"laddr"` //ip:port

	// Address to advertise to peers for them to dial
	ExternalAddress string `mapstructure:"external_address"` //ip

	// Maximum number of peers to connect to
	MaxNumPeers int `mapstructure:"max_num_peers"`

	// Minimum number of outbound peers
	MinOutboundPeers int `mapstructure:"min_outbound_peers"`

	// Maximum size of a message packet payload, in bytes
	MaxPacketMsgPayloadSize int `mapstructure:"max_packet_msg_payload_size"`

	// Peer connection configuration.
	HandshakeTimeout time.Duration `mapstructure:"handshake_timeout"`
	DialTimeout      time.Duration `mapstructure:"dial_timeout"`
}

// DefaultP2PConfig returns a default configuration for the peer-to-peer layer
func DefaultP2PConfig() *P2PConfig {
	return &P2PConfig{
		ListenAddress:    ":13500",
		ExternalAddress:  "",
		MaxNumPeers:      50,
		MinOutboundPeers: 10,
		//MaxPacketMsgPayloadSize: 5 * 1024 * 1024, // 5M
		MaxPacketMsgPayloadSize: 32 * 1024,
		HandshakeTimeout:        20 * time.Second,
		DialTimeout:             3 * time.Second,
	}
}

// TestP2PConfig returns a configuration for testing the peer-to-peer layer
func TestP2PConfig() *P2PConfig {
	cfg := DefaultP2PConfig()
	cfg.ListenAddress = "tcp://0.0.0.0:36656"
	return cfg
}

// FuzzConnConfig is a FuzzedConnection configuration.
type FuzzConnConfig struct {
	Mode         int
	MaxDelay     time.Duration
	ProbDropRW   float64
	ProbDropConn float64
	ProbSleep    float64
}

// DefaultFuzzConnConfig returns the default config.
func DefaultFuzzConnConfig() *FuzzConnConfig {
	return &FuzzConnConfig{
		Mode:         FuzzModeDrop,
		MaxDelay:     3 * time.Second,
		ProbDropRW:   0.2,
		ProbDropConn: 0.00,
		ProbSleep:    0.00,
	}
}

//-----------------------------------------------------------------------------
// MempoolConfig

// MempoolConfig defines the configuration options for the mempool
type MempoolConfig struct {
	RootDir           string        `mapstructure:"home"`
	Recheck           bool          `mapstructure:"recheck"`
	RecheckEmpty      bool          `mapstructure:"recheck_empty"`
	Broadcast         bool          `mapstructure:"broadcast"`
	BroadcastChanSize int           `mapstructure:"broadcast_size"`
	WalPath           string        `mapstructure:"wal_dir"`
	Size              int           `mapstructure:"size"`
	MaxReapSize       int           `mapstructure:"max_reapSize"`
	SpecSize          int           `mapstructure:"specialTxsSize"`
	UTXOSize          int           `mapstructure:"UTXOSize"`
	FutureSize        int           `mapstructure:"future_size"` // Maximum number of non-executable transaction slots for all accounts
	CacheSize         int           `mapstructure:"cache_size"`
	AccountQueue      int           `mapstructure:"account_queue"` // Maximum number of non-executable transaction slots permitted per account
	Lifetime          time.Duration `mapstructure:"life_time"`     // Maximum amount of time non-executable transaction are queued
	RemoveFutureTx    bool          `mapstructure:"removeFutureTx"`
	ReceiveP2pTx      bool          `mapstructure:"receive_p2pTx"`
}

// DefaultMempoolConfig returns a default configuration for the mempool
func DefaultMempoolConfig() *MempoolConfig {
	return &MempoolConfig{
		Recheck:           true,
		RecheckEmpty:      true,
		Broadcast:         true,
		BroadcastChanSize: 10000,
		WalPath:           filepath.Join(defaultDataDir, "mempool.wal"),
		Size:              3000,
		MaxReapSize:       10000,
		SpecSize:          100,
		UTXOSize:          1000,
		CacheSize:         203000,
		FutureSize:        100000,
		AccountQueue:      1000,
		Lifetime:          60 * time.Second,
		RemoveFutureTx:    false,
		ReceiveP2pTx:      false,
	}
}

// TestMempoolConfig returns a configuration for testing the mempool
func TestMempoolConfig() *MempoolConfig {
	cfg := DefaultMempoolConfig()
	cfg.CacheSize = 1000
	return cfg
}

// WalDir returns the full path to the mempool's write-ahead log
func (cfg *MempoolConfig) WalDir() string {
	return rootify(cfg.WalPath, cfg.RootDir)
}

//-----------------------------------------------------------------------------
// ConsensusConfig

// ConsensusConfig defines the configuration for the consensus service,
// including timeouts and details about the WAL and the block structure.
type ConsensusConfig struct {
	RootDir string `mapstructure:"home"`
	WalPath string `mapstructure:"wal_file"`
	walFile string // overrides WalPath if set

	// All timeouts are in milliseconds
	TimeoutPropose        int `mapstructure:"timeout_propose"`
	TimeoutProposeDelta   int `mapstructure:"timeout_propose_delta"`
	TimeoutPrevote        int `mapstructure:"timeout_prevote"`
	TimeoutPrevoteDelta   int `mapstructure:"timeout_prevote_delta"`
	TimeoutPrecommit      int `mapstructure:"timeout_precommit"`
	TimeoutPrecommitDelta int `mapstructure:"timeout_precommit_delta"`
	TimeoutCommit         int `mapstructure:"timeout_commit"`

	// rewardChain
	CurrentTimeoutCommit int

	// Make progress as soon as we have all the precommits (as if TimeoutCommit = 0)
	SkipTimeoutCommit bool `mapstructure:"skip_timeout_commit"`

	// EmptyBlocks mode and possible interval between empty blocks in seconds
	CreateEmptyBlocks         bool `mapstructure:"create_empty_blocks"`
	CreateEmptyBlocksInterval int  `mapstructure:"create_empty_blocks_interval"`

	// Reactor sleep duration parameters are in milliseconds
	PeerGossipSleepDuration     int `mapstructure:"peer_gossip_sleep_duration"`
	PeerQueryMaj23SleepDuration int `mapstructure:"peer_query_maj23_sleep_duration"`
}

// DefaultConsensusConfig returns a default configuration for the consensus service
func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		WalPath:                     filepath.Join(defaultDataDir, "cs.wal", "wal"),
		TimeoutPropose:              5000, //4000,
		TimeoutProposeDelta:         650,
		TimeoutPrevote:              5000, //1500,
		TimeoutPrevoteDelta:         650,
		TimeoutPrecommit:            2000, //1500,
		TimeoutPrecommitDelta:       650,
		TimeoutCommit:               1500,
		CurrentTimeoutCommit:        0,
		SkipTimeoutCommit:           false,
		CreateEmptyBlocks:           true,
		CreateEmptyBlocksInterval:   0,
		PeerGossipSleepDuration:     100,
		PeerQueryMaj23SleepDuration: 2000,
	}
}

// TestConsensusConfig returns a configuration for testing the consensus service
func TestConsensusConfig() *ConsensusConfig {
	cfg := DefaultConsensusConfig()
	cfg.TimeoutPropose = 100
	cfg.TimeoutProposeDelta = 1
	cfg.TimeoutPrevote = 10
	cfg.TimeoutPrevoteDelta = 1
	cfg.TimeoutPrecommit = 10
	cfg.TimeoutPrecommitDelta = 1
	cfg.TimeoutCommit = 10
	cfg.SkipTimeoutCommit = true
	cfg.PeerGossipSleepDuration = 5
	cfg.PeerQueryMaj23SleepDuration = 250
	return cfg
}

// WaitForTxs returns true if the consensus should wait for transactions before entering the propose step
func (cfg *ConsensusConfig) WaitForTxs() bool {
	return !cfg.CreateEmptyBlocks || cfg.CreateEmptyBlocksInterval > 0
}

// EmptyBlocks returns the amount of time to wait before proposing an empty block or starting the propose timer if there are no txs available
func (cfg *ConsensusConfig) EmptyBlocksInterval() time.Duration {
	//no more then 10min since consensus will recover if no new block produced in 15min
	if cfg.CreateEmptyBlocksInterval > 600 {
		cfg.CreateEmptyBlocksInterval = 600
	}
	return time.Duration(cfg.CreateEmptyBlocksInterval) * time.Second
}

// Propose returns the amount of time to wait for a proposal
func (cfg *ConsensusConfig) Propose(round int) time.Duration {
	return time.Duration(cfg.TimeoutPropose+cfg.TimeoutProposeDelta*round) * time.Millisecond
}

// Prevote returns the amount of time to wait for straggler votes after receiving any +2/3 prevotes
func (cfg *ConsensusConfig) Prevote(round int) time.Duration {
	return time.Duration(cfg.TimeoutPrevote+cfg.TimeoutPrevoteDelta*round) * time.Millisecond
}

// Precommit returns the amount of time to wait for straggler votes after receiving any +2/3 precommits
func (cfg *ConsensusConfig) Precommit(round int) time.Duration {
	return time.Duration(cfg.TimeoutPrecommit+cfg.TimeoutPrecommitDelta*round) * time.Millisecond
}

// Commit returns the amount of time to wait for straggler votes after receiving +2/3 precommits for a single block (ie. a commit).
func (cfg *ConsensusConfig) Commit(t time.Time) time.Time {
	if cfg.CurrentTimeoutCommit > 0 {
		return t.Add(time.Duration(cfg.CurrentTimeoutCommit) * time.Millisecond)
	}
	return t.Add(time.Duration(cfg.TimeoutCommit) * time.Millisecond)
}

// PeerGossipSleep returns the amount of time to sleep if there is nothing to send from the ConsensusReactor
func (cfg *ConsensusConfig) PeerGossipSleep() time.Duration {
	return time.Duration(cfg.PeerGossipSleepDuration) * time.Millisecond
}

// PeerQueryMaj23Sleep returns the amount of time to sleep after each VoteSetMaj23Message is sent in the ConsensusReactor
func (cfg *ConsensusConfig) PeerQueryMaj23Sleep() time.Duration {
	return time.Duration(cfg.PeerQueryMaj23SleepDuration) * time.Millisecond
}

// WalFile returns the full path to the write-ahead log file
func (cfg *ConsensusConfig) WalFile() string {
	if cfg.walFile != "" {
		return cfg.walFile
	}
	return rootify(cfg.WalPath, cfg.RootDir)
}

// SetWalFile sets the path to the write-ahead log file
func (cfg *ConsensusConfig) SetWalFile(walFile string) {
	cfg.walFile = walFile
}

//-----------------------------------------------------------------------------
// InstrumentationConfig

// InstrumentationConfig defines the configuration for metrics reporting.
type InstrumentationConfig struct {
	// When true, Prometheus metrics are served under /metrics on
	// PrometheusListenAddr.
	// Check out the documentation for the list of available metrics.
	Prometheus bool `mapstructure:"prometheus"`

	// Address to listen for Prometheus collector(s) connections.
	PrometheusListenAddr string `mapstructure:"prometheus_listen_addr"`

	// Maximum number of simultaneous connections.
	// If you want to accept more significant number than the default, make sure
	// you increase your OS limits.
	// 0 - unlimited.
	MaxOpenConnections int `mapstructure:"max_open_connections"`
}

// DefaultInstrumentationConfig returns a default configuration for metrics
// reporting.
func DefaultInstrumentationConfig() *InstrumentationConfig {
	return &InstrumentationConfig{
		Prometheus:           false,
		PrometheusListenAddr: ":26660",
		MaxOpenConnections:   3,
	}
}

// TestInstrumentationConfig returns a default configuration for metrics
// reporting.
func TestInstrumentationConfig() *InstrumentationConfig {
	return DefaultInstrumentationConfig()
}

//-----------------------------------------------------------------------------

// BootNodeConfig defines the configuration of bootnode
type BootNodeConfig struct {
	Addrs []string `mapstructure:"addrs"` //https://ip1:port1,https://ip2:port2
}

//bootnode
func DefaultBootNodeConfig() *BootNodeConfig {
	return &BootNodeConfig{
		Addrs: []string{},
	}
}

//-----------------------------------------------------------------------------
// Utils

// helper function to make config creation independent of root dir
func rootify(path, root string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

//-----------------------------------------------------------------------------
// Moniker

var defaultMoniker = getDefaultMoniker()

// getDefaultMoniker returns a default moniker, which is the host name. If runtime
// fails to get the host name, "anonymous" will be returned.
func getDefaultMoniker() string {
	moniker, err := os.Hostname()
	if err != nil {
		moniker = "anonymous"
	}
	return moniker
}
