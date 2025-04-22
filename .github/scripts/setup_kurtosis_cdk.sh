#!/bin/bash

if [ $# -lt 1 ]; then
    echo "Usage: $0 <docker_registry_ip>"
    exit 1
fi
DOCKER_REGISTRY_IP_PORT=$1

# Start Docker daemon
echo "{ \"insecure-registries\":[\"$DOCKER_REGISTRY_IP_PORT\"] }" > /etc/docker/daemon.json
dockerd > /dockerd.log 2>&1 &
sleep 5

# Build xlayer-erigon
cd /app/xlayer-erigon
docker build -t cdk-erigon:local --file Dockerfile .

# Run kurtosis
# Pull Docker images to avoid "You have reached your unauthenticated pull rate limit"
cd ci/utils/
./docker-cache-pull.sh

cd /app/kurtosis-cdk
# kurtosis run --enclave cdk-v1 --args-file params.yml --image-download always . '{"args": {"erigon_strict_mode": false, "cdk_erigon_node_image": "cdk-erigon:local"}}'
kurtosis run --enclave cdk-v1 --args-file params.yml . '{"args": {"erigon_strict_mode": false, "cdk_erigon_node_image": "cdk-erigon:local"}}'