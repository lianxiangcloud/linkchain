package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var GitCommit string
var GitBranch string

// VersionCmd ...
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version info",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("wallet git version: %s \n", GitCommit)
		fmt.Printf("wallet git branch: %s \n", GitBranch)
	},
}
