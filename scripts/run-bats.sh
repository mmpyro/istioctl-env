#!/bin/bash
# scripts/run-bats.sh
#
# Internal script to run BATS tests inside the Docker container.

set -uo pipefail

# ── Colour helpers ─────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BOLD}Running BATS suite inside container...${NC}"
mkdir -p /src/reports

# Run BATS and capture exit code
# We use the 'pretty' formatter for human-readable output.
bats --formatter pretty /src/tests/integration.bats
BATS_EXIT=$?

echo ""
if [ $BATS_EXIT -eq 0 ]; then
    echo -e "${GREEN}${BOLD}PASSED: All integration tests successful.${NC}"
else
    echo -e "${RED}${BOLD}FAILED: Some integration tests failed.${NC}"
fi

exit $BATS_EXIT
