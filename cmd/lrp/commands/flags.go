package commands

import (
	"github.com/ledgerwatch/erigon/cmd/lrp/utils"
	"github.com/spf13/cobra"
)

var (
	path     string
	branch   string
	commitID string

	chaindata string
)

func WithPathFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&path, "path", "p", utils.GetDefaultPath(path), "the destination directory which will store the testing result(recommended to use default)")
}

func WithGitFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&commitID, "commitID", "c", "", "which commit to checkout to build the erigon client")
	cmd.Flags().StringVarP(&branch, "branch", "b", utils.DEFAULT_BRANCH, "which branch to checkout to build the erigon client(lose effect if commitID is set)")
}

func WithWorkspaceFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&chaindata, "chaindata", utils.DEFAULT_DESTINATION_DIR, "the directory which will be imported to the testing environment")
}
