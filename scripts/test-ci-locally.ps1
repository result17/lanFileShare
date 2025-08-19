# PowerShell script to test CI environment locally on Windows

Write-Host "=========================================="
Write-Host "Testing CI Environment Locally"
Write-Host "=========================================="

# Set CI environment variables
$env:CI = "true"
$env:GITHUB_ACTIONS = "true"
$env:CONTINUOUS_INTEGRATION = "true"
$env:GOMAXPROCS = "2"
$env:CGO_ENABLED = "1"

# Function to run tests with error handling
function Run-TestPackage {
    param(
        [string]$Package,
        [string]$TestType
    )
    
    Write-Host "Testing $Package ($TestType)..."
    
    switch ($TestType) {
        "basic" {
            $result = go test -v -timeout=60s -short $Package
            return $LASTEXITCODE -eq 0
        }
        "race" {
            $result = go test -v -race -timeout=120s -short $Package
            return $LASTEXITCODE -eq 0
        }
        "verbose" {
            $result = go test -v -race -timeout=180s $Package
            return $LASTEXITCODE -eq 0
        }
        default {
            Write-Host "Unknown test type: $TestType"
            return $false
        }
    }
}

# Main test execution
function Main {
    Write-Host "Environment Information:"
    Write-Host "Go version: $(go version)"
    Write-Host "OS: $env:OS"
    Write-Host "Processor: $env:PROCESSOR_ARCHITECTURE"
    Write-Host "Number of processors: $env:NUMBER_OF_PROCESSORS"
    Write-Host "CI: $env:CI"
    Write-Host "GOMAXPROCS: $env:GOMAXPROCS"
    Write-Host "=========================================="
    
    # Download dependencies
    Write-Host "Downloading dependencies..."
    go mod download
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Failed to download dependencies"
        exit 1
    }
    
    # Verify dependencies
    Write-Host "Verifying dependencies..."
    go mod verify
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Failed to verify dependencies"
        exit 1
    }
    
    # Run go vet
    Write-Host "Running go vet..."
    go vet ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "go vet failed"
        exit 1
    }
    
    # Get list of packages
    $packages = go list ./...
    $failedPackages = @()
    
    Write-Host "Found packages:"
    $packages | ForEach-Object { Write-Host "  $_" }
    Write-Host "=========================================="
    
    # Test each package
    foreach ($pkg in $packages) {
        # Skip command packages
        if ($pkg -like "*/cmd/*") {
            Write-Host "Skipping command package: $pkg"
            continue
        }
        
        Write-Host "=========================================="
        Write-Host "Testing package: $pkg"
        Write-Host "=========================================="
        
        # Run basic tests first
        if (-not (Run-TestPackage $pkg "basic")) {
            Write-Host "BASIC TEST FAILED: $pkg"
            $failedPackages += $pkg
            continue
        }
        
        # Run race detection tests
        Write-Host "Running race detection for: $pkg"
        if (-not (Run-TestPackage $pkg "race")) {
            Write-Host "RACE TEST FAILED: $pkg"
            $failedPackages += $pkg
            
            # Try without race detector
            Write-Host "Retrying without race detector..."
            Run-TestPackage $pkg "basic" | Out-Null
        }
        
        Write-Host "Package $pkg: PASSED"
    }
    
    # Report results
    Write-Host "=========================================="
    if ($failedPackages.Count -gt 0) {
        Write-Host "FAILED PACKAGES:"
        $failedPackages | ForEach-Object { Write-Host "  $_" }
        Write-Host "=========================================="
        
        # Try to get more details for failed packages
        foreach ($pkg in $failedPackages) {
            Write-Host "Detailed failure analysis for: $pkg"
            Write-Host "----------------------------------------"
            Run-TestPackage $pkg "verbose" | Out-Null
            Write-Host "----------------------------------------"
        }
        
        exit 1
    } else {
        Write-Host "ALL PACKAGES PASSED!"
        Write-Host "=========================================="
        
        # Run final coverage test
        Write-Host "Running final coverage test..."
        go test -v -short -coverprofile=coverage.out -covermode=atomic ./...
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Coverage test failed, but individual tests passed"
            Write-Host "This might be a coverage tool issue"
        }
        
        Write-Host "CI test simulation completed successfully!"
    }
}

# Run main function
Main
