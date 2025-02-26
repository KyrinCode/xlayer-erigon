package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var KeyCmd = &cobra.Command{
	Use:   "setkey",
	Short: "Set the RPC key",
	Run: func(cmd *cobra.Command, args []string) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			fmt.Println("Error creating directory:", err)
			return
		}

		rpcKeyFile := filepath.Join(path, "rpc.key")

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

func readRpcKey() ([]byte, error) {
	fd := int(syscall.Stdin)
	key, err := term.ReadPassword(fd)
	if err != nil {
		return nil, err
	}
	return key, nil
}
