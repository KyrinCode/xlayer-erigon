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
tasks_dind["kurtosis-cdk"]="./.github/scripts/run_kurtosis_cdk"
tasks_dind["kurtosis-cdk-post-london"]="./.github/scripts/run_kurtosis_cdk_post_london.sh"
tasks_dind["data_loss"]="cd ./test && make test-data-loss"
tasks_dind["test-e2e"]="cd test && make test-e2e"
tasks_dind["test-executor"]="./.github/scripts/update_config.sh && cd test && make test-executor"

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

BASE_CMD="--privileged xlayer-erigon-ci:latest sh -c \"./.github/scripts/configure_kurtosis_cdk.sh && ./.github/scripts/setup_kurtosis_cdk.sh"

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