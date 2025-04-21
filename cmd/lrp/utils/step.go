package utils

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	testscripts "github.com/ledgerwatch/erigon/cmd/lrp/test-scripts"
	"gopkg.in/yaml.v2"
)

func SpawnWorkDirectoryByDefault(path, commitID string, batchFrom, batchTo int) (string, *LRPConfig, error) {
	timestamp := time.Now().Format("20060102-150405")
	portDiff, _ := getFreePortDiff()

	rawConfigPath := filepath.Join(".", LRP_CONFIG_FILE)
	data, err := os.ReadFile(rawConfigPath)
	if err != nil {
		fmt.Printf("Failed to read config file: %v\n", err)
		return "", nil, err
	}

	var rawConfig LRPConfig
	err = yaml.Unmarshal(data, &rawConfig)
	if err != nil {
		fmt.Printf("Failed to unmarshal config file: %v\n", err)
		return "", nil, err
	}

	config := &LRPConfig{
		User:                   rawConfig.User,
		RpcKey:                 rawConfig.RpcKey,
		GitCommit:              commitID,
		PortDiff:               int64(portDiff),
		BatchFrom:              uint64(batchFrom),
		BatchTo:                uint64(batchTo),
		UseExternalDatastream:  rawConfig.UseExternalDatastream,
		ExternalDataStreamPath: rawConfig.ExternalDataStreamPath,
		SrcMainnetDataPath:     rawConfig.SrcMainnetDataPath,
		ProcessCount:           rawConfig.ProcessCount,
	}

	workDir := filepath.Join(path, "workspace", fmt.Sprintf("%s-%s-%s-%d-%d", timestamp, commitID, config.User, config.BatchFrom, config.BatchTo))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", nil, fmt.Errorf("failed to create work folder: %v", err)
	}

	makefilePath := filepath.Join(workDir, "Makefile")
	dockerFilePath := filepath.Join(workDir, "docker-compose.yml")

	if err := CreateFileIfNotExist(makefilePath, testscripts.MakefileContent); err != nil {
		return "", nil, err
	}

	if err := CreateFileIfNotExist(dockerFilePath, testscripts.DockerComposeFileContent); err != nil {
		return "", nil, err
	}

	if err := createXlayerConfigFile(config.RpcKey, path, workDir); err != nil {
		return "", nil, err
	}

	configPath := filepath.Join(workDir, LRP_CONFIG_FILE)
	data, _ = yaml.Marshal(&config)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", nil, fmt.Errorf("failed to write config file: %v", err)
	}

	return workDir, config, nil
}

func CreateFileIfNotExist(path string, content []byte) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.WriteFile(path, content, 0644)
		if err != nil {
			return err
		}
		fmt.Println("Generated file at", path)
	} else if err != nil {
		return err
	} else {
		fmt.Println("File already exists at", path, "using it directly")
	}
	return nil
}

func createXlayerConfigFile(rpcKey, path, workDir string) error {
	// Define the destination file path
	configPath := filepath.Join(workDir, "xlayerconfig-mainnet.yaml")

	// Copy the source file to the destination
	sourceData, err := os.ReadFile(filepath.Join(path, LRP_MAINNET_CONFIG_FILE))
	if err != nil {
		return fmt.Errorf("failed to read source file: %v", err)
	}

	// Write the source data to the destination file
	err = os.WriteFile(configPath, sourceData, 0644)
	if err != nil {
		return fmt.Errorf("failed to copy file to %s: %v", configPath, err)
	}

	var config map[string]interface{}

	err = yaml.Unmarshal(sourceData, &config)
	if err != nil {
		return fmt.Errorf("failed to unmarshal config file: %v", err)
	}

	if url, ok := config["zkevm.l1-rpc-url"].(string); ok {
		config["zkevm.l1-rpc-url"] = url + "/" + rpcKey
	}

	modifiedData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %v", err)
	}

	err = os.WriteFile(configPath, modifiedData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}
	return nil
}

func getFreePortDiff() (int, error) {
	basePorts := []int{
		6060,  // PPROF_PORT
		8545,  // RPC_PORT
		6900,  // DS_PORT
		9095,  // METRICS_PORT
		16900, // EXTERNAL_DS_PORT
	}

	for portDiff := 0; portDiff <= 65535; portDiff++ {
		allFree := true
		for _, base := range basePorts {
			port := base + portDiff
			if port > 65535 {
				return -1, fmt.Errorf("port %d exceeds maximum port number 65535", port)
			}
			// Check if the port is free by attempting to listen on it
			addr := fmt.Sprintf(":%d", port)
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				// Port is in use
				allFree = false
				break
			}
			// Close the listener immediately since we only need to test availability
			listener.Close()
		}
		if allFree {
			return portDiff, nil
		}
	}
	return -1, fmt.Errorf("no PORT_DIFF found with all five ports free")
}

func RunMainnetUnwind(workDir string, config *LRPConfig) (string, error) {
	// use 'make lrp-mainnet-unwind' to make blockchain unwound to specfied height
	return runLRPMainnetUnwind(workDir, config)
}

func RunMainnetReplay(workDir string, config *LRPConfig) (string, error) {
	// use 'make lrp-mainnet-replay' to replay txs
	return runLRPMainnetReplay(workDir, config)
}

func RunMainnetDataCompact(workDir string) error {
	// use 'make lrp-mainnet-data-compact' to compact mainnet data
	return runLRPMainnetDataCompact(workDir)
}

func RunMainnetReplayVmtouch(workDir string, config *LRPConfig) (string, error) {
	// use 'make lrp-mainnet-replay-vmtouch' to replay txs
	return runLRPMainnetReplayVmtouch(workDir, config)
}

func RunDockerWait(ctx context.Context, cancel context.CancelFunc, containerID string) (int64, error) {
	return dockerWait(ctx, cancel, containerID)
}

func RunLRPStop(workDir string) error {
	// use 'make lrp-stop' to stop the container
	return runLRPStop(workDir)
}

func RunLRPClean(workDir string) error {
	// use 'make lrp-clean' to stop the container and clean the mainnet chaindata
	return runLRPClean(workDir)
}

func WriteUnwindContainerLog(containerID, workDir string) error {
	return writeContainerLogs(containerID, filepath.Join(workDir, UNWIND_LOG))
}

func WriteReplayContainerLog(containerID, workDir string) error {
	return writeContainerLogs(containerID, filepath.Join(workDir, REPLAY_LOG))
}

func IsLRPBusy() (bool, string, error) {
	return isLRPBusy()
}

func FindUnwoundDirectory(batchFrom uint64, path string) string {
	if _, err := os.Stat(path); err != nil {
		return ""
	}

	unwoundRepo := filepath.Join(path, UNWOUND_REPO)
	entries, err := os.ReadDir(unwoundRepo)
	if err != nil {
		return ""
	}

	closest := -1
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		folderNum, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		if folderNum >= int(batchFrom) {
			if closest == -1 || folderNum < closest {
				closest = folderNum
			}
		}
	}

	if closest == -1 {
		return ""
	}
	return filepath.Join(unwoundRepo, strconv.Itoa(closest))
}

func MonitorChaindataSize(ctx context.Context, workDir string, sizeLimit int64) {
	// current setting is 1 minute interval, there is no need to make it too short
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	chaindataDir := filepath.Join(workDir, DEFAULT_SOURCE_MAINNET_DATA_PATH)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			size, err := getFolderSize(chaindataDir)
			if err != nil {
				fmt.Printf("Error checking folder size: %v", err)
				continue
			}

			if size >= sizeLimit {
				fmt.Printf("Folder size %d bytes exceeds limit %d bytes, pausing replay...", size, sizeLimit)

				// use `make lrp-mainnet-replay-pause` to pasue the replay container
				runLRPMainnetReplayPause(workDir)
				return
			}
		}
	}
}

func getFolderSize(chaindataDir string) (int64, error) {
	var size int64
	err := filepath.Walk(chaindataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to calculate size of %s: %v", chaindataDir, err)
	}
	return size, nil
}

func CommentOutLine(filePath, targetLine string) error {
	// Open the file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Store modified lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Comment out the target line
		if strings.TrimSpace(line) == targetLine {
			line = "// " + line
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Write back to the file
	err = os.WriteFile(filePath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}
