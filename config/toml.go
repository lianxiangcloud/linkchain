package config

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
)

var configTemplate *template.Template

func init() {
	var err error
	if configTemplate, err = template.New("configFileTemplate").Parse(defaultConfigTemplate); err != nil {
		panic(err)
	}
}

/****** these are for production settings ***********/

// EnsureRoot creates the root, config, and data directories if they don't exist,
// and panics if it fails.
func EnsureRoot(rootDir string, config *Config) {
	if err := cmn.EnsureDir(rootDir, 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}
	if err := cmn.EnsureDir(filepath.Join(rootDir, defaultConfigDir), 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}
	if err := cmn.EnsureDir(filepath.Join(rootDir, defaultDataDir), 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}

	configFilePath := filepath.Join(rootDir, defaultConfigFilePath)

	// Write default config file if missing.
	if !cmn.FileExists(configFilePath) {
		//writeDefaultConfigFile(configFilePath)
		if config == nil {
			WriteConfigFile(configFilePath, DefaultConfig())
		} else {
			WriteConfigFile(configFilePath, config)
		}
	}
}

// XXX: this func should probably be called by cmd/commands/init.go
// alongside the writing of the genesis.json and priv_validator.json
func writeDefaultConfigFile(configFilePath string) {
	WriteConfigFile(configFilePath, DefaultConfig())
}

// WriteConfigFile renders config using the template and writes it to configFilePath.
func WriteConfigFile(configFilePath string, config *Config) {
	var buffer bytes.Buffer

	if err := configTemplate.Execute(&buffer, config); err != nil {
		panic(err)
	}

	cmn.MustWriteFile(configFilePath, buffer.Bytes(), 0644)
}

// Note: any changes to the comments/variables/mapstructure
// must be reflected in the appropriate struct in config/config.go
const defaultConfigTemplate = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

##### main base config options #####

# A custom human readable name for this node
moniker = "{{ .BaseConfig.Moniker }}"

# If this node is many blocks behind the tip of the chain, FastSync
# allows them to catchup quickly by downloading blocks in parallel
# and verifying their commits
fast_sync = {{ .BaseConfig.FastSync }}

# Database backend: leveldb | memdb
db_backend = "{{ .BaseConfig.DBBackend }}"

# Database directory
db_path = "{{ js .BaseConfig.DBPath }}"

# Database split counts
db_counts = "{{ js .BaseConfig.DBCounts }}"

# Output level for logging, including package level options
log_level = "{{ .BaseConfig.LogLevel }}"

##### additional base config options #####

# Path to the JSON file containing the initial validator set and other meta data
genesis_file = "{{ js .BaseConfig.Genesis }}"

# Path to the JSON file containing the private key to use as a validator in the consensus protocol
priv_validator_file = "{{ js .BaseConfig.PrivValidator }}"

# TCP or UNIX socket address for the profiling server to listen on
pprof = "{{ .BaseConfig.ProfListenAddress }}"

# If true, the app can decide a new peer if we should keep the connection or not
filter_peers = {{ .BaseConfig.FilterPeers }}

# wasm gas rate
wasm_gas_rate = {{ .BaseConfig.WasmGasRate }}

#test mode
istestmode = {{ .BaseConfig.IsTestMode }}

##### advanced configuration options #####

##### log rotate configuration options #####
[log]

# Log file name
filename = "{{ .Log.Filename }}"

# Log files kept for maxdays
maxdays = "{{ .Log.MaxDays }}"

# Support log rotate
rotate = "{{ .Log.Rotate }}"

# Rotate log hourly
hourly = "{{ .Log.Hourly }}"

# Rotate log daily
daily = "{{ .Log.Daily }}"

# Rotate file perm
rotateperm = "{{ .Log.RotatePerm }}"

# Log file perm
perm = "{{ .Log.Perm }}"

##### rpc server configuration options #####
[rpc]


# TCP or UNIX socket address for the RPC server to listen on
http_endpoint = "{{ .RPC.HTTPEndpoint }}"

# TCP or UNIX socket address for the WS server to listen on
ws_endpoint = "{{ .RPC.WSEndpoint }}"
http_modules = {{println "["}}{{ range $index, $element := .RPC.HTTPModules }} {{printf "%q,\n" ($element)}} {{ end }}{{"]"}}
ws_modules = {{println "["}}{{ range $index, $element := .RPC.WSModules }} {{printf "%q,\n" ($element)}} {{ end }}{{"]"}}
ws_exposeall = {{ .RPC.WSExposeAll }}

ipcpath = "{{ .RPC.IpcEndpoint }}"

evm_interval = "{{ .RPC.EVMInterval }}"

evm_max = {{ .RPC.EVMMax }}

##### peer to peer configuration options #####
[p2p]

# Address to listen for incoming connections
laddr = "{{ .P2P.ListenAddress }}"

# Address to advertise to peers for them to dial
# If empty, will use the same port as the laddr,
# and will introspect on the listener or use UPnP
# to figure out the address.
external_address = "{{ .P2P.ExternalAddress }}"

# Maximum number of peers to connect to
max_num_peers = {{ .P2P.MaxNumPeers }}

# Maximum size of a message packet payload, in bytes
max_packet_msg_payload_size = {{ .P2P.MaxPacketMsgPayloadSize }}


##### mempool configuration options #####
[mempool]

recheck = {{ .Mempool.Recheck }}
recheck_empty = {{ .Mempool.RecheckEmpty }}
broadcast = {{ .Mempool.Broadcast }}
wal_dir = "{{ js .Mempool.WalPath }}"

# size of the good tx queue
size = {{ .Mempool.Size }}

# max reap txs num of block
max_reapSize = {{ .Mempool.MaxReapSize }}

# size of the cache (used to filter transactions we saw earlier)
cache_size = {{ .Mempool.CacheSize }}

# size of the future tx queue
future_size = {{ .Mempool.FutureSize }}

removeFutureTx = {{ .Mempool.RemoveFutureTx }}

##### consensus configuration options #####
[consensus]

wal_file = "{{ js .Consensus.WalPath }}"

# All timeouts are in milliseconds
timeout_propose = {{ .Consensus.TimeoutPropose }}
timeout_propose_delta = {{ .Consensus.TimeoutProposeDelta }}
timeout_prevote = {{ .Consensus.TimeoutPrevote }}
timeout_prevote_delta = {{ .Consensus.TimeoutPrevoteDelta }}
timeout_precommit = {{ .Consensus.TimeoutPrecommit }}
timeout_precommit_delta = {{ .Consensus.TimeoutPrecommitDelta }}
timeout_commit = {{ .Consensus.TimeoutCommit }}

# Make progress as soon as we have all the precommits (as if TimeoutCommit = 0)
skip_timeout_commit = {{ .Consensus.SkipTimeoutCommit }}

# EmptyBlocks mode and possible interval between empty blocks in seconds
create_empty_blocks = {{ .Consensus.CreateEmptyBlocks }}
create_empty_blocks_interval = {{ .Consensus.CreateEmptyBlocksInterval }}

# Reactor sleep duration parameters are in milliseconds
peer_gossip_sleep_duration = {{ .Consensus.PeerGossipSleepDuration }}
peer_query_maj23_sleep_duration = {{ .Consensus.PeerQueryMaj23SleepDuration }}

##### instrumentation configuration options #####
[instrumentation]

# When true, Prometheus metrics are served under /metrics on
# PrometheusListenAddr.
# Check out the documentation for the list of available metrics.
prometheus = {{ .Instrumentation.Prometheus }}

# Address to listen for Prometheus collector(s) connections
prometheus_listen_addr = "{{ .Instrumentation.PrometheusListenAddr }}"

# Maximum number of simultaneous connections.
# If you want to accept more significant number than the default, make sure
# you increase your OS limits.
# 0 - unlimited.
max_open_connections = {{ .Instrumentation.MaxOpenConnections }}

[bootnode]
addrs = "{{ .BootNodeSvr.Addrs }}"
`

/****** these are for test settings ***********/

func ResetTestRoot(testName string) *Config {
	rootDir := os.ExpandEnv("$HOME/.blockchain_test")
	rootDir = filepath.Join(rootDir, testName)
	// Remove ~/.blockchain_test_bak
	if cmn.FileExists(rootDir + "_bak") {
		if err := os.RemoveAll(rootDir + "_bak"); err != nil {
			cmn.PanicSanity(err.Error())
		}
	}
	// Move ~/.blockchain_test to ~/.blockchain_test_bak
	if cmn.FileExists(rootDir) {
		if err := os.Rename(rootDir, rootDir+"_bak"); err != nil {
			cmn.PanicSanity(err.Error())
		}
	}
	// Create new dir
	if err := cmn.EnsureDir(rootDir, 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}
	if err := cmn.EnsureDir(filepath.Join(rootDir, defaultConfigDir), 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}
	if err := cmn.EnsureDir(filepath.Join(rootDir, defaultDataDir), 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}

	baseConfig := DefaultBaseConfig()
	configFilePath := filepath.Join(rootDir, defaultConfigFilePath)
	genesisFilePath := filepath.Join(rootDir, baseConfig.Genesis)
	privFilePath := filepath.Join(rootDir, baseConfig.PrivValidator)

	// Write default config file if missing.
	if !cmn.FileExists(configFilePath) {
		writeDefaultConfigFile(configFilePath)
	}
	if !cmn.FileExists(genesisFilePath) {
		cmn.MustWriteFile(genesisFilePath, []byte(testGenesis), 0644)
	}
	// we always overwrite the priv val
	cmn.MustWriteFile(privFilePath, []byte(testPrivValidator), 0644)

	config := TestConfig().SetRoot(rootDir)
	return config
}

var testGenesis = `{
	"genesis_time": "2019-08-12 14:30:21.470984537 +0800 CST",
	"chain_id": "chainID",
	"consensus_params": {
	  "block_size_params": {
		"max_bytes": "22020096",
		"max_txs": "10000",
		"max_gas": "5000000000"
	  },
	  "tx_size_params": {
		"max_bytes": "10240",
		"max_gas": "5000000000"
	  },
	  "block_gossip_params": {
		"block_part_size_bytes": "32768"
	  },
	  "evidence_params": {
		"max_age": "100000"
	  }
	},
	"validators": [
	  {
		"pub_key": {
		  "type": "PubKeyEd25519",
		  "value": "0x724c2517228e6aa0022fb404e5280239797cdd703e74e721a1b93fbdc36d1182375edc45749a5ea9"
		},
		"coinbase": "0x0000000000000000000000000000000000000000",
		"power": "10",
		"name": ""
	  }
	]
  }`

var testPrivValidator = `{
	"address": "B03C2966D8F6046ED6F1CCF5E6D30CA3AA7220F3",
	"pub_key": {
	  "type": "PubKeyEd25519",
	  "value": "0x724c2517228e6aa0022fb404e5280239797cdd703e74e721a1b93fbdc36d1182375edc45749a5ea9"
	},
	"last_height": "0",
	"last_round": "0",
	"last_step": 0,
	"priv_key": {
	  "type": "PrivKeyEd25519",
	  "value": "0x9e5e70a1b9af8fb840d71020d61d4ebb2053b5c869ebfbd9dae46c47fe4e68d0be72a0e2359c2819f9022fb404e5280239797cdd703e74e721a1b93fbdc36d1182375edc45749a5ea9"
	}
  }`
