package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/ledgerwatch/erigon/cmd/lrp/utils"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lrp",
	Short: "LRP is the root command of running lrp test",
	Long:  `LRP is the root command of running lrp test`,
	Run: func(cmd *cobra.Command, args []string) {
		// Step 0: Check the environment
		if err := utils.CheckEnviorment(path); err != nil {
			fmt.Printf("Checking enviornment returns an error: %v\n", err)
			return
		}

		if !ignoreRunning {
			if busy, _ := utils.IsLRPBusy(); busy {
				fmt.Println("There are currently running lrp tests")
				return
			}
		}

		// Step 1: checkout to the target branch or commit
		repoPath := filepath.Join(path, utils.REPO_NAME)
		commitID, err := utils.CheckoutGitTarget(repoPath, branch, commitID)
		if err != nil {
			fmt.Printf("Can't checkout to %s as error: %v\n", commitID, err)
			return
		}

		// Step 2-1: prepare - create a work directory
		workDir, config, err := utils.SpawnWorkDirectory(path, commitID)
		if err != nil {
			fmt.Printf("Creating work directory returns an error: %v\n", err)
			return
		}

		// Step 2-2: prepare - copy chain data to the work directory
		copyProgress := utils.CopyProgress{Mu: sync.Mutex{}}
		if err := copyProgress.Progress(config.SrcMainnetDataPath, filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH)); err != nil {
			fmt.Printf("Received an error while copy mainnet data from  %s to %s: %v\n", config.SrcMainnetDataPath, filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), err)
			return
		}

		// Step 3: run test
		unwindCSV := filepath.Join(workDir, "unwind-container-stats.csv")
		if containerID, err := utils.RunMainnetUnwind(workDir, config); err != nil {
			fmt.Printf("Running unwind step returns an error: %v\n", err)
			return
		} else {
			ctx, cancel := context.WithCancel(context.Background())
			go monitorContainer(ctx, containerID, unwindCSV, false)
			utils.RunDockerWait(containerID, cancel, "")

			if err := utils.WriteUnwindContainerLog(containerID, workDir); err != nil {
				fmt.Printf("Output unwind container log failed as: %v\n", err)
			}
			fmt.Println("The mainnnet data unwound successfully, now prepare for running replay")
		}

		replayCSV := filepath.Join(workDir, "replay-container-stats.csv")
		if containerID, err := utils.RunMainnetReplay(workDir, config); err != nil {
			fmt.Printf("Running replay step returns an error: %v\n", err)
			return
		} else {
			ctx, cancel := context.WithCancel(context.Background())
			go monitorContainer(ctx, containerID, replayCSV, true)
			utils.RunDockerWait(containerID, cancel, utils.REPLAY_STOP_SIGN)

			// turn off the replay container and output logs
			if err := utils.WriteReplayContainerLog(containerID, workDir); err != nil {
				fmt.Printf("Output replay container log failed as: %v\n", err)
			}
			utils.RunLRPStop(workDir)
			fmt.Println("The replay container is stopped, now prepared to show test result")
		}

		// Step 4: show test report
		showReport(replayCSV)
		fmt.Println("LRP test completed!")
	},
}

func RootCommand() *cobra.Command {
	return rootCmd
}
