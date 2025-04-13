package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	REPO_URL       = "https://github.com/okx/xlayer-erigon"
	DEFAULT_BRANCH = "dev"
)

func pullCode(path string) error {
	// If the destination directory does not exist, create it
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", path, err)
		}
	}

	// Check if the destination directory is empty or already a Git repository
	repoPath := filepath.Join(path, filepath.Base(REPO_URL))
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// Clone the repository
		cmd := exec.Command("git", "clone", REPO_URL, repoPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone repository: %v", err)
		}
	} else {
		fmt.Println("Repository already exists, skipping clone")
	}

	// Check for Git lock
	lockFile := filepath.Join(repoPath, ".git", "index.lock")
	if _, err := os.Stat(lockFile); err == nil {
		if err := os.Remove(lockFile); err != nil {
			return fmt.Errorf("failed to remove git lock file %s: %v", lockFile, err)
		}
		fmt.Println("Removed stale git lock file")
	}

	// Git reset
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get original directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(repoPath); err != nil {
		return fmt.Errorf("failed to change to directory %s: %v", repoPath, err)
	}
	cmd := exec.Command("git", "reset", "--hard")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to git reset repo: %v", err)
	}

	cmd = exec.Command("git", "fetch", "origin", "--prune")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch origin: %v", err)
	}

	return nil
}

func CheckoutGitTarget(path, branch, commitID string) (string, error) {
	// Check if it is a valid Git repository
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("directory %s is not a Git repository: %v", path, err)
	}

	// Determine the checkout target
	var target string
	if commitID != "" {
		// Check if the commitID exists
		exists, err := commitExists(path, commitID)
		if err != nil {
			return "", fmt.Errorf("failed to check if commit ID %s exists: %v", commitID, err)
		}
		if !exists {
			return "", fmt.Errorf("commit ID %s does not exist", commitID)
		}
		target = commitID
	} else if branch != "" {
		target = "origin/" + branch
	} else {
		return "", fmt.Errorf("either branch or commitID must be provided")
	}

	// Save the original directory and restore it after finishing
	originalDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get original directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Change to the target repository directory
	if err := os.Chdir(path); err != nil {
		return "", fmt.Errorf("failed to change to directory %s: %v", path, err)
	}

	// Execute git checkout
	cmd = exec.Command("git", "checkout", target)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to checkout %s: %v", target, err)
	}

	// Get the latest commit ID
	cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = path
	commitIDBytes, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get latest commit ID: %v", err)
	}

	commitID = strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-' {
			return r
		}
		return -1
	}, string(commitIDBytes))

	return commitID, nil
}

func commitExists(repoDir, commitID string) (bool, error) {
	cmd := exec.Command("git", "cat-file", "-t", commitID)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}
