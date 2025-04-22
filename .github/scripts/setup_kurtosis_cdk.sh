#!/bin/bash

# Start Docker daemon
dockerd > /dockerd.log 2>&1 &
sleep 5

# Build xlayer-erigon
cd /app/xlayer-erigon
docker build -t cdk-erigon:local --file Dockerfile .

# Setup and run Kurtosis
cd /app/kurtosis-cdk
/app/xlayer-erigon/.github/scripts/configure_kurtosis_cdk.sh
kurtosis run --enclave cdk-v1 --args-file params.yml --image-download always . '{"args": {"erigon_strict_mode": false, "cdk_erigon_node_image": "cdk-erigon:local"}}'