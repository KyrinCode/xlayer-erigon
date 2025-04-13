package main

import (
	"fmt"
	"os"

	"github.com/ledgerwatch/erigon/cmd/lrp/commands"
)

func main() {
	rootCmd := commands.RootCommand()
	rootCmd.AddCommand(commands.AddCmd)
	rootCmd.AddCommand(commands.InitCmd)

	commands.WithPathFlags(rootCmd)
	commands.WithGitFlags(rootCmd)
	commands.WithExtraFlags(rootCmd)
	commands.WithPathFlags(commands.AddCmd)
	commands.WithPathFlags(commands.InitCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
