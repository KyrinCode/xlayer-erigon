package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func runLrpConfig(dir string) error {
	cmd := exec.Command("make", "-C", dir, LRP_CONFIG)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute make lrp-config: %w", err)
	}
	return nil
}

func runLrpMainnetUnwind(dir string, config *LrpConfig) (string, error) {
	cmd := exec.Command("make", "-C", dir, LRP_MAINNET_UNWIND)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to execute make lrp-mainnet-unwind: %v", err)
	}

	containerID := fmt.Sprintf("%s-xlayer-mainnet-unwind", config.User)
	if containerID == "" {
		return "", fmt.Errorf("no container ID constructed")
	}

	return containerID, nil
}

func runLrpMainnetReplay(dir string, config *LrpConfig) (string, error) {
	cmd := exec.Command("make", "-C", dir, LRP_MAINNET_REPLAY)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute make lrp-mainnet-replay: %w", err)
	}

	containerID := fmt.Sprintf("%s-xlayer-mainnet-replay", config.User)
	if containerID == "" {
		return "", fmt.Errorf("no container ID constructed")
	}

	return containerID, nil
}
