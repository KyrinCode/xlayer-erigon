# LRP (Local Replay) Command-Line Tool

## Overview

`lrp` (Local Replay) is a command-line tool designed for running local replay tests on blockchain data. It automates the process of checking out a target Git commit, preparing a working directory, copying chain data, and executing unwind and replay steps using Docker containers. This tool is intended for developers to test and debug blockchain nodes locally with specific batch ranges or configurations.

Use cases:
- Test specific blockchain node versions by checking out a Git commit or branch.
- Replay blockchain data locally to validate functionality or debug issues.
- Automate the setup and execution of blockchain replay tests.

## Prerequisites

Before using the `lrp` command, ensure the following requirements are met:
1. Go environment installed (to build and run the tool).
2. Docker installed (to run test containers).
3. Sufficient disk space (blockchain data can be large).
4. Access to the target Git repository (`erigon`) via `git`.

## Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd xlayer-erigon
   ```

2. Build the tool:
   ```bash
   # method 1
   make

   # method 2
   go install ./cmd/lrp
   ```

## Usage

### 1. Initialize Configuration

Before running tests, initialize the configuration file and RPC key:

```bash
lrp init
```

#### Functionality
- Generates a default `lrp.config.yaml` file in the current directory if it doesn't exist.
- Prompts for an RPC key and saves it to `rpc.key` (path specified by `--path`, default: `~/.lrp`).
- Skips steps if the file already exists, with a prompt.

#### Example
```bash
$ lrp init
Default config file generated at ./lrp.config.yaml
Enter your RPC key: <enter your RPC key>
RPC key saved to $HOME/.lrp/rpc.key
RPC key entered: <your L1 RPC key>
```

### 2. Add Batch Range

Define a batch range for testing using the following command:

```bash
lrp add
```

#### Functionality
- Prompts for `batchFrom`, `batchTo`, and a `description`.
- Saves the batch range to `options.json` (path specified by `--path`, default: `~/.lrp/options.json`).
- Saved batch ranges can be selected when running tests.

#### Example
```bash
$ lrp add
Enter batchFrom: 1000
Enter batchTo: 2000
Enter description: Test batch range for mainnet data
Batch range option added successfully!
```

### 3. Run Tests

Run a blockchain data replay test with the following command:

```bash
lrp -c <COMMIT_ID>
```

#### Functionality
The `lrp` command executes the test in the following steps:
1. **Environment Check**:
   - Validates command-line flags and environment setup.
   - Checks for running `lrp` test containers; if found, enters monitoring mode.
2. **Git Checkout**:
   - Switches to the specified Git branch or commit (via `--commitID` or config file).
3. **Prepare Working Directory**:
   - Creates a working directory (default: `~/.lrp/workspace`).
   - Copies blockchain (mainnet) data to the working directory.
4. **Run Test**:
   - If needed, performs an unwind step to roll back the blockchain data to the specified batch.
   - Executes the replay step to process the specified batch range.
   - Supports `vmtouch` mode (enabled with `--vmtouch`) for optimized data loading.
   - Optional: Backs up unwound chain data (enabled with `--backup`).
   - Optional: Monitors data directory size and fuses replay if the limit is exceeded (enabled with `--fuse`).
5. **Show Test Report**:
   - Displays a test report upon completion.

#### Command-Line Flags
| Flag          | Shorthand | Default                     | Description                                                                 |
|---------------|-----------|-----------------------------|-----------------------------------------------------------------------------|
| `--path`      | `-p`      | `~/.lrp`                    | Root directory for test data (repo and workspace, recommended to use default). |
| `--commitID`  | `-c`      | Empty                       | Git commit ID to checkout (overrides `GitCommit` in config).               |
| `--branch`    | `-b`      | `main`                      | Git branch to checkout (used if `commitID` is not set).                    |
| `--chaindata` |           | `~/.lrp/mainnet/seq`        | Directory containing blockchain data to import into the test environment.  |
| `--backup`    |           | `false`                     | Whether to back up unwound chain data.                                     |
| `--fuse`      |           | `false`                     | Monitor mainnet data directory size and fuse replay if size exceeds limit. |
| `--sample`    |           | `10s`                       | Sampling interval for Docker container, minimum 1 second.                  |
| `--vmtouch`   |           | `false`                     | Run replay container in `vmtouch` mode (optimizes data loading).           |

#### Example
```bash
$ lrp -c abc123def456
# Checks out commit abc123def456, sets up working directory, copies data, and runs the test
Batch Range Options (Select with Enter, Quit with q/Ctrl+C)
Batch 1000-2000 | Test batch range for mainnet data | Created: 2025-04-12 10:00:00 | Last Selected: Never
# Select a batch range to proceed
The mainnnet data unwound successfully, now prepare for running replay
The replay container is stopped, now prepared to show test result
LRP test completed!
Now is stopping and cleaning the containers, please wait for seconds...
```

#### Interrupting Tests
- Press `Ctrl+C` to interrupt the test; the tool will gracefully shut down and clean up resources.
- If a container is already running, `lrp` enters monitoring mode to display its status.

### 4. Select Batch Range
When running `lrp`, the tool presents an interactive interface listing all saved batch range options (from `options.json`). Navigate and select:
- Use `↑` and `↓` keys to navigate.
- Press `Enter` to select a batch range.
- Press `q` or `Ctrl+C` to exit selection.

The test will proceed with the selected batch range.

## File and Directory Structure

After running `lrp`, the following files and directories are created (assuming `--path` is the default `~/.lrp`):
- `~/.lrp/lrp.config.yaml`: Configuration file with default Git commit, data paths, etc.
- `~/.lrp/rpc.key`: Stores the user-provided RPC key.
- `~/.lrp/options.json`: Stores user-added batch range options.
- `~/.lrp/workspace/`: Working directory containing test data and logs.
  - `unwind-container-stats.csv`: Statistics for the unwind container.
  - `replay-container-stats.csv`: Statistics for the replay container.
- `~/.lrp/unwound/`: Backup of unwound data (if `--backup` is enabled).

## Notes

1. **Disk Space**: Blockchain data can take up hundreds of GB; ensure the target mount point has enough space.
2. **Permissions**: Ensure the current user has read/write permissions for the directory specified by `--path`.
3. **Docker Dependency**: Tests rely on Docker; ensure the Docker daemon is running.
4. **Interruption Handling**: Pressing `Ctrl+C` during a test will gracefully stop and clean up resources.
5. **Log Access**: Logs are saved in the working directory (`unwind-container-stats.csv` and `replay-container-stats.csv`).
