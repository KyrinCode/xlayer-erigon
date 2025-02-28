package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/ledgerwatch/erigon/cmd/lrp/utils"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lrp",
	Short: "LRP is the root command of running lrp test",
	Long:  `LRP is the root command of running lrp test`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		go func() {
			<-sigChan
			fmt.Println("Received Ctrl+C, shutting down...")
			cancel()
		}()

		// Step 0: Check flags and the environment
		if err := checkFlags(); err != nil {
			fmt.Printf("Checking flag variables return an error: %v\n", err)
			return
		}
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

		defer func() {
			utils.RunLRPStop(workDir)
			// utils.RunLRPClean(workDir)
		}()

		// Step 2-2: prepare - copy chain data to the work directory
		unwoundPath := utils.FindUnwoundDirectory(config.BatchFrom, path)
		if unwoundPath != "" {
			copyProgress := utils.CopyProgress{Title: "Unwound Mainnet Data Copy Progress", Mu: sync.Mutex{}}
			if err := copyProgress.Progress(unwoundPath, filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), true); err != nil {
				fmt.Printf("Received an error while copy unwound mainnet data from %s to %s: %v\n", config.SrcMainnetDataPath, filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), err)
				return
			}
		} else {
			copyProgress := utils.CopyProgress{Title: "Mainnet Data Copy Progress", Mu: sync.Mutex{}}
			if err := copyProgress.Progress(config.SrcMainnetDataPath, filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), true); err != nil {
				fmt.Printf("Received an error while copy mainnet data from %s to %s: %v\n", config.SrcMainnetDataPath, filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), err)
				return
			}
		}

		// Step 3: run test
		var needUnwind = false
		targetPath := filepath.Join(path, utils.UNWOUND_REPO, strconv.Itoa(int(config.BatchFrom)))
		if unwoundPath != targetPath {
			needUnwind = true
		}

		if needUnwind {
			unwindCSV := filepath.Join(workDir, "unwind-container-stats.csv")
			if containerID, err := utils.RunMainnetUnwind(workDir, config); err != nil {
				fmt.Printf("Running unwind step returns an error: %v\n", err)
				return
			} else {
				monitorCtx, monitorCancel := context.WithCancel(ctx)
				go monitorContainer(monitorCtx, containerID, unwindCSV, "", sampleIntv, false)
				utils.RunDockerWait(containerID, monitorCancel, "")
				if err := utils.WriteUnwindContainerLog(containerID, workDir); err != nil {
					fmt.Printf("Output unwind container log failed as: %v\n", err)
				}
				fmt.Println("The mainnnet data unwound successfully, now prepare for running replay")
			}
		}

		// backup the unwound chaindata if necessary
		if backupUnwound && needUnwind {
			copyProgress := utils.CopyProgress{Title: "Backing Up Unwound Mainnet Data Progress", Mu: sync.Mutex{}}
			go copyProgress.Progress(filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), targetPath, false)
			// if err := copyProgress.Progress(filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), targetPath, false); err != nil {
			// 	fmt.Printf("Received an error while backup unwound mainnet data from %s to %s: %v\n", config.SrcMainnetDataPath, filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), err)
			// 	return
			// }
		}

		replayCSV := filepath.Join(workDir, "replay-container-stats.csv")
		replayTPSCSV := filepath.Join(workDir, "replay-container-stats-tps.csv")
		if containerID, err := utils.RunMainnetReplay(workDir, config); err != nil {
			fmt.Printf("Running replay step returns an error: %v\n", err)
			return
		} else {
			monitorCtx, monitorCancel := context.WithCancel(ctx)
			go monitorContainer(monitorCtx, containerID, replayCSV, replayTPSCSV, sampleIntv, true)
			utils.RunDockerWait(containerID, monitorCancel, utils.REPLAY_STOP_SIGN)
			if err := utils.WriteReplayContainerLog(containerID, workDir); err != nil {
				fmt.Printf("Output replay container log failed as: %v\n", err)
			}
			fmt.Println("The replay container is stopped, now prepared to show test result")
		}

		// Step 4: show test report
		showReport(replayTPSCSV)
		fmt.Println("LRP test completed!")

		select {
		case <-ctx.Done():
			fmt.Println("Run function exiting due to context cancellation")
			return
		default:
		}
	},
}

func RootCommand() *cobra.Command {
	return rootCmd
}

func checkFlags() error {
	if sampleIntv < time.Second {
		return fmt.Errorf("the sampling interval is set too small, the minimum value is %d second", utils.MIN_SMAPLE_INTERVAL)
	}
	return nil
}
