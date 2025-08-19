#!/bin/bash

# Test Like CI Script
# This script simulates the CI environment locally to help debug CI failures

set -e

echo "=========================================="
echo "Testing Like CI Environment"
echo "=========================================="

# Set CI environment variables
export CI=true
export GITHUB_ACTIONS=true
export CONTINUOUS_INTEGRATION=true
export GOMAXPROCS=2
export CGO_ENABLED=1

# Function to run tests like CI
run_ci_tests() {
    echo "Running tests with CI configuration..."
    
    # Download dependencies
    echo "Downloading dependencies..."
    go mod download
    
    # Verify dependencies
    echo "Verifying dependencies..."
    go mod verify
    
    # Run go vet
    echo "Running go vet..."
    go vet ./...
    
    # Run tests for each package separately (like CI)
    echo "Running tests for each package..."
    failed_packages=""
    
    for pkg in $(go list ./...); do
        echo "=========================================="
        echo "Testing package: $pkg"
        echo "=========================================="
        
        # Run basic tests first
        if ! go test -v -timeout=60s -short "$pkg"; then
            echo "BASIC TEST FAILED: $pkg"
            failed_packages="$failed_packages $pkg"
            continue
        fi
        
        # Run with race detector
        echo "Running race detection for: $pkg"
        if ! go test -v -race -timeout=90s -short "$pkg"; then
            echo "RACE TEST FAILED: $pkg"
            failed_packages="$failed_packages $pkg"
        fi
    done
    
    # Report failed packages
    if [ -n "$failed_packages" ]; then
        echo "=========================================="
        echo "FAILED PACKAGES: $failed_packages"
        echo "=========================================="
        return 1
    fi
    
    # Run coverage test
    echo "=========================================="
    echo "Running coverage test..."
    echo "=========================================="
    go test -v -short -coverprofile=coverage.out -covermode=atomic ./...
    
    echo "All tests passed!"
    return 0
}

# Function to analyze test failures
analyze_failures() {
    echo "Analyzing potential CI failure causes..."
    
    # Check for time-sensitive tests
    echo "Checking for time-sensitive patterns..."
    grep -r "time\.Sleep" --include="*_test.go" . | head -10 || echo "No time.Sleep found"
    grep -r "time\.After" --include="*_test.go" . | head -10 || echo "No time.After found"
    
    # Check for file system operations
    echo "Checking for file system operations..."
    grep -r "os\.Create\|os\.Open\|ioutil\." --include="*_test.go" . | head -10 || echo "No file operations found"
    
    # Check for network operations
    echo "Checking for network operations..."
    grep -r "net\.\|http\." --include="*_test.go" . | head -10 || echo "No network operations found"
    
    # Check for parallel tests
    echo "Checking for parallel tests..."
    grep -r "t\.Parallel" --include="*_test.go" . | head -10 || echo "No parallel tests found"
    
    # Check for race conditions
    echo "Checking for potential race conditions..."
    grep -r "go func\|goroutine" --include="*_test.go" . | head -10 || echo "No goroutines found in tests"
}

# Function to suggest fixes
suggest_fixes() {
    echo "=========================================="
    echo "Suggested fixes for CI failures:"
    echo "=========================================="
    
    echo "1. Increase timeouts in CI environment"
    echo "   - Use environment variables to detect CI"
    echo "   - Multiply timeouts by 2-3x in CI"
    
    echo "2. Reduce test parallelism"
    echo "   - Set GOMAXPROCS=2 in CI"
    echo "   - Avoid t.Parallel() in resource-intensive tests"
    
    echo "3. Make tests more deterministic"
    echo "   - Replace time.Sleep with proper synchronization"
    echo "   - Use channels or sync.WaitGroup instead of sleep"
    
    echo "4. Handle file system differences"
    echo "   - Use t.TempDir() for temporary files"
    echo "   - Handle case-sensitive file systems"
    
    echo "5. Skip flaky tests in CI"
    echo "   - Use build tags: // +build !ci"
    echo "   - Check CI environment variables"
    
    echo "6. Add retry logic for flaky operations"
    echo "   - Retry network operations"
    echo "   - Retry file operations that might fail due to timing"
}

# Main execution
main() {
    echo "Starting CI simulation..."
    
    # Show environment
    echo "Environment:"
    echo "  Go version: $(go version)"
    echo "  OS: $(uname -a)"
    echo "  CPU cores: $(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 'unknown')"
    echo "  CI: $CI"
    echo "  GOMAXPROCS: $GOMAXPROCS"
    echo "=========================================="
    
    # Analyze potential issues first
    analyze_failures
    
    echo "=========================================="
    echo "Running tests..."
    echo "=========================================="
    
    # Run tests
    if run_ci_tests; then
        echo "=========================================="
        echo "✅ All tests passed in CI simulation!"
        echo "=========================================="
    else
        echo "=========================================="
        echo "❌ Tests failed in CI simulation"
        echo "=========================================="
        suggest_fixes
        exit 1
    fi
}

# Check if script is being sourced or executed
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
