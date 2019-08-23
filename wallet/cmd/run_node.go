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
	// cmd.Flags().String("password", config.BaseConfig.Password, "Wallet password")
	// cmd.Flags().String("password_file", config.BaseConfig.PasswordFile, "Wallet password file")
	// cmd.Flags().String("keystore_file", config.BaseConfig.KeystoreFile, "Use KeystoreFile")
	// cmd.Flags().Int("kdf_rounds", config.BaseConfig.KdfRounds, "Number of rounds for the key derivation function")
	cmd.Flags().Bool("detach", config.BaseConfig.Detach, "Run as daemon")
	cmd.Flags().Int("max_concurrency", config.BaseConfig.MaxConcurrency, "Max number of threads to use for a parallel job")
	// cmd.Flags().String("pidfile", config.BaseConfig.Pidfile, "File path to write the daemon's PID to")
	cmd.Flags().String("log_level", config.BaseConfig.LogLevel, "0-4 or categories")
	cmd.Flags().String("home", config.BaseConfig.RootDir, "home")
	cmd.Flags().String("log_dir", config.BaseConfig.LogPath, "log_dir")

	cmd.Flags().String("daemon.peer_rpc", config.Daemon.PeerRPC, "peer rpc url")
	// cmd.Flags().String("daemon.login", config.Daemon.Login, "Specify username[:password] for daemon RPC client")
	// cmd.Flags().Bool("daemon.trusted", config.Daemon.Trusted, "Enable commands which rely on a trusted daemon")
	// cmd.Flags().Bool("daemon.testnet", config.Daemon.Testnet, "For testnet. Daemon must also be launched with --testnet flag")
	cmd.Flags().Bool("test_net", config.BaseConfig.TestNet, "signparam will be set to 29154 if this flag is set")

	// rpc flags
	cmd.Flags().StringSlice("rpc.http_modules", config.RPC.HTTPModules, "API's offered over the HTTP-RPC interface")
	cmd.Flags().String("rpc.http_endpoint", config.RPC.HTTPEndpoint, "RPC listen address. Port required")
	cmd.Flags().String("rpc.ws_endpoint", config.RPC.WSEndpoint, " WS-RPC server listening address. Port required")
	// cmd.Flags().String("rpc.http_eth_endpoint", config.RPC.HTTPEthEndpoint, "Eth RPC listen address. Port required")
	// cmd.Flags().String("rpc.ws_eth_endpoint", config.RPC.WSEthEndpoint, "Eth RPC listen address. Port required")
	cmd.Flags().StringSlice("rpc.ws_modules", config.RPC.WSModules, "API's offered over the WS-RPC interface")
	cmd.Flags().Bool("rpc.ws_expose_all", config.RPC.WSExposeAll, "Enable the WS-RPC server to expose all APIs")
	cmd.Flags().String("rpc.ipc_endpoint", config.RPC.IpcEndpoint, "Filename for IPC socket/pipe within the datadir (explicit paths escape it)")
	// cmd.Flags().String("rpc.bind_ip", config.RPC.BindIP, "Specify IP to bind RPC server")
	// cmd.Flags().Uint64("rpc.bind_port", config.RPC.BindPort, "Sets bind port for server")
	// cmd.Flags().String("rpc.login", config.RPC.Login, "Specify username[:password] required for RPC server")
	// cmd.Flags().Bool("rpc.disable_login", config.RPC.DisableLogin, "Disable HTTP authentication for RPC connections served by this process")
	// cmd.Flags().String("rpc.access_control_origins", config.RPC.AccessControlOrigins, "Specify a comma separated list of origins to allow cross origin resource sharing")
}

// NewRunNodeCmd returns the command that allows the CLI to start a node.
// It can be used with a custom PrivValidator and in-process ABCI application.
func NewRunNodeCmd(nodeProvider nm.NodeProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Run the wallet node",
		RunE: func(cmd *cobra.Command, args []string) error {
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
