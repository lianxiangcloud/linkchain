package commands

import (
	"fmt"

	"github.com/lianxiangcloud/linkchain/version"
	"github.com/spf13/cobra"
)

// VersionCmd ...
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version info",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("linkchain version: %s, gitCommit:%s \n", version.Version, version.GitCommit)
	},
}
