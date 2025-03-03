package utils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// dockerWaitAPI waits for the container to exit or stops when stopSign is found in logs
func dockerWaitAPI(containerID string, cancel context.CancelFunc, stopSign string) (int64, error) {
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return -1, fmt.Errorf("failed to create Docker client: %v", err)
	}
	defer cli.Close()

	ctx, logCancel := context.WithCancel(context.Background())
	defer logCancel()

	statusCh, errCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	// read container logs
	logReader, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return -1, fmt.Errorf("failed to get container logs: %v", err)
	}
	defer logReader.Close()

	if stopSign != "" {
		// run a goroutine to check logs
		go func() {
			scanner := bufio.NewScanner(logReader)
			for scanner.Scan() {
				line := scanner.Text()
				if len(line) > 8 {
					line = line[8:]
				}
				if strings.Contains(line, stopSign) {
					fmt.Printf("Stop sign '%s' found in logs, canceling wait\n", stopSign)
					logCancel()
					return
				}
			}
			if err := scanner.Err(); err != nil {
				fmt.Printf("Error reading logs: %v\n", err)
			}
		}()
	}

	select {
	case <-ctx.Done():
		return 0, nil
	case err := <-errCh:
		return -1, fmt.Errorf("error waiting for container: %v", err)
	case status := <-statusCh:
		return status.StatusCode, nil
	}
}

// dockerWait wraps dockerWaitAPI and handles the result
func dockerWait(containerID string, cancel context.CancelFunc, stopSign string) error {
	exitCode, err := dockerWaitAPI(containerID, cancel, stopSign)
	if err != nil {
		return fmt.Errorf("failed to execute docker wait %s: %w", containerID, err)
	}
	fmt.Printf("Container %s exited with code: %d\n", containerID, exitCode)
	return nil
}

func writeContainerLogs(containerID, outputFile string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %v", err)
	}
	defer cli.Close()

	// Open output file
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create log file %s: %v", outputFile, err)
	}
	defer file.Close()

	// Get container logs
	logsReader, err := cli.ContainerLogs(context.Background(), containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Tail:       "all",
	})
	if err != nil {
		return fmt.Errorf("failed to get logs for container %s: %v", containerID, err)
	}
	defer logsReader.Close()

	// Copy logs to file
	_, err = io.Copy(file, logsReader)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to write logs to %s: %v", outputFile, err)
	}
	return nil
}
