#!/bin/bash

# Local CI Test Script
# Simulates GitHub Actions environment locally for debugging
# Usage: ./scripts/test-like-ci.sh

set -e

echo "┌─────────────────────────────────────────────────────────────────────────────────┐"
echo "│ 🔍 LOCAL CI SIMULATION"
echo "└─────────────────────────────────────────────────────────────────────────────────┘"

# Set CI environment variables to match GitHub Actions
export CI=true
export GITHUB_ACTIONS=true
export CGO_ENABLED=1
export GOMAXPROCS=1  # Match CI settings
export GOMEMLIMIT=1GiB

echo "📊 Environment:"
echo "  Go version: $(go version)"
echo "  OS: $(uname -a 2>/dev/null || echo 'Windows')"
echo "  GOMAXPROCS: $GOMAXPROCS"
echo ""

# Clean test cache
echo "🧹 Cleaning test cache..."
go clean -testcache

# Run tests with CI configuration
echo "┌─────────────────────────────────────────────────────────────────────────────────┐"
echo "│ 🧪 RUNNING TESTS (CI Configuration)"
echo "└─────────────────────────────────────────────────────────────────────────────────┘"

failed_packages=""
total_packages=$(go list ./... | wc -l)
current_package=0

for pkg in $(go list ./...); do
    current_package=$((current_package + 1))
    pkg_name=$(basename "$pkg")

    echo ""
    echo "📦 [$current_package/$total_packages] Testing: $pkg_name"

    # Run with CI settings: extended timeout, no parallelism
    if ! go test -v -timeout=300s -short -count=1 -p=1 "$pkg"; then
        echo "❌ FAILED: $pkg_name"
        failed_packages="$failed_packages $pkg"
    else
        echo "✅ PASSED: $pkg_name"
    fi
done

# Report results
echo ""
if [ -n "$failed_packages" ]; then
    echo "┌─────────────────────────────────────────────────────────────────────────────────┐"
    echo "│ ❌ FAILED PACKAGES:"
    for pkg in $failed_packages; do
        echo "│   - $(basename $pkg)"
    done
    echo "└─────────────────────────────────────────────────────────────────────────────────┘"
    exit 1
else
    echo "┌─────────────────────────────────────────────────────────────────────────────────┐"
    echo "│ ✅ ALL TESTS PASSED!"
    echo "└─────────────────────────────────────────────────────────────────────────────────┘"
fi


