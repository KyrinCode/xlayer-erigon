#!/bin/bash
set -e

# Detect OS type
OS_TYPE=$(uname -s)
echo "Detected OS: $OS_TYPE"

# Default values - only set if not already defined in environment
: ${DA_MODE:="cdk-validium"}
: ${AC_SPLIT:="default"}
: ${LOGS_DIR:="ci_logs"}
: ${LOG_NAME:="evm-rpc-tests-logs"}

echo "Initial configuration:"
echo "DA_MODE=$DA_MODE"
echo "AC_SPLIT=$AC_SPLIT"
echo "LOGS_DIR=$LOGS_DIR"
echo "LOG_NAME=$LOG_NAME"

# Parse command line arguments - only override if not already set via environment
while [[ $# -gt 0 ]]; do
  case $1 in
    --da-mode)
      if [ -z "${DA_MODE_SET}" ]; then
        DA_MODE="$2"
        DA_MODE_SET=1
        echo "Setting DA_MODE=$DA_MODE from command line"
      else
        echo "Ignoring --da-mode flag as DA_MODE is already set to $DA_MODE"
      fi
      shift 2
      ;;
    --ac-split)
      if [ -z "${AC_SPLIT_SET}" ]; then
        AC_SPLIT="$2"
        AC_SPLIT_SET=1
        echo "Setting AC_SPLIT=$AC_SPLIT from command line"
      else
        echo "Ignoring --ac-split flag as AC_SPLIT is already set to $AC_SPLIT"
      fi
      shift 2
      ;;
    --logs-dir)
      if [ -z "${LOGS_DIR_SET}" ]; then
        LOGS_DIR="$2"
        LOGS_DIR_SET=1
        echo "Setting LOGS_DIR=$LOGS_DIR from command line"
      else
        echo "Ignoring --logs-dir flag as LOGS_DIR is already set to $LOGS_DIR"
      fi
      shift 2
      ;;
    --log-name)
      if [ -z "${LOG_NAME_SET}" ]; then
        LOG_NAME="$2"
        LOG_NAME_SET=1
        echo "Setting LOG_NAME=$LOG_NAME from command line"
      else
        echo "Ignoring --log-name flag as LOG_NAME is already set to $LOG_NAME"
      fi
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

echo "Running with final configuration: DA_MODE=$DA_MODE, AC_SPLIT=$AC_SPLIT"

# Setup paths
CURRENT_DIR=$(pwd)
APP_DIR="$CURRENT_DIR/test/app"

# Create test/app directory if it doesn't exist
mkdir -p "$APP_DIR"

# Compatible sed command for Mac and Linux
sed_inplace() {
  if [ "$OS_TYPE" = "Darwin" ]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

# Get Go bin directory
get_go_bin_path() {
  # First try to get from GOPATH environment variable
  if [ -n "$GOPATH" ]; then
    GO_BIN="$GOPATH/bin"
  # If GOPATH is not set, use default ~/go/bin path
  else
    GO_BIN="$HOME/go/bin"
  fi
  
  # Ensure directory exists
  mkdir -p "$GO_BIN"
  echo "$GO_BIN"
}

# Setup Kurtosis environment
setup_kurtosis() {
  echo "Setting up Kurtosis environment..."
  
  # Check if kurtosis is installed
  if ! command -v kurtosis &> /dev/null; then
    echo "Installing Kurtosis CLI..."
    if [ "$OS_TYPE" = "Darwin" ]; then
      # macOS installation
      echo "Installing Kurtosis using Homebrew..."
      /opt/homebrew/bin/brew install kurtosis-tech/tap/kurtosis-cli || {
        echo "Homebrew installation failed, trying alternative method..."
        ARCH=$(uname -m)
        if [ "$ARCH" = "arm64" ]; then
          curl -L "https://github.com/kurtosis-tech/kurtosis-cli-release-artifacts/releases/latest/download/kurtosis-cli_darwin_arm64.tar.gz" -o kurtosis-cli.tar.gz
        else
          curl -L "https://github.com/kurtosis-tech/kurtosis-cli-release-artifacts/releases/latest/download/kurtosis-cli_darwin_amd64.tar.gz" -o kurtosis-cli.tar.gz
        fi
        tar -xzf kurtosis-cli.tar.gz
        sudo mv kurtosis /usr/local/bin/
        rm kurtosis-cli.tar.gz
      }
      
      # Reload PATH and shell configuration
      export PATH="/opt/homebrew/bin:$PATH"
      source ~/.bash_profile 2>/dev/null || source ~/.bashrc 2>/dev/null || source ~/.zshrc 2>/dev/null || true
    else
      # Linux installation
      echo "Installing Kurtosis on Linux..."
      # Add Kurtosis repository
      echo "deb [trusted=yes] https://apt.fury.io/kurtosis-tech/ /" | sudo tee /etc/apt/sources.list.d/kurtosis.list
      sudo apt-get update
      sudo apt-get install -y kurtosis-cli
      
      # Verify installation
      if ! command -v kurtosis &> /dev/null; then
        echo "Failed to install Kurtosis via apt, trying manual installation..."
        ARCH=$(uname -m)
        if [ "$ARCH" = "x86_64" ]; then
          curl -L "https://github.com/kurtosis-tech/kurtosis-cli-release-artifacts/releases/latest/download/kurtosis-cli_linux_amd64.tar.gz" -o kurtosis-cli.tar.gz
        else
          curl -L "https://github.com/kurtosis-tech/kurtosis-cli-release-artifacts/releases/latest/download/kurtosis-cli_linux_arm64.tar.gz" -o kurtosis-cli.tar.gz
        fi
        tar -xzf kurtosis-cli.tar.gz
        sudo mv kurtosis /usr/local/bin/
        rm kurtosis-cli.tar.gz
      fi
    fi
  fi
  
  # Verify and configure Kurtosis
  echo "Configuring Kurtosis..."
  kurtosis version
  kurtosis analytics disable
  
  # Check if kurtosis-cdk repository exists
  KURTOSIS_CDK_REPO="$APP_DIR/kurtosis-cdk"
  KURTOSIS_CDK_BRANCH="v0.2.24"
  
  if [ -d "$KURTOSIS_CDK_REPO" ]; then
    echo "Kurtosis-cdk repository already exists, updating to $KURTOSIS_CDK_BRANCH..."
    cd "$KURTOSIS_CDK_REPO"
    
    # Fetch latest changes
    git fetch
    
    # Check current branch/tag
    CURRENT_BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null || git describe --tags --exact-match 2>/dev/null || git rev-parse HEAD)
    
    if [ "$CURRENT_BRANCH" != "$KURTOSIS_CDK_BRANCH" ]; then
      echo "Switching from $CURRENT_BRANCH to $KURTOSIS_CDK_BRANCH"
      git checkout $KURTOSIS_CDK_BRANCH
    else
      echo "Already on $KURTOSIS_CDK_BRANCH"
    fi
    
    # Check if in detached HEAD state
    if git symbolic-ref -q HEAD >/dev/null; then
      # On a branch, can pull normally
      echo "On a branch, pulling latest changes..."
      git pull
    else
      # In detached HEAD state (e.g., on a tag), don't try to pull
      echo "On detached HEAD state (tag or commit), not pulling changes"
    fi
  else
    echo "Cloning kurtosis-cdk repository..."
    git clone --branch $KURTOSIS_CDK_BRANCH https://github.com/0xPolygon/kurtosis-cdk.git "$KURTOSIS_CDK_REPO"
  fi
  
  # Install Foundry if not installed
  source ~/.bashrc 2>/dev/null || source ~/.bash_profile 2>/dev/null || source ~/.zshrc 2>/dev/null || true
  if ! command -v forge &> /dev/null; then
    echo "Installing Foundry..."
    curl -L https://foundry.paradigm.xyz | bash
    source ~/.bashrc || source ~/.bash_profile || true
    foundryup
  fi
  
  # Get Go bin path
  GO_BIN_PATH=$(get_go_bin_path)
  echo "Using Go bin path: $GO_BIN_PATH"
  
  # Install polycli if not installed
  if ! command -v polycli &> /dev/null && [ ! -f "$GO_BIN_PATH/polycli" ]; then
    echo "Installing polycli to Go bin directory: $GO_BIN_PATH"
    if [ "$OS_TYPE" = "Darwin" ]; then
      # macOS installation
      ARCH=$(uname -m)
      if [ "$ARCH" = "arm64" ]; then
        POLYCLI_URL="https://github.com/0xPolygon/polygon-cli/releases/download/v0.1.48/polycli_v0.1.48_darwin_arm64"
      else
        POLYCLI_URL="https://github.com/0xPolygon/polygon-cli/releases/download/v0.1.48/polycli_v0.1.48_darwin_amd64"
      fi
      # Download binary directly to Go bin directory
      curl -L "$POLYCLI_URL" -o "$GO_BIN_PATH/polycli"
      chmod +x "$GO_BIN_PATH/polycli"
    else
      # Linux installation
      tmp_dir=$(mktemp -d)
      curl -L https://github.com/0xPolygon/polygon-cli/releases/download/v0.1.48/polycli_v0.1.48_linux_amd64.tar.gz | tar -xz -C "$tmp_dir"
      mv "$tmp_dir"/* "$GO_BIN_PATH/polycli" && rm -rf "$tmp_dir"
      chmod +x "$GO_BIN_PATH/polycli"
    fi
    
    # Ensure Go bin directory is in PATH
    if [[ ":$PATH:" != *":$GO_BIN_PATH:"* ]]; then
      echo "Adding $GO_BIN_PATH to PATH for this session"
      export PATH="$GO_BIN_PATH:$PATH"
    fi
    
    # Test installation
    "$GO_BIN_PATH/polycli" version || echo "Warning: polycli installation may not be complete"
  else
    echo "polycli already installed"
  fi
  
  # Install yq if not installed
  if ! command -v yq &> /dev/null; then
    echo "Installing yq..."
    if [ "$OS_TYPE" = "Darwin" ]; then
      # macOS installation
      ARCH=$(uname -m)
      if [ "$ARCH" = "arm64" ]; then
        YQ_ARCH="darwin_arm64"
      else
        YQ_ARCH="darwin_amd64"
      fi
      sudo curl -L https://github.com/mikefarah/yq/releases/download/v4.44.2/yq_${YQ_ARCH} -o /usr/local/bin/yq
    else
      # Linux installation
      sudo curl -L https://github.com/mikefarah/yq/releases/download/v4.44.2/yq_linux_amd64 -o /usr/local/bin/yq
    fi
    sudo chmod +x /usr/local/bin/yq
    /usr/local/bin/yq --version
  fi
  
  # Build docker image
  echo "Building docker image..."
  cd "$CURRENT_DIR"
  docker build -t cdk-erigon:local --file Dockerfile .
  
  # Remove unused flags
  echo "Configuring Kurtosis CDK templates..."
  cd "$KURTOSIS_CDK_REPO"
  sed_inplace '/zkevm.sequencer-batch-seal-time:/d' templates/cdk-erigon/config.yml
  sed_inplace '/zkevm.sequencer-non-empty-batch-seal-time:/d' templates/cdk-erigon/config.yml
  sed_inplace '/zkevm\.sequencer-initial-fork-id/d' ./templates/cdk-erigon/config.yml
  sed_inplace '/sentry.drop-useless-peers:/d' templates/cdk-erigon/config.yml
  sed_inplace '/zkevm\.pool-manager-url/d' ./templates/cdk-erigon/config.yml
  
  # Mac and Linux handle appending lines differently
  if [ "$OS_TYPE" = "Darwin" ]; then
    echo "zkevm.disable-virtual-counters: true" >> ./templates/cdk-erigon/config.yml
  else
    sed_inplace '$a\zkevm.disable-virtual-counters: true' ./templates/cdk-erigon/config.yml
  fi
  
  sed_inplace '/zkevm.l2-datastreamer-timeout:/d' templates/cdk-erigon/config.yml
  
  if [ "$AC_SPLIT" = "ac-split" ]; then
    echo "Will use ac-split"
    echo -e "\n"  >> templates/cdk-erigon/config.yml
    echo "zkevm.standalone-smt-db: true" >> templates/cdk-erigon/config.yml
    echo "zkevm.enable-async-commit: true" >> templates/cdk-erigon/config.yml
  fi
  
  # Create params.yml overrides
  echo "Creating params.yml overrides..."
  echo 'args:' > params.yml
  echo '  cdk_erigon_node_image: cdk-erigon:local' >> params.yml
  echo '  el-1-geth-lighthouse: ethpandaops/lighthouse@sha256:4902d9e4a6b6b8d4c136ea54f0e51582a32f356f3dec7194a1adee13ed2d662e' >> params.yml
  /usr/local/bin/yq -i ".args.data_availability_mode = \"$DA_MODE\"" params.yml
  
  sed_inplace 's/"londonBlock": [0-9]\+/"londonBlock": 0/' ./templates/cdk-erigon/chainspec.json
  sed_inplace 's/"normalcyBlock": [0-9]\+/"normalcyBlock": 0/' ./templates/cdk-erigon/chainspec.json
  sed_inplace 's/"shanghaiTime": [0-9]\+/"shanghaiTime": 0/' ./templates/cdk-erigon/chainspec.json
  sed_inplace 's/"cancunTime": [0-9]\+/"cancunTime": 0/' ./templates/cdk-erigon/chainspec.json
  sed_inplace '/"terminalTotalDifficulty"/d' ./templates/cdk-erigon/chainspec.json
  
  # Deploy Kurtosis CDK package
  echo "Deploying Kurtosis CDK package..."
  kurtosis run --enclave cdk-v1 --args-file params.yml --image-download always . '{"args": {"erigon_strict_mode": false, "cdk_erigon_node_image": "cdk-erigon:local"}}'
}

# Call the setup function
setup_kurtosis

# Update paths after setup
CDK_ERIGON_DIR="$CURRENT_DIR"
KURTOSIS_CDK_DIR="$APP_DIR/kurtosis-cdk"

# Wait for 30 seconds
sleep 30

# Monitor verified batches
echo "Monitoring verified batches..."
cd "$KURTOSIS_CDK_DIR"
timeout 900s .github/scripts/monitor-verified-batches.sh --enclave cdk-v1 --rpc-url $(kurtosis port print cdk-v1 cdk-erigon-rpc-001 rpc) --target 20 --timeout 900

# Set up Docker Buildx
echo "Setting up Docker Buildx..."
if ! docker buildx inspect &>/dev/null; then
  docker buildx create --use
fi

# Set up environment variables
echo "Setting up environment variables..."
cd "$CURRENT_DIR"
# Clean up existing directory if it exists
if [ -d "bridge-config-artifact" ]; then
  echo "Removing existing bridge-config-artifact directory..."
  rm -rf bridge-config-artifact
fi
kurtosis files download cdk-v1 bridge-config-artifact
BRIDGE_ADDRESS=$(/usr/local/bin/yq '.NetworkConfig.PolygonBridgeAddress' bridge-config-artifact/bridge-config.toml)
ETH_RPC_URL=$(kurtosis port print cdk-v1 el-1-geth-lighthouse rpc)
BRIDGE_API_URL=$(kurtosis port print cdk-v1 zkevm-bridge-service-001 rpc)
L2_RPC_URL=$(kurtosis port print cdk-v1 cdk-erigon-rpc-001 rpc)

echo "BRIDGE_ADDRESS=$BRIDGE_ADDRESS"
echo "ETH_RPC_URL=$ETH_RPC_URL"
echo "BRIDGE_API_URL=$BRIDGE_API_URL"
echo "L2_RPC_URL=$L2_RPC_URL"

# Handle bridge repository
BRIDGE_REPO="$APP_DIR/bridge"
BRIDGE_BRANCH="v0.6.0-RC10"

if [ -d "$BRIDGE_REPO" ]; then
  echo "Bridge repository already exists, updating to $BRIDGE_BRANCH..."
  cd "$BRIDGE_REPO"
  
  # Fetch latest changes
  git fetch
  
  # Check current branch/tag
  CURRENT_BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null || git describe --tags --exact-match 2>/dev/null || git rev-parse HEAD)
  
  if [ "$CURRENT_BRANCH" != "$BRIDGE_BRANCH" ]; then
    echo "Switching from $CURRENT_BRANCH to $BRIDGE_BRANCH"
    git checkout $BRIDGE_BRANCH
    # Update submodules after branch change
    git submodule update --init --recursive
  else
    echo "Already on $BRIDGE_BRANCH"
    
    # Check if in detached HEAD state (like we did for kurtosis-cdk)
    if git symbolic-ref -q HEAD >/dev/null; then
      # On a branch, can perform other operations
      echo "On a branch, updating submodules..."
      git submodule update --init --recursive
    else
      # In detached HEAD state, only update submodules
      echo "On detached HEAD state (tag or commit), just updating submodules"
      git submodule update --init --recursive
    fi
  fi
else
  echo "Cloning bridge repository..."
  git clone --recurse-submodules -j8 https://github.com/0xPolygonHermez/zkevm-bridge-service.git -b $BRIDGE_BRANCH "$BRIDGE_REPO"
fi

# Build docker image
echo "Building docker image..."
cd "$BRIDGE_REPO"
make build-docker-e2e-real_network

# Run test ERC20 Bridge
echo "Running ERC20 Bridge test..."
mkdir -p tmp
cat <<EOF > ./tmp/test.toml
TestL1AddrPrivate="0x12d7de8621a77640c9241b2595ba78ce443d05e94090365ab3bb5e19df82c625"
TestL2AddrPrivate="0x12d7de8621a77640c9241b2595ba78ce443d05e94090365ab3bb5e19df82c625"
[ConnectionConfig]
L1NodeURL="http://$ETH_RPC_URL"
L2NodeURL="$L2_RPC_URL"
BridgeURL="$BRIDGE_API_URL"
L1BridgeAddr="$BRIDGE_ADDRESS"
L2BridgeAddr="$BRIDGE_ADDRESS"
EOF
docker run --network=host --volume "./tmp/:/config/" --env BRIDGE_TEST_CONFIG_FILE=/config/test.toml bridge-e2e-realnetwork-erc20

# Upload evm-rpc-tests logs
if [ -f "$CDK_ERIGON_DIR/logs/evm-rpc-tests.log" ]; then
  echo "Uploading evm-rpc-tests logs..."
  mkdir -p "$LOGS_DIR"
  cp "$CDK_ERIGON_DIR/logs/evm-rpc-tests.log" "$LOGS_DIR/${LOG_NAME}.log"
  echo "Logs saved to $LOGS_DIR/${LOG_NAME}.log"
fi

# Create logs directory
run_status=$?
if [ $run_status -ne 0 ]; then
  echo "Tests failed. Collecting logs..."
  
  mkdir -p "$LOGS_DIR"
  cd "$LOGS_DIR"
  
  kurtosis service logs cdk-v1 cdk-erigon-rpc-001 --all > cdk-erigon-rpc-001.log
  kurtosis service logs cdk-v1 cdk-erigon-sequencer-001 --all > cdk-erigon-sequencer-001.log
  kurtosis service logs cdk-v1 zkevm-agglayer-001 --all > zkevm-agglayer-001.log
  kurtosis service logs cdk-v1 zkevm-prover-001 --all > zkevm-prover-001.log
  kurtosis service logs cdk-v1 cdk-node-001 --all > cdk-node-001.log
  kurtosis service logs cdk-v1 zkevm-bridge-service-001 --all > zkevm-bridge-service-001.log
  
  echo "Logs collected in $LOGS_DIR directory"
  exit $run_status
fi

echo "All tests completed successfully!" 