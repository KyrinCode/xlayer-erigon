package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	testscripts "github.com/ledgerwatch/erigon/cmd/lrp/test-scripts"
	"gopkg.in/yaml.v2"
)

func SpawnWorkDirectory(path, commitID string) (string, *LrpConfig, error) {
	timestamp := time.Now().Format("20060102-150405")
	workDir := filepath.Join(path, "workspace", fmt.Sprintf("%s-%s", timestamp, commitID))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", nil, fmt.Errorf("failed to create work folder: %v", err)
	}

	makefilePath := filepath.Join(workDir, "Makefile")
	dockerFilePath := filepath.Join(workDir, "docker-compose.yml")

	if err := createFileIfNotExist(makefilePath, testscripts.MakefileContent); err != nil {
		return "", nil, err
	}

	if err := createFileIfNotExist(dockerFilePath, testscripts.DockerComposeFileContent); err != nil {
		return "", nil, err
	}

	rpcKeyFile := filepath.Join(path, "rpc.key")
	rpcKey, err := os.ReadFile(rpcKeyFile)
	if err != nil {
		return "", nil, err
	}

	if err := createXlayerConfigFile(string(rpcKey), workDir); err != nil {
		return "", nil, err
	}

	// use 'make lrp-config' to generate new config file
	if err := runLrpConfig(workDir); err != nil {
		return "", nil, err
	}

	configPath := filepath.Join(workDir, LRP_CONFIG_FILE)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", nil, err
	}

	var config LrpConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", nil, fmt.Errorf("failed to read config file: %v", err)
	}
	config.GitCommit = commitID

	// override the config file
	data, _ = yaml.Marshal(&config)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", nil, fmt.Errorf("failed to overwrite config file: %v", err)
	}

	// rename work directory
	newWorkDir := filepath.Join(path, "workspace", fmt.Sprintf("%s-%s-%s-%d-%d", timestamp, commitID, config.User, config.BatchFrom, config.BatchTo))
	return newWorkDir, &config, os.Rename(workDir, newWorkDir)
}

func createFileIfNotExist(path string, content []byte) error {
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

func createXlayerConfigFile(rpcKey, path string) error {
	var config map[string]interface{}
	err := yaml.Unmarshal([]byte(testscripts.XlayerConfigMainnetContent), &config)
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

	configPath := filepath.Join(path, "xlayerconfig-mainnet.yaml")
	err = os.WriteFile(configPath, modifiedData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}
	return nil
}

func RunMainnetUnwind(dir string, config *LrpConfig) (string, error) {
	// use 'make lrp-mainnet-unwind' to make blockchain unwound to specfied height
	return runLrpMainnetUnwind(dir, config)
}

func RunMainnetReplay(dir string, config *LrpConfig) (string, error) {
	// use 'make lrp-mainnet-replay' to replay txs
	return runLrpMainnetReplay(dir, config)
}

func RunDockerWait(containerID string, cancel context.CancelFunc, stopSign string) error {
	return dockerWait(containerID, cancel, stopSign)
}
