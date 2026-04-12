#!/usr/bin/env bash
# Test script for Aetronyx M1 — run unit and integration tests with coverage.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo "Running Aetronyx M1 tests..."

# Run all tests with coverage
echo "Running unit tests with race detector..."
go test -race -count=1 -coverprofile=coverage.out ./...

echo "Running integration tests..."
go test -race -count=1 ./test/integration/...

# Generate coverage report
echo "Generating coverage report..."
go tool cover -func=coverage.out | tail -1

# Extract total coverage percentage
total=$(go tool cover -func=coverage.out | tail -1 | awk '{print $(NF-1)}')
total_num=${total%\%}

echo ""
echo -e "${GREEN}✓ All tests passed${NC}"
echo "Total coverage: ${total}"

# Check if coverage meets target (75% for M1)
if (( $(echo "$total_num >= 75" | bc -l) )); then
    echo -e "${GREEN}✓ Coverage target (75%) met${NC}"
else
    echo -e "${RED}✗ Coverage below target: ${total} < 75%${NC}"
fi

exit 0
