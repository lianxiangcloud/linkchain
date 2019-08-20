package commands

import (
	"github.com/spf13/cobra"
	"github.com/lianxiangcloud/linkchain/consensus"
)

// ReplayCmd allows replaying of messages from the WAL.
var ReplayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Replay messages from WAL",
	Run: func(cmd *cobra.Command, args []string) {
		consensus.RunReplayFile(config, false)
	},
}

// ReplayConsoleCmd allows replaying of messages from the WAL in a
// console.
var ReplayConsoleCmd = &cobra.Command{
	Use:   "replay_console",
	Short: "Replay messages from WAL in a console",
	Run: func(cmd *cobra.Command, args []string) {
		consensus.RunReplayFile(config, true)
	},
}
