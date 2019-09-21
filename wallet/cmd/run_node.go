package main

import (
	"fmt"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/types"
	cfg "github.com/lianxiangcloud/linkchain/wallet/config"
	nm "github.com/lianxiangcloud/linkchain/wallet/node"
	"github.com/spf13/cobra"
)

// AddWalletFlags exposes some common configuration options on the command-line
// These are exposed for convenience of commands embedding a node
func AddNodeFlags(cmd *cobra.Command) {
	// bind flags
	cmd.Flags().Bool("detach", config.BaseConfig.Detach, "Run as daemon")
	cmd.Flags().Int("max_concurrency", config.BaseConfig.MaxConcurrency, "Max number of threads to use for a parallel job")
	// cmd.Flags().String("pidfile", config.BaseConfig.Pidfile, "File path to write the daemon's PID to")
	cmd.Flags().String("log_level", config.BaseConfig.LogLevel, "0-4 or categories")
	cmd.Flags().String("home", config.BaseConfig.RootDir, "home")
	cmd.Flags().String("log_dir", config.BaseConfig.LogPath, "log_dir")
	cmd.Flags().Bool("test_net", config.BaseConfig.TestNet, "signparam will be set to 29154 if this flag is set")

	cmd.Flags().String("daemon.peer_rpc", config.Daemon.PeerRPC, "peer rpc url")
	cmd.Flags().Bool("daemon.sync_quick", config.Daemon.SyncQuick, "wallet sync block use quick api")
	cmd.Flags().String("daemon.nc", config.Daemon.NC, "set daemon header nc")
	cmd.Flags().String("daemon.origin", config.Daemon.Origin, "set daemon header origin")
	cmd.Flags().String("daemon.appversion", config.Daemon.Appversion, "set daemon header appversion")

	// rpc flags
	cmd.Flags().StringSlice("rpc.http_modules", config.RPC.HTTPModules, "API's offered over the HTTP-RPC interface")
	cmd.Flags().String("rpc.http_endpoint", config.RPC.HTTPEndpoint, "RPC listen address. Port required")
	cmd.Flags().String("rpc.ws_endpoint", config.RPC.WSEndpoint, " WS-RPC server listening address. Port required")
	cmd.Flags().StringSlice("rpc.ws_modules", config.RPC.WSModules, "API's offered over the WS-RPC interface")
	cmd.Flags().Bool("rpc.ws_expose_all", config.RPC.WSExposeAll, "Enable the WS-RPC server to expose all APIs")
	cmd.Flags().String("rpc.ipc_endpoint", config.RPC.IpcEndpoint, "Filename for IPC socket/pipe within the datadir (explicit paths escape it)")
}

// NewRunNodeCmd returns the command that allows the CLI to start a node.
// It can be used with a custom PrivValidator and in-process ABCI application.
func NewRunNodeCmd(nodeProvider nm.NodeProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Run the wallet node",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.BaseConfig.SavePid()
			if err != nil {
				panic(err)
			}
			logger.Info("NewRunNodeCmd", "base", config.BaseConfig, "daemon", config.Daemon, "rpc", config.RPC, "log", config.Log)
			fmt.Printf("conf:%v\n", *config)

			types.InitSignParam(config.TestNet)

			initKeyStoreDir(config)
			// Create & start node
			n, err := nodeProvider(config, logger.With("module", "node"))
			if err != nil {
				return fmt.Errorf("Failed to create node: %v", err)
			}

			if err := n.Start(); err != nil {
				return fmt.Errorf("Failed to start node: %v", err)
			}
			logger.Info("Started node", "nodeInfo", "n.Switch().NodeInfo()")

			// Trap signal, run forever.
			n.RunForever()

			return nil
		},
	}

	AddNodeFlags(cmd)
	return cmd
}

func initKeyStoreDir(config *cfg.Config) error {
	keystoreDir := config.KeyStoreDir()
	if err := cmn.EnsureDir(keystoreDir, 0700); err != nil {
		cmn.PanicSanity(err.Error())
	}
	return nil
}
