#!/bin/bash
# scripts/test-docker.sh
#
# Host-side runner for istioctl-env Docker integration tests.
# This script builds the Docker image and runs the test container.

set -euo pipefail

# ── Colour helpers ─────────────────────────────────────────────────────────────
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BOLD}=========================================${NC}"
echo -e "${BOLD}  istioctl-env Docker Integration Test Runner  ${NC}"
echo -e "${BOLD}=========================================${NC}"
echo ""

# Ensure we are in the project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/.."

IMAGE_NAME="istioctl-env-test"

echo "Building Docker image (Dockerfile.test)..."
docker build -f Dockerfile.test -t "${IMAGE_NAME}" .

echo ""
echo -e "${BOLD}Launching tests inside Docker container...${NC}"
echo ""

# Run the container with TERM set to ensure BATS/tput works correctly
docker run --rm -e TERM=xterm "${IMAGE_NAME}"
