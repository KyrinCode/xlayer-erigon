package main

import (
	"fmt"
	"os"

	"github.com/ledgerwatch/erigon/cmd/lrp/commands"
)

func main() {
	rootCmd := commands.RootCommand()
	rootCmd.AddCommand(commands.KeyCmd)
	rootCmd.AddCommand(commands.StatsCmd)

	commands.WithPathFlags(rootCmd)
	commands.WithGitFlags(rootCmd)
	commands.WithPathFlags(commands.KeyCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
