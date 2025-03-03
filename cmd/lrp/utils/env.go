package utils

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	testscripts "github.com/ledgerwatch/erigon/cmd/lrp/test-scripts"
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
	User                   string `yaml:"user"`
	GitCommit              string `yaml:"gitCommit"`
	PortDiff               int64  `yaml:"portDiff"`
	BatchFrom              uint64 `yaml:"fromBatchNumber"`
	BatchTo                uint64 `yaml:"toBatchNumber"`
	UseExternalDatastream  bool   `yaml:"useExternalDatastream"`
	ExternalDataStreamPath string `yaml:"externalDatastreamPath"`
	SrcMainnetDataPath     string `yaml:"srcMainnetDataPath"`
}

func (c *LRPConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawConfig struct {
		User                   string `yaml:"user"`
		GitCommit              string `yaml:"gitCommit"`
		PortDiff               int64  `yaml:"portDiff"`
		BatchFrom              uint64 `yaml:"fromBatchNumber"`
		BatchTo                uint64 `yaml:"toBatchNumber"`
		UseExternalDatastream  bool   `yaml:"useExternalDatastream"`
		ExternalDataStreamPath string `yaml:"externalDatastreamPath"`
		SrcMainnetDataPath     string `yaml:"srcMainnetDataPath"`
	}

	var raw rawConfig
	if err := unmarshal(&raw); err != nil {
		return err
	}

	c.SrcMainnetDataPath = raw.SrcMainnetDataPath
	if c.SrcMainnetDataPath == "" {
		c.SrcMainnetDataPath = filepath.Join(GetDefaultPath(""), DEFAULT_SOURCE_MAINNET_DATA_PATH)
	}

	if raw.ExternalDataStreamPath == "" {
		c.ExternalDataStreamPath = filepath.Join(GetDefaultPath(""), DEFAULT_EXTERNAL_DATASTREAM_PATH)
	}

	c.User = raw.User
	c.PortDiff = raw.PortDiff
	c.BatchFrom = raw.BatchFrom
	c.BatchTo = raw.BatchTo
	c.UseExternalDatastream = raw.UseExternalDatastream

	return nil
}

func getHomeDir(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return home
}

func GetDefaultPath(path string) string {
	return filepath.Join(getHomeDir(path), DEFAULT_DESTINATION_DIR)
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
		fmt.Println("rpc.key is not set, please run 'lrp setkey -h' for help")
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
	if err := createFileIfNotExist(dockerfileLocalPath, testscripts.DockerfileLRPContent); err != nil {
		return err
	}

	fmt.Println("Check environment done")
	return nil
}

func copyDependencies(repoPath, destPath string) error {
	err := os.MkdirAll(destPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %v", err)
	}

	for _, file := range dependencies {
		srcFile := filepath.Join(repoPath, file)
		destFile := filepath.Join(destPath, file)

		if _, err := os.Stat(srcFile); os.IsNotExist(err) {
			fmt.Printf("Warning: %s does not exist in %s, skipping\n", file, repoPath)
			continue
		} else if err != nil {
			return fmt.Errorf("error checking %s: %v", srcFile, err)
		}

		err = os.MkdirAll(filepath.Dir(destFile), 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory for %s: %v", destFile, err)
		}

		src, err := os.Open(srcFile)
		if err != nil {
			return fmt.Errorf("failed to open source file %s: %v", srcFile, err)
		}
		defer src.Close()

		dest, err := os.Create(destFile)
		if err != nil {
			return fmt.Errorf("failed to create destination file %s: %v", destFile, err)
		}
		defer src.Close()

		_, err = io.Copy(dest, src)
		if err != nil {
			return fmt.Errorf("failed to copy %s to %s: %v", srcFile, destFile, err)
		}

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
func isLRPBusy() (bool, error) {
	// Check running Docker containers
	hasRunningContainers, err := checkRunningContainers()
	if err != nil {
		return false, fmt.Errorf("failed to check Docker containers: %v", err)
	}
	if hasRunningContainers {
		return true, nil
	}

	// Check active lrp commands, excluding the current process
	hasActiveLRP, err := checkActiveLRPCommands()
	if err != nil {
		return false, fmt.Errorf("failed to check lrp commands: %v", err)
	}
	if hasActiveLRP {
		return true, nil
	}

	return false, nil
}

// checkRunningContainers checks if there are running Docker containers
func checkRunningContainers() (bool, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return false, err
	}

	containers, err := cli.ContainerList(context.Background(), container.ListOptions{
		All: false, // Only list running containers
	})
	if err != nil {
		return false, err
	}

	for _, container := range containers {
		for _, name := range container.Names {
			if strings.Contains(strings.ToLower(name), "unwind") {
				return true, nil
			}
			if strings.Contains(strings.ToLower(name), "replay") {
				return true, nil
			}
		}
	}

	return false, nil
}

// checkActiveLRPCommands checks if there are any active lrp commands excluding the current process
func checkActiveLRPCommands() (bool, error) {
	// Get current process ID
	currentPID := os.Getpid()

	// Use `ps` command to list processes
	cmd := exec.Command("ps", "-eo", "pid,cmd")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to execute ps command: %v", err)
	}

	// Split output into lines
	lines := strings.Split(string(output), "\n")
	lrpCount := 0

	// Look for processes with "lrp" in the command name
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		pidStr := fields[0]
		cmdLine := strings.Join(fields[1:], " ")

		// Check if the command contains "lrp"
		if strings.Contains(cmdLine, "lrp") {
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}
			// Exclude the current process
			if pid != currentPID {
				lrpCount++
			}
		}
	}

	return lrpCount > 0, nil
}
