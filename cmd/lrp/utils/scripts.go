package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func runLRPConfig(dir string) error {
	cmd := exec.Command("make", "-C", dir, LRP_CONFIG)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute make lrp-config: %w", err)
	}
	return nil
}

func runLRPMainnetUnwind(dir string, config *LRPConfig) (string, error) {
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

func runLRPMainnetReplay(dir string, config *LRPConfig) (string, error) {
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

func runLRPMainnetDataCompact(dir string) error {
	cmd := exec.Command("make", "-C", dir, LRP_MAINNET_DATA_COMPACT)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute make lrp-mainnet-data-compact: %w", err)
	}

	return nil
}

func runLRPMainnetReplayVmtouch(dir string, config *LRPConfig) (string, error) {
	cmd := exec.Command("make", "-C", dir, LRP_MAINNET_REPLAY_VMTOUCH)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute make lrp-mainnet-replay-vmtouch: %w", err)
	}

	containerID := fmt.Sprintf("%s-xlayer-mainnet-replay", config.User)
	if containerID == "" {
		return "", fmt.Errorf("no container ID constructed")
	}

	return containerID, nil
}

func runLRPStop(dir string) error {
	cmd := exec.Command("make", "-C", dir, LRP_STOP)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute make lrp-stop: %w", err)
	}
	return nil
}

func runLRPClean(dir string) error {
	cmd := exec.Command("make", "-C", dir, LRP_CLEAN)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute make lrp-clean: %w", err)
	}
	return nil
}

func runLRPMainnetReplayPause(dir string) error {
	cmd := exec.Command("make", "-C", dir, LRP_MAINNET_REPLAY_PAUSE)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute make lrp-mainnet-replay-pause: %w", err)
	}
	return nil
}
