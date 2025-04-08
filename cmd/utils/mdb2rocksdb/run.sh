#!/bin/bash

# 获取项目根目录的绝对路径
ROOT_DIR=$(git rev-parse --show-toplevel)
CURRENT_DIR=$(pwd)

# --- 检查并安装依赖 ---
echo "Checking/installing dependencies (snappy, lz4, zstd)..."
brew list snappy > /dev/null || brew install snappy
brew list lz4 > /dev/null || brew install lz4
brew list zstd > /dev/null || brew install zstd
# --------------------

# --- 获取依赖库的 Homebrew 路径 ---
SNAPPY_PATH=$(brew --prefix snappy)
LZ4_PATH=$(brew --prefix lz4)
ZSTD_PATH=$(brew --prefix zstd)
# -----------------------------------

# --- 确保 grocksdb 版本兼容 ---
echo "Forcing compatible grocksdb version..."
# 使用 -u 确保获取指定版本，即使已存在
go get -u github.com/linxGnu/grocksdb@v1.6.24
go mod tidy
# --------------------------------

echo "Preparing environment with RocksDB from submodule..."
echo "Project root: $ROOT_DIR"
echo "Current directory: $CURRENT_DIR"

# 设置编译环境变量，指向 submodule 和系统依赖库
export CGO_CFLAGS="-I$ROOT_DIR/deps/rocksdb/include"
export CGO_LDFLAGS="-L$ROOT_DIR/deps/rocksdb -L$SNAPPY_PATH/lib -L$LZ4_PATH/lib -L$ZSTD_PATH/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd"

echo "CGO_CFLAGS=$CGO_CFLAGS"
echo "CGO_LDFLAGS=$CGO_LDFLAGS"

# --- 修改：使用 go run 并传递参数 ---
echo "Running mdbx2rocksdb via go run..."
# 使用 "$@" 将所有传递给脚本的参数转发给 go run
go run main.go "$@"
# ----------------------------------

# 获取 go run 的退出状态
EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo "Execution finished successfully."
else
    echo "Execution failed with exit code $EXIT_CODE."
fi

exit $EXIT_CODE