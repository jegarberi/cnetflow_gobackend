#!/bin/bash
set -e

echo "Building cnetflow_gobackend for RHEL7 using Docker..."

# Create output directory if it doesn't exist
mkdir -p output

# Build the Docker image
echo "Building Docker image..."
docker build -f Dockerfile.rhel7 -t cnetflow-rhel7-builder .

# Run the container to build and extract RPM
echo "Running build container..."
docker run --rm -v "$(pwd)/output:/output" cnetflow-rhel7-builder

echo ""
echo "Build complete! RPM package available in output/ directory:"
ls -lh output/*.rpm
