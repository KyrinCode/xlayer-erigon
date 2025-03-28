package commands

import (
	"path/filepath"
	"time"

	"github.com/ledgerwatch/erigon/cmd/lrp/utils"
	"github.com/spf13/cobra"
)

var (
	path     string
	branch   string
	commitID string

	chaindata     string
	backupUnwound bool
	ignoreRunning bool
	fuse          bool

	vmtouch    bool
	parallel   int
	sampleIntv time.Duration
	custom     bool
)

func WithPathFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&path, "path", "p", utils.GetDefaultPath(path), "the root directory which will store the whole testing data, including repo and workspace(recommended to use default)")
}

func WithGitFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&commitID, "commitID", "c", "", "which commit to checkout to build the erigon client")
	cmd.Flags().StringVarP(&branch, "branch", "b", utils.DEFAULT_BRANCH, "which branch to checkout to build the erigon client(lose effect if commitID is set)")
}

func WithExtraFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&chaindata, "chaindata", filepath.Join(utils.GetDefaultPath(path), utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), "the directory which will be imported to the testing environment(e.g. ~/Downloads/mainnet/seq)")
	cmd.Flags().BoolVar(&backupUnwound, "backup", false, "determine whether to backup unwound chaindata")
	cmd.Flags().BoolVarP(&ignoreRunning, "ignoreRunning", "i", false, "determine whether to ignore other tests that are already running")
	cmd.Flags().BoolVar(&fuse, "fuse", false, "if true, monitor the mainnet data directory size and fuse replay when the size exceeds the limit")
	cmd.Flags().DurationVar(&sampleIntv, "sample", utils.DEFAULT_SAMPLE_INTERVAL, "set the sampling interval for the Docker container, the minimum value is 1 second")
	cmd.Flags().BoolVar(&vmtouch, "vmtouch", false, "when enabled, the replay container will run on vmtouch mode")
	cmd.Flags().IntVar(&parallel, "parallel", utils.DEFAULT_PROCESS_COUNT, "determine how many process will run for multi-process test")
	cmd.Flags().BoolVar(&custom, "custom", false, "if true, you can customize the playback range, otherwise use the preset range. The default is false")
}
