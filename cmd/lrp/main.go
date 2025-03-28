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
	rootCmd.AddCommand(commands.AddCmd)
	// rootCmd.AddCommand(commands.RenderCmd)
	// rootCmd.AddCommand(commands.DockerCmd)

	commands.WithPathFlags(rootCmd)
	commands.WithGitFlags(rootCmd)
	commands.WithExtraFlags(rootCmd)
	commands.WithPathFlags(commands.KeyCmd)
	commands.WithPathFlags(commands.StatsCmd)
	commands.WithPathFlags(commands.AddCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
