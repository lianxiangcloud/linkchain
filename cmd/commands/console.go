package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/lianxiangcloud/linkchain/console"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/spf13/cobra"
)

func registerAttachFlags(cmd *cobra.Command) {
	cmd.Flags().String("home", config.RootDir, "")
}

func attach(cmd *cobra.Command, args []string) error {
	endpoint := args[0]
	if endpoint == "" {
		endpoint = config.RPC.IPCFile()
	}

	client, err := dialRPC(endpoint)
	if err != nil {
		return err
	}

	cfg := console.Config{
		DataDir: config.RootDir,
		DocRoot: ".",
		Client:  client,
		Preload: []string{},
	}

	console, err := console.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to start the JavaScript console: %v", err)
	}
	defer console.StopAndClearHistory(false)

	if script := config.ExecFlag; script != "" {
		console.Evaluate(script)
		return nil
	}

	// Otherwise print the welcome screen and enter interactive mode
	console.Welcome()
	console.Interactive()

	return nil
}

func NewConsoleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "attach",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("expect rpc server address, see help")
			}
			return nil
		},
		Example: "linkchain attach http://127.0.0.1:16000",
		Short:   "Start an interactive JavaScript environment (connect to node)",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			handler := log.StreamHandler(ioutil.Discard, log.TerminalFormat(true))
			logger.SetHandler(handler)
			logger = logger.With("module", "console")
		},
		RunE: attach,
		PostRun: func(cmd *cobra.Command, args []string) {
			console.Stdin.Close()
		},
	}
	registerAttachFlags(cmd)
	return cmd
}

// dialRPC returns a RPC client which connects to the given endpoint.
func dialRPC(endpoint string) (*rpc.Client, error) {
	if endpoint == "" {
		return nil, errors.New("")
	}
	if strings.HasPrefix(endpoint, "rpc:") || strings.HasPrefix(endpoint, "ipc:") {
		// Backwards compatibility with geth < 1.5 which required
		// these prefixes.
		endpoint = endpoint[4:]
	}
	return rpc.Dial(endpoint)
}
