#!/bin/bash
set -e

echo "Building cnetflow_gobackend for Debian using Docker..."

# Create output directory if it doesn't exist
mkdir -p output

# Build the Docker image
echo "Building Docker image..."
docker build -f Dockerfile.debian -t cnetflow-debian-builder .

# Run the container to build and extract DEB
echo "Running build container..."
docker run --rm -v "$(pwd)/output:/output" cnetflow-debian-builder

echo ""
echo "Build complete! DEB package available in output/ directory:"
ls -lh output/*.deb
