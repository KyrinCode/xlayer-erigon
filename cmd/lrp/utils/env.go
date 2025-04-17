package utils

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	testscripts "github.com/ledgerwatch/erigon/cmd/lrp/test-scripts"
	"gopkg.in/yaml.v2"
)

var dependencies = []string{
	".dockerignore",
	"go.mod",
	"go.sum",
	"erigon-lib/go.mod",
	"erigon-lib/go.sum",
	"tools.go",
	"Makefile",
}

type LRPConfig struct {
	WorkPath               string `yaml:"workPath"`
	User                   string `yaml:"user"`
	GitCommit              string `yaml:"gitCommit"`
	PortDiff               int64  `yaml:"portDiff"`
	BatchFrom              uint64 `yaml:"fromBatchNumber"`
	BatchTo                uint64 `yaml:"toBatchNumber"`
	UseExternalDatastream  bool   `yaml:"useExternalDatastream"`
	ExternalDataStreamPath string `yaml:"externalDatastreamPath,omitempty"`
	SrcMainnetDataPath     string `yaml:"srcMainnetDataPath"`
	ProcessCount           int    `yaml:"processCount"`
}

func (c *LRPConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawConfig struct {
		WorkPath               string `yaml:"workPath"`
		User                   string `yaml:"user"`
		GitCommit              string `yaml:"gitCommit"`
		PortDiff               int64  `yaml:"portDiff"`
		BatchFrom              uint64 `yaml:"fromBatchNumber"`
		BatchTo                uint64 `yaml:"toBatchNumber"`
		UseExternalDatastream  bool   `yaml:"useExternalDatastream"`
		ExternalDataStreamPath string `yaml:"externalDatastreamPath,omitempty"`
		SrcMainnetDataPath     string `yaml:"srcMainnetDataPath"`
		ProcessCount           int    `yaml:"processCount"`
	}

	var raw rawConfig
	if err := unmarshal(&raw); err != nil {
		return err
	}

	c.WorkPath = raw.WorkPath
	c.User = raw.User
	c.PortDiff = raw.PortDiff
	c.BatchFrom = raw.BatchFrom
	c.BatchTo = raw.BatchTo
	c.UseExternalDatastream = raw.UseExternalDatastream
	c.ProcessCount = raw.ProcessCount
	c.GitCommit = raw.GitCommit

	if raw.SrcMainnetDataPath != "" {
		c.SrcMainnetDataPath = raw.SrcMainnetDataPath
	}

	if raw.ExternalDataStreamPath != "" {
		c.ExternalDataStreamPath = raw.ExternalDataStreamPath
	}

	return nil
}

func ParseLRPConfigFIle() (*LRPConfig, error) {
	var config *LRPConfig

	// Get the content of lrp.config.yaml file if it exists
	configPath := filepath.Join(".", LRP_CONFIG_FILE)
	if data, err := os.ReadFile(configPath); err != nil {
		fmt.Println("lrp.config.yaml is not initialized, please run 'lrp init -h' for help")
		return nil, err
	} else {
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal lrp.config.yaml: %v", err)
		}
	}
	return config, nil

}

// CheckEnviorment checks the environment for required tools and files
func CheckEnviorment(path string) error {
	// Check if git is installed
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed")
	}

	// Check if docker is installed
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is not installed")
	}

	// Check if Docker daemon is running
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("docker daemon is not running: %v", err)
	}
	defer cli.Close()

	// Test connection to Docker daemon with a simple ping
	_, err = cli.Ping(context.Background())
	if err != nil {
		return fmt.Errorf("failed to connect to docker daemon, ensure it is running: %v", err)
	}
	cli.Close() // Close the client after checking

	// Check if rpc.key exists
	rpcKeyPath := filepath.Join(path, "rpc.key")
	if _, err := os.Stat(rpcKeyPath); err != nil {
		fmt.Println("rpc.key is not set, please run 'lrp init -h' for help")
		return err
	}

	// Fetch repo
	if err := pullCode(path); err != nil {
		return err
	}

	// Check if dependended files exist, if not, copy them from repo
	repoPath := filepath.Join(path, REPO_NAME)
	if err := copyDependencies(repoPath, path); err != nil {
		return err
	}

	// Check if Dockerfile.local exists, if not, create it
	dockerfileLocalPath := filepath.Join(path, "Dockerfile.local")
	if err := CreateFileIfNotExist(dockerfileLocalPath, testscripts.DockerfileLocalContent); err != nil {
		return err
	}

	fmt.Println("Check environment done")
	return nil
}

// copyDependencies copies dependency files from repoPath to destPath, overwriting existing files.
// It ensures all specified dependencies are copied, skipping only if the source file is missing.
func copyDependencies(repoPath, destPath string) error {
	// Create the destination directory if it doesn't exist
	err := os.MkdirAll(destPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %v", err)
	}

	// Iterate over the list of dependency files
	for _, file := range dependencies {
		srcFile := filepath.Join(repoPath, file)
		destFile := filepath.Join(destPath, file)

		// Check if the source file exists; skip with a warning if it doesn't
		if _, err := os.Stat(srcFile); os.IsNotExist(err) {
			fmt.Printf("Warning: %s does not exist in %s, skipping\n", file, repoPath)
			continue
		} else if err != nil {
			return fmt.Errorf("error checking %s: %v", srcFile, err)
		}

		// Ensure the destination directory structure exists
		err = os.MkdirAll(filepath.Dir(destFile), 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory for %s: %v", destFile, err)
		}

		// Open the source file for reading
		src, err := os.Open(srcFile)
		if err != nil {
			return fmt.Errorf("failed to open source file %s: %v", srcFile, err)
		}
		defer src.Close()

		// Create (or overwrite) the destination file
		dest, err := os.Create(destFile)
		if err != nil {
			return fmt.Errorf("failed to create destination file %s: %v", destFile, err)
		}
		defer dest.Close() // Note: Fixed defer to close dest, not src again

		// Copy the contents from source to destination, overwriting any existing file
		_, err = io.Copy(dest, src)
		if err != nil {
			return fmt.Errorf("failed to copy %s to %s: %v", srcFile, destFile, err)
		}

		// Sync the destination file to ensure the write is complete
		err = dest.Sync()
		if err != nil {
			return fmt.Errorf("failed to sync %s: %v", destFile, err)
		}

		fmt.Printf("Copied %s to %s\n", srcFile, destFile)
	}

	return nil
}

// isLRPBusy checks if there are running Docker containers or active lrp commands
// Returns true if either condition is met
func isLRPBusy() (bool, string, error) {
	// Check running Docker containers
	runningContainer, err := getRunningContainers()
	if err != nil {
		return false, "", fmt.Errorf("failed to check Docker containers: %v", err)
	}
	if runningContainer != "" {
		return true, runningContainer, nil
	}

	// Check active lrp commands, excluding the current process
	hasActiveLRP, err := checkActiveLRPCommands()
	if err != nil {
		return false, "", fmt.Errorf("failed to check lrp commands: %v", err)
	}
	if hasActiveLRP {
		return true, "", nil
	}

	return false, "", nil
}

// getRunningContainers gets container id if there are running Docker containers
func getRunningContainers() (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}

	containers, err := cli.ContainerList(context.Background(), container.ListOptions{
		All: false, // Only list running containers
	})
	if err != nil {
		return "", err
	}

	for _, container := range containers {
		for _, name := range container.Names {
			if strings.Contains(strings.ToLower(name), "unwind") {
				return name, nil
			}
			if strings.Contains(strings.ToLower(name), "replay") {
				return name, nil
			}
		}
	}

	return "", nil
}

// checkActiveLRPCommands checks if there are any active lrp commands excluding the current process
func checkActiveLRPCommands() (bool, error) {
	// Use `ps aux` to list all processes with detailed command information
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to execute ps command: %v", err)
	}

	// Split output into lines
	lines := strings.Split(string(output), "\n")
	lrpCount := 0

	// Skip the header line and look for processes with "lrp" in the command
	for i, line := range lines {
		if i == 0 {
			continue // Skip header
		}
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue // Ensure enough fields are present
		}

		cmdLine := strings.Join(fields[10:], " ") // Command is from the 11th field onward

		// Check if the command contains "lrp" (case-insensitive)
		if strings.Contains(strings.ToLower(cmdLine), "lrp") {
			if err != nil {
				continue
			}
			lrpCount++
		}
	}

	return lrpCount > 1, nil
}
