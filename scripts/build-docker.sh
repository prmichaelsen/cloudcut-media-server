#!/bin/bash
set -e

VERSION=${1:-latest}
IMAGE_NAME="cloudcut-media-server"

echo "Building Docker image: ${IMAGE_NAME}:${VERSION}"

docker build -t ${IMAGE_NAME}:${VERSION} .

echo "Build complete!"
echo "Image size:"
docker images ${IMAGE_NAME}:${VERSION} --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"

echo ""
echo "To run locally:"
echo "  docker run -p 8080:8080 ${IMAGE_NAME}:${VERSION}"
echo ""
echo "To test with docker-compose:"
echo "  docker-compose up"
