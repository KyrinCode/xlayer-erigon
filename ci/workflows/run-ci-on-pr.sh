#!/bin/bash

# This script is used to run CI tasks in Docker containers
# Here we define the tasks
declare -A tasks
tasks["build"]="make cdk-erigon"
tasks["tests"]="make -B test"
tasks["lint"]="cd ./docs/endpoints && make check-doc"
tasks["unwind"]="make unwind"

declare -A task_status

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Get git tag (commit hash)
GIT_ROOT=`git rev-parse --show-toplevel`
cd $GIT_ROOT
GIT_TAG=`git rev-parse HEAD`
echo "Testing for git commit: $GIT_TAG"

# Build base image
docker build -f Dockerfile.ci -t xlayer-erigon-ci:latest --build-arg GIT_TAG=$GIT_TAG .

# Base Docker-based command
BASE_CMD="docker run xlayer-erigon-ci:latest"

# Run all tasks
for task in "${!tasks[@]}"; do
    echo "Running task: $task"
    CMD="${BASE_CMD} sh -c \"${tasks[$task]}\""
    echo "Command: $CMD"
    eval $CMD > logs-$task.log 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${NC}Task $task ${RED}failed${NC}."
        task_status[$task]="failed"
    else
        echo -e "${NC}Task $task ${GREEN}succeeded${NC}."
        task_status[$task]="succeeded"
    fi
done

echo "All tasks completed."