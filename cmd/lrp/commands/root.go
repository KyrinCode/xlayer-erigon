package commands

import (
	"context"
	"log"
	"path/filepath"
	"sync"

	"github.com/ledgerwatch/erigon/cmd/lrp/utils"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lrp",
	Short: "LRP is the root command of running lrp test",
	Long:  `LRP is the root command of running lrp test`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Step 0: Check the environment
		if err := utils.CheckEnviorment(path); err != nil {
			return err
		}

		// Step 1: checkout to the target branch or commit
		repoPath := filepath.Join(path, utils.REPO_NAME)
		commitID, err := utils.CheckoutGitTarget(repoPath, branch, commitID)
		if err != nil {
			return err
		}

		// Step 2-1: prepare - create a work directory
		workDir, config, err := utils.SpawnWorkDirectory(path, commitID)
		if err != nil {
			return err
		}

		// Step 2-2: prepare - copy chain data to the work directory
		copyProgress := utils.CopyProgress{Mu: sync.Mutex{}}
		if err := copyProgress.Progress(config.SrcMainnetDataPath, filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH)); err != nil {
			return err
		}

		// Step 3: run test
		if containerID, err := utils.RunMainnetUnwind(workDir, config); err != nil {
			return err
		} else {
			csvPath := filepath.Join(workDir, "unwind-container-stats.csv")
			ctx, cancel := context.WithCancel(context.Background())
			go monitorContainer(ctx, containerID, csvPath, false)
			utils.RunDockerWait(containerID, cancel, "")
			log.Println("The mainnnet data unwound successfully, now prepare for running replay")
		}

		if containerID, err := utils.RunMainnetReplay(workDir, config); err != nil {
			return err
		} else {
			csvPath := filepath.Join(workDir, "replay-container-stats.csv")
			ctx, cancel := context.WithCancel(context.Background())
			go monitorContainer(ctx, containerID, csvPath, true)
			utils.RunDockerWait(containerID, cancel, utils.REPLAY_STOP_SIGN)
		}

		// TODO: show the test result
		log.Printf("LRP test completed!")

		return nil
	},
}

func RootCommand() *cobra.Command {
	return rootCmd
}
