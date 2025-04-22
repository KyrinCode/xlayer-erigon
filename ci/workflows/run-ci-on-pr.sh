#!/bin/bash

# This script is used to run CI tasks in Docker containers
# Here we define the tasks that do not need Docker-in-Docker (DinD)
declare -A tasks
tasks["build"]="make cdk-erigon"
tasks["tests"]="make -B test"
tasks["lint"]="cd ./docs/endpoints && make check-doc"
tasks["check_chinese_characters"]="./.github/scripts/check_chinese_characters.sh"
tasks["unwind"]="make test-unwind"

# Tasks that require Docker-in-Docker
declare -A tasks_dind
tasks_dind["kurtosis-cdk"]="./.github/scripts/run_kurtosis_cdk.sh"
tasks_dind["kurtosis-cdk-post-london"]="./.github/scripts/run_kurtosis_cdk_post_london.sh"
#tasks_dind["data_loss"]="cd ./test && make test-data-loss"
#tasks_dind["test-e2e"]="cd test && make test-e2e"
#tasks_dind["test-executor"]="./.github/scripts/update_config.sh && cd test && make test-executor"

declare -A task_status

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

TSTAMP=$(date +%Y%m%d%H%M%S%N)
LOGSDIR="logs-ci-$TSTAMP"
mkdir -p $LOGSDIR

# Get git tag (commit hash)
GIT_ROOT=`git rev-parse --show-toplevel`
cd $GIT_ROOT
GIT_TAG=`git rev-parse HEAD`
echo "Testing for git commit: $GIT_TAG"

# Build base image
docker build -f Dockerfile.ci -t xlayer-erigon-ci:latest --build-arg GIT_TAG=$GIT_TAG .
CDK_IMAGE_TAG="cdk-erigon:local"
docker build -t $CDK_IMAGE_TAG --file Dockerfile .
docker tag $CDK_IMAGE_TAG localhost:5000/$CDK_IMAGE_TAG
docker push localhost:5000/$CDK_IMAGE_TAG
docker rmi localhost:5000/$CDK_IMAGE_TAG

# Push Docker images to local registry
./ci/utils/docker-cache-push.sh > $LOGSDIR/docker-cache-push.log 2>&1
DOCKER_REGISTRY_IP_PORT=$(cat $LOGSDIR/docker-cache-push.log | grep "Docker registry IP" | cut -d ' ' -f 4)
echo "Docker registry IP: $DOCKER_REGISTRY_IP_PORT"
cd $GIT_ROOT

# Base Docker command
BASE_CMD="docker run xlayer-erigon-ci:latest"

# Run non-dind tasks
for task in "${!tasks[@]}"; do
    echo "Running task: $task"
    CMD="${BASE_CMD} sh -c \"${tasks[$task]}\""
    echo "Command: $CMD"
    eval $CMD > $LOGSDIR/logs-$task.log 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${NC}Task $task ${RED}failed${NC}."
        task_status[$task]="failed"
    else
        echo -e "${NC}Task $task ${GREEN}succeeded${NC}."
        task_status[$task]="succeeded"
    fi
done

# DinD Docker command
BASE_CMD="--privileged xlayer-erigon-ci:latest sh -c \"./.github/scripts/configure_kurtosis_cdk.sh && ./.github/scripts/setup_kurtosis_cdk.sh $DOCKER_REGISTRY_IP_PORT"

# Run DinD (Docker-in-Docker) tasks
for task in "${!tasks_dind[@]}"; do
    echo "Running task: $task"
    LOGSSUBDIR="$LOGSDIR/$task"
    mkdir -p $LOGSSUBDIR
    CMD="docker run -v ./$LOGSSUBDIR:/logs ${BASE_CMD} && ${tasks_dind[$task]}\""
    echo "Command: $CMD"
    eval $CMD > $LOGSDIR/logs-$task.log 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${NC}Task $task ${RED}failed${NC}."
        task_status[$task]="failed"
    else
        echo -e "${NC}Task $task ${GREEN}succeeded${NC}."
        task_status[$task]="succeeded"
    fi
done

echo "All tasks completed. Logs are in $LOGSDIR"