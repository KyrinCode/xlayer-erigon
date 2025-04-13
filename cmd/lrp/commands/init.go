package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	testscripts "github.com/ledgerwatch/erigon/cmd/lrp/test-scripts"
	"github.com/ledgerwatch/erigon/cmd/lrp/utils"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a default configuration file and set the rpc key",
	Long:  "Generate a default lrp.config.yaml file in the current directory if it doesn't exist, and set the rpc key",
	Run: func(cmd *cobra.Command, args []string) {
		err := initializeConfigFile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "initializeConfigFile Error: %v\n", err)
			return
		}
		err = os.MkdirAll(path, 0755)
		if err != nil {
			fmt.Println("Error creating directory:", err)
			return
		}

		rpcKeyFile := filepath.Join(path, "rpc.key")
		if _, err := os.Stat(rpcKeyFile); err == nil {
			fmt.Println("rpc.key already exists at", rpcKeyFile, "- skipping RPC key setup.")
			return
		} else if !os.IsNotExist(err) {
			fmt.Println("Error checking rpc.key file:", err)
			return
		}

		fmt.Print("Enter your RPC key: ")
		rpcKey, err := readRpcKey()
		if err != nil {
			fmt.Println("Error reading RPC key:", err)
			return
		}

		err = os.WriteFile(rpcKeyFile, rpcKey, 0600)
		if err != nil {
			fmt.Println("Error saving RPC key to file:", err)
			return
		}

		fmt.Println("\nRPC key saved to", rpcKeyFile)
		fmt.Println("\nRPC key entered:", string(rpcKey))
	},
}

func initializeConfigFile() error {
	configPath := filepath.Join(".", utils.LRP_CONFIG_FILE)

	if _, err := os.Stat(configPath); err == nil {
		fmt.Println("Config file already exists at", configPath, "- skipping generation.")
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check config file: %v", err)
	}

	if err := utils.CreateFileIfNotExist(configPath, testscripts.XlayerLRPConfigExampleContent); err != nil {
		return fmt.Errorf("failed to read embedded config: %v", err)
	}

	fmt.Println("Default config file generated at", configPath)
	return nil
}

func readRpcKey() ([]byte, error) {
	fd := int(syscall.Stdin)
	key, err := term.ReadPassword(fd)
	if err != nil {
		return nil, err
	}
	return key, nil
}
