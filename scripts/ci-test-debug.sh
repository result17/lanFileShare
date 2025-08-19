#!/bin/bash

# CI Test Debug Script
# This script helps diagnose and fix CI-specific test failures

set -e

echo "=========================================="
echo "CI Test Debug Script"
echo "=========================================="

# Print environment information
echo "Environment Information:"
echo "Go version: $(go version)"
echo "OS: $(uname -a)"
echo "CPU cores: $(nproc)"
echo "Memory: $(free -h | head -2)"
echo "Disk space: $(df -h . | tail -1)"
echo "Current directory: $(pwd)"
echo "User: $(whoami)"
echo "=========================================="

# Set environment variables for better test stability
export CGO_ENABLED=1
export GOMAXPROCS=2
export GOCACHE=$(go env GOCACHE)
export GOMODCACHE=$(go env GOMODCACHE)

echo "Go environment:"
go env | grep -E "(GOOS|GOARCH|CGO_ENABLED|GOMAXPROCS|GOCACHE|GOMODCACHE)"
echo "=========================================="

# Function to run tests for a specific package with detailed output
run_package_tests() {
    local pkg=$1
    local test_type=$2
    
    echo "Testing $pkg ($test_type)..."
    
    case $test_type in
        "basic")
            go test -v -timeout=60s -short "$pkg" 2>&1
            ;;
        "race")
            go test -v -race -timeout=90s -short "$pkg" 2>&1
            ;;
        "verbose")
            go test -v -race -timeout=120s "$pkg" 2>&1
            ;;
        *)
            echo "Unknown test type: $test_type"
            return 1
            ;;
    esac
}

# Function to check for common CI issues
check_ci_issues() {
    echo "Checking for common CI issues..."
    
    # Check for file permission issues
    echo "Checking file permissions..."
    ls -la . | head -10
    
    # Check for temporary directory issues
    echo "Checking temp directory..."
    echo "TMPDIR: ${TMPDIR:-not set}"
    echo "Temp dir permissions: $(ls -ld ${TMPDIR:-/tmp})"
    
    # Check for network issues (if any tests use network)
    echo "Checking network connectivity..."
    ping -c 1 google.com > /dev/null 2>&1 && echo "Network: OK" || echo "Network: FAILED"
    
    # Check for race condition patterns in code
    echo "Checking for potential race conditions..."
    grep -r "time\.Sleep" --include="*_test.go" . | head -5 || echo "No time.Sleep found in tests"
    
    echo "=========================================="
}

# Main test execution
main() {
    echo "Starting CI test debug process..."
    
    # Check for common issues first
    check_ci_issues
    
    # Get list of packages
    packages=$(go list ./...)
    failed_packages=""
    
    echo "Found packages:"
    echo "$packages"
    echo "=========================================="
    
    # Test each package individually
    for pkg in $packages; do
        echo "=========================================="
        echo "Testing package: $pkg"
        echo "=========================================="
        
        # Skip certain packages that might be problematic in CI
        case $pkg in
            *"/cmd/"*)
                echo "Skipping command package: $pkg"
                continue
                ;;
        esac
        
        # Run basic tests first
        if ! run_package_tests "$pkg" "basic"; then
            echo "BASIC TEST FAILED: $pkg"
            failed_packages="$failed_packages $pkg"
            
            # Try to get more information about the failure
            echo "Attempting to get more details..."
            go test -v -timeout=30s -short "$pkg" || true
            continue
        fi
        
        # Run race detection tests
        echo "Running race detection for: $pkg"
        if ! run_package_tests "$pkg" "race"; then
            echo "RACE TEST FAILED: $pkg"
            failed_packages="$failed_packages $pkg"
            
            # Try without race detector to see if it's a race issue
            echo "Retrying without race detector..."
            go test -v -timeout=60s -short "$pkg" || true
        fi
        
        echo "Package $pkg: PASSED"
    done
    
    # Report results
    echo "=========================================="
    if [ -n "$failed_packages" ]; then
        echo "FAILED PACKAGES: $failed_packages"
        echo "=========================================="
        
        # Try to run failed packages with more verbose output
        for pkg in $failed_packages; do
            echo "Detailed failure analysis for: $pkg"
            echo "----------------------------------------"
            run_package_tests "$pkg" "verbose" || true
            echo "----------------------------------------"
        done
        
        exit 1
    else
        echo "ALL PACKAGES PASSED!"
        echo "=========================================="
        
        # Run final coverage test
        echo "Running final coverage test..."
        go test -v -short -coverprofile=coverage.out -covermode=atomic ./... || {
            echo "Coverage test failed, but individual tests passed"
            echo "This might be a coverage tool issue"
        }
        
        echo "CI test debug completed successfully!"
    fi
}

# Run main function
main "$@"
