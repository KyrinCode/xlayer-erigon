#!/bin/bash

SCRIPT_DIR=$(pwd)

cd ../../../
ROOT_DIR=$(pwd)

docker build -t mdbx2rocksdb -f cmd/utils/mdbx2rocksdb/Dockerfile .

cd "$SCRIPT_DIR"

docker run -v "$ROOT_DIR/test/data:/data" mdbx2rocksdb --mdbx /data/seq/chaindata --rocksdb /data/seq/chaindata_rocks --verbose