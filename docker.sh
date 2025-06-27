#!/bin/bash

# Simple wrapper script to run Docker commands from the root directory

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$SCRIPT_DIR/docker"

# Check if docker directory exists
if [ ! -d "$DOCKER_DIR" ]; then
    echo "Error: Docker directory not found at $DOCKER_DIR"
    exit 1
fi

# Change to docker directory and run the command
cd "$DOCKER_DIR"
./docker-scripts.sh "$@" 