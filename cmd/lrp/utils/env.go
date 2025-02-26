package utils

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

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

type LrpConfig struct {
	User                   string `yaml:"user"`
	GitCommit              string `yaml:"gitCommit"`
	PortDiff               int64  `yaml:"portDiff"`
	BatchFrom              uint64 `yaml:"fromBatchNumber"`
	BatchTo                uint64 `yaml:"toBatchNumber"`
	UseExternalDatastream  bool   `yaml:"useExternalDatastream"`
	ExternalDataStreamPath string `yaml:"externalDatastreamPath"`
	SrcMainnetDataPath     string `yaml:"srcMainnetDataPath"`
}

func (c *LrpConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
		// c.SrcMainnetDataPath = filepath.Join(GetDefaultPath(""), DEFAULT_SOURCE_MAINNET_DATA_PATH)
		c.SrcMainnetDataPath = "/Users/oker/Downloads/mainnet/seq"
	}

	if raw.ExternalDataStreamPath == "" {
		// c.ExternalDataStreamPath = filepath.Join(GetDefaultPath(""), DEFAULT_SOURCE_MAINNET_DATA_PATH)
		c.ExternalDataStreamPath = "/Users/oker/Downloads/mainnet/seq/data-stream"
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
	if err := createFileIfNotExist(dockerfileLocalPath, testscripts.DockerfileLocalContent); err != nil {
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
