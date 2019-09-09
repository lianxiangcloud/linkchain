package commands

import (
	"fmt"

	"github.com/lianxiangcloud/linkchain/bootnode"
	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/log"
	nm "github.com/lianxiangcloud/linkchain/node"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/spf13/cobra"
)

// AddNodeFlags exposes some common configuration options on the command-line
// These are exposed for convenience of commands embedding a node
func AddNodeFlags(cmd *cobra.Command) {
	// bind flags
	cmd.Flags().String("moniker", config.BaseConfig.Moniker, "Node Name")

	// node flags
	cmd.Flags().Bool("fast_sync", config.BaseConfig.FastSync, "Fast blockchain syncing")
	cmd.Flags().String("info_addr", config.BaseConfig.InfoAddr, "The UDP addr of infoData")
	cmd.Flags().String("info_prefix", config.BaseConfig.InfoPrefix, "The prefix of infoData")
	cmd.Flags().String("pprof", config.BaseConfig.ProfListenAddress, "The http pprof server address")
	cmd.Flags().Bool("roll_back", config.BaseConfig.RollBack, "roll-back one block, default false")
	cmd.Flags().Uint64("wasm_gas_rate", config.BaseConfig.WasmGasRate, "wasm vm gas rate,default 1")
	cmd.Flags().Bool("is_test_mode", config.BaseConfig.IsTestMode, "for test")
	cmd.Flags().Bool("test_net", config.BaseConfig.TestNet, "signparam will be set to 29154 if this flag is set")
	// rpc flags
	cmd.Flags().StringSlice("rpc.http_modules", config.RPC.HTTPModules, "API's offered over the HTTP-RPC interface")
	cmd.Flags().String("rpc.http_endpoint", config.RPC.HTTPEndpoint, "RPC listen address. Port required")
	cmd.Flags().String("rpc.ws_endpoint", config.RPC.WSEndpoint, " WS-RPC server listening address. Port required")
	cmd.Flags().StringSlice("rpc.ws_modules", config.RPC.WSModules, "API's offered over the WS-RPC interface")
	cmd.Flags().Bool("rpc.ws_expose_all", config.RPC.WSExposeAll, "Enable the WS-RPC server to expose all APIs")
	cmd.Flags().String("rpc.ipc_endpoint", config.RPC.IpcEndpoint, "Filename for IPC socket/pipe within the datadir (explicit paths escape it)")
	cmd.Flags().Duration("rpc.evm_interval", config.RPC.EVMInterval, "Rate for evm call and estimate")
	cmd.Flags().Int("rpc.evm_max", config.RPC.EVMMax, "Maximum evm created by evm call and estimate")

	// p2p flags
	cmd.Flags().String("p2p.laddr", config.P2P.ListenAddress, "Node listen address. (0.0.0.0:0 means any interface, any port)")
	cmd.Flags().Int("p2p.max_num_peers", config.P2P.MaxNumPeers, "max p2p connection num")
	// consensus flags
	cmd.Flags().Bool("consensus.create_empty_blocks", config.Consensus.CreateEmptyBlocks, "Set this to false to only produce blocks when there are txs or when the AppHash changes")
	cmd.Flags().Int("consensus.create_empty_blocks_interval", config.Consensus.CreateEmptyBlocksInterval, "the interval time between two empty block")
	cmd.Flags().Int("consensus.timeout_commit", config.Consensus.TimeoutCommit, "the interval between blocks in ms(Milliseconds)")

	cmd.Flags().Duration("mempool.life_time", config.Mempool.Lifetime, "Life time of cached future transactions in mempool")
	cmd.Flags().Bool("mempool.removeFutureTx", config.Mempool.RemoveFutureTx, "Remove future tx when mempool future tx queue is full")
	cmd.Flags().Int("mempool.size", config.Mempool.Size, "max size in good tx")
	cmd.Flags().Int("mempool.max_reapSize", config.Mempool.MaxReapSize, "reap txs num of block")
	// log
	cmd.Flags().String("log.filename", config.Log.Filename, "log file name")

	cmd.Flags().Bool("full_node", config.BaseConfig.FullNode, "light-weight node or full node")
	cmd.Flags().Uint64("keep_latest_blocks", config.BaseConfig.KeepLatestBlocks, "number of latest blocks to keep")
	cmd.Flags().Uint64("clear_data_interval", config.BaseConfig.ClearDataInterval, "number of seconds between two startup cleanups")
	cmd.Flags().Bool("save_balance_record", config.BaseConfig.SaveBalanceRecord, "open transactions record storage")
	//bootnode
	cmd.Flags().String("bootnode.addr", config.BootNodeSvr.Addr, "Addr or filepath of the bootnode")
}

// NewRunNodeCmd returns the command that allows the CLI to start a node.
// It can be used with a custom PrivValidator and in-process ABCI application.
func NewRunNodeCmd(nodeProvider nm.NodeProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Run the node",
		RunE: func(cmd *cobra.Command, args []string) error {
			if config.OnLine && config.IsTestMode {
				return fmt.Errorf("Don't config IsTestMode when OnLine is true")
			}
			//set types.IsTestMode
			types.IsTestMode = config.IsTestMode
			types.InitSignParam(config.TestNet)

			cfg.WasmGasRate = config.BaseConfig.WasmGasRate
			// Create & start node
			n, err := nodeProvider(config, logger)
			if err != nil {
				return fmt.Errorf("Failed to create node: %v", err)
			}

			if err := n.Start(); err != nil {
				return fmt.Errorf("Failed to start node: %v", err)
			}
			logger.Info("Started node", "nodeInfo", n.P2PManager().LocalNodeInfo())

			filter, ok := logger.(*log.Filter)
			if ok {
				err = filter.SetBaseInfo(config.InfoAddr, config.InfoPrefix, bootnode.GetLocalNodeType().String())
				logger.Info("log.Filter SetBaseInfo", "InfoAddr", config.InfoAddr, "err", err)
				// call log.Report(msg, "logID", 70001, "height", 8, "validators", 9, "peers", 10)
			}

			// Trap signal, run forever.
			n.RunForever()

			return nil
		},
	}

	AddNodeFlags(cmd)
	return cmd
}
