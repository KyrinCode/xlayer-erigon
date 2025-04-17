package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ledgerwatch/erigon/cmd/lrp/utils"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lrp",
	Short: "Run local replay tests for blockchain data",
	Long: `LRP (Local Replay) is a command-line tool for running local replay tests on blockchain data.
	It automates the process of checking out a target Git commit, preparing a working directory, copying chain data,
	and executing unwind and replay steps using Docker containers. This tool is designed for developers to replay blockchain
	data locally, allowing them to test and debug blockchain nodes with specific batch ranges or configurations.

	Before running this command, please run lrp init to initialize the configuration file and set the RPC key.
	
	Usage example:
	  lrp -c COMMIT_ID`,
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
		var (
			config   *utils.LRPConfig
			workDir  string
			commitID string
		)

		err := checkFlags()
		if err != nil {
			fmt.Printf("Checking flag variables return an error: %v", err)
			return
		}

		if config, err = utils.ParseLRPConfigFIle(); err != nil {
			fmt.Printf("Parsing LRP config file returns an error: %v", err)
			return
		}
		if config.WorkPath != "" {
			path = config.WorkPath
		}

		// Step 0.5: Check for running containers and monitor them
		if busy, runningContainer, _ := utils.IsLRPBusy(); busy {
			fmt.Println("There is currently running lrp tests")
			if runningContainer != "" {
				monitorCtx, monitorCancel := context.WithCancel(ctx)
				defer monitorCancel()
				isReplay := strings.Contains(strings.ToLower(runningContainer), "replay")
				csvFile := filepath.Join(path, fmt.Sprintf("%s-container-stats.csv", strings.Split(runningContainer, "-")[3]))
				go monitorContainer(monitorCtx, runningContainer, csvFile, sampleIntv, isReplay, 0)
				if _, err := utils.RunDockerWait(ctx, monitorCancel, runningContainer, ""); err != nil {
					fmt.Printf("Error monitoring running container %s: %v\n", runningContainer, err)
					return
				}
				fmt.Printf("Container %s has completed\n", runningContainer)
			}
			return
		}

		if err = utils.CheckEnviorment(path); err != nil {
			fmt.Printf("Checking enviornment returns an error: %v", err)
			return
		}

		// Step 1: checkout to the target branch or commit
		repoPath := filepath.Join(path, utils.REPO_NAME)
		// commitID flag has the high priority than config file
		if commitID == "" && config.GitCommit != "" {
			commitID = config.GitCommit
		}
		commitID, err = utils.CheckoutGitTarget(repoPath, branch, commitID)
		if err != nil {
			fmt.Printf("Can't checkout to %s as error: %v\n", commitID, err)
			return
		}
		config.GitCommit = commitID

		fmt.Println("comment file: ", filepath.Join(path, utils.COMMENT_OUT_FILE))
		utils.CommentOutLine(filepath.Join(path, utils.COMMENT_OUT_FILE), utils.COMMENT_TARGET_LINE)

		// Step 2-1: prepare - create a work directory
		selected, err := selectBatchRange(path)
		if err != nil {
			fmt.Printf("Got an error while selecting batch range: %v\n", err)
			return
		}
		if workDir, config, err = utils.SpawnWorkDirectoryByDefault(path, commitID, selected.BatchFrom, selected.BatchTo); err != nil {
			fmt.Printf("Creating work directory returns an error: %v\n", err)
			return
		}

		if !vmtouch {
			defer utils.RunLRPClean(workDir)
		}

		// Step 2-2: prepare - copy chain data to the work directory
		unwoundPath := utils.FindUnwoundDirectory(config.BatchFrom, path)
		if unwoundPath != "" {
			copyProgress := utils.CopyProgress{Title: "Unwound Mainnet Data Copy Progress", Mu: sync.Mutex{}}
			if err := copyProgress.Progress(filepath.Join(unwoundPath, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), workDir); err != nil {
				fmt.Printf("Received an error while copy unwound mainnet data from %s to %s: %v\n", config.SrcMainnetDataPath, filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), err)
				return
			}
		} else {
			copyProgress := utils.CopyProgress{Title: "Mainnet Data Copy Progress", Mu: sync.Mutex{}}
			if err := copyProgress.Progress(config.SrcMainnetDataPath, workDir); err != nil {
				fmt.Printf("Received an error while copy mainnet data from %s to %s: %v\n", config.SrcMainnetDataPath, filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), err)
				return
			}
		}

		// Step 3: run test
		var needUnwind = false
		backupTargetPath := filepath.Join(path, utils.UNWOUND_REPO, strconv.Itoa(int(config.BatchFrom)))
		if unwoundPath != backupTargetPath {
			needUnwind = true
		}

		if needUnwind {
			unwindCSV := filepath.Join(workDir, "unwind-container-stats.csv")
			if containerID, err := utils.RunMainnetUnwind(workDir, config); err != nil {
				fmt.Printf("Running unwind step returns an error: %v\n", err)
				return
			} else {
				monitorCtx, monitorCancel := context.WithCancel(ctx)
				go monitorContainer(monitorCtx, containerID, unwindCSV, sampleIntv, false, 0)
				if _, err := utils.RunDockerWait(ctx, monitorCancel, containerID, ""); err != nil {
					fmt.Printf("Receive an error during waiting for unwind container to complete execution: %v\n", err)
					return
				}
				if err := utils.WriteUnwindContainerLog(containerID, workDir); err != nil {
					fmt.Printf("Output unwind container log failed as: %v\n", err)
				}
				fmt.Println("The mainnnet data unwound successfully, now prepare for running replay")
			}
		}

		// backup the unwound chaindata if necessary
		if backupUnwound && needUnwind {
			if err := utils.RunMainnetDataCompact(workDir); err != nil {
				fmt.Printf("Received an error while compact mainnet data: %v\n", err)
				return
			}
			copyProgress := utils.CopyProgress{Title: "Backing Up Unwound Mainnet Data Progress", Mu: sync.Mutex{}}
			if err := copyProgress.Progress(filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), backupTargetPath); err != nil {
				fmt.Printf("Received an error while backup unwound mainnet data from %s to %s: %v\n", filepath.Join(workDir, utils.DEFAULT_SOURCE_MAINNET_DATA_PATH), backupTargetPath, err)
				return
			}
		}

		replayCSV := filepath.Join(workDir, "replay-container-stats.csv")
		var replayContainerID string
		if vmtouch {
			replayContainerID, err = utils.RunMainnetReplayVmtouch(workDir, config)
			if err != nil {
				fmt.Printf("Running replay step returns an error: %v\n", err)
				return
			}
		} else {
			replayContainerID, err = utils.RunMainnetReplay(workDir, config)
			if err != nil {
				fmt.Printf("Running replay step returns an error: %v\n", err)
				return
			}
		}

		if fuse && config.UseExternalDatastream {
			for {
				monitorCtx, monitorCancel := context.WithCancel(ctx)
				go monitorContainer(monitorCtx, replayContainerID, replayCSV, sampleIntv, true, config.BatchTo)
				go utils.MonitorChaindataSize(monitorCtx, workDir, utils.DEFAULT_CHAINDATA_LIMIT)
				exitCode, err := utils.RunDockerWait(ctx, monitorCancel, replayContainerID, "")
				if err != nil {
					fmt.Printf("Receive an error during waiting for replay container to complete execution: %v\n", err)
					return
				}
				if exitCode == 0 {
					break
				}
			}
		} else {
			monitorCtx, monitorCancel := context.WithCancel(ctx)
			go monitorContainer(monitorCtx, replayContainerID, replayCSV, sampleIntv, true, config.BatchTo)
			if _, err := utils.RunDockerWait(ctx, monitorCancel, replayContainerID, utils.REPLAY_STOP_SIGN); err != nil {
				fmt.Printf("Receive an error during waiting for replay container to complete execution: %v\n", err)
				return
			}
		}

		if err := utils.WriteReplayContainerLog(replayContainerID, workDir); err != nil {
			fmt.Printf("Output replay container log failed as: %v\n", err)
		}
		fmt.Println("The replay container is stopped, now prepared to show test result")

		// Step 4: show test report
		showReport(path, workDir, commitID)
		fmt.Println("LRP test completed!")
		fmt.Println("Now is stopping and cleaning the containers, please wait for seconds...")
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
