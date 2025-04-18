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
		fmt.Print("Enter your RPC key: ")
		rpcKey, err := readRpcKey()
		if err != nil {
			fmt.Println("Error reading RPC key:", err)
			return
		}
		fmt.Println("\nRPC key entered:", string(rpcKey))

		err = initializeConfigFile(string(rpcKey))
		if err != nil {
			fmt.Fprintf(os.Stderr, "initializeConfigFile Error: %v\n", err)
			return
		}
		fmt.Println("Config file initialized successfully.")
	},
}

func initializeConfigFile(rpcKey string) error {
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

	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("\nrpcKey: %s\n", rpcKey))

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
