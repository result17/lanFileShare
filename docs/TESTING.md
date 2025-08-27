# Testing Guide

This document outlines testing procedures for the lanFileSharer project, including service discovery, transfer status management, and end-to-end transfer workflows.

## Testing Framework and Best Practices

### Using Testify

This project uses [testify](https://github.com/stretchr/testify) as the primary testing framework. Testify provides powerful assertion and mocking capabilities that make tests more readable and maintainable.

#### Key Testify Packages

- **`assert`**: For non-fatal assertions that continue test execution
- **`require`**: For fatal assertions that stop test execution on failure
- **`mock`**: For creating mock objects (when needed)
- **`suite`**: For test suites (when needed)

#### Basic Usage Examples

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestExample(t *testing.T) {
    // Use require for critical assertions that should stop the test
    data, err := loadTestData()
    require.NoError(t, err, "Failed to load test data")
    require.NotNil(t, data, "Test data should not be nil")

    // Use assert for non-critical checks
    assert.Equal(t, "expected", data.Value, "Data value should match")
    assert.True(t, data.IsValid, "Data should be valid")
    assert.Contains(t, data.Tags, "test", "Should contain test tag")
}
```

#### When to Use `require` vs `assert`

- **Use `require`** for:

  - Setup operations that must succeed (file creation, network connections)
  - Critical preconditions that make the rest of the test meaningless
  - Operations that could cause panics if they fail

- **Use `assert`** for:
  - Business logic verification
  - Multiple related checks in the same test
  - Non-critical validations

#### File and Directory Assertions

```go
// File existence checks
assert.FileExists(t, "/path/to/file", "File should exist")
assert.NoFileExists(t, "/path/to/file", "File should not exist")

// Directory checks
assert.DirExists(t, "/path/to/dir", "Directory should exist")
```

#### Error Handling Patterns

```go
// For operations that should succeed
result, err := someOperation()
require.NoError(t, err, "Operation should succeed")
assert.Equal(t, expectedResult, result)

// For operations that should fail
result, err := invalidOperation()
require.Error(t, err, "Operation should fail")
assert.Contains(t, err.Error(), "expected error message")
assert.Nil(t, result, "Result should be nil on error")
```

#### Collection Assertions

```go
// Slice/Array checks
assert.Len(t, items, 3, "Should have 3 items")
assert.Contains(t, items, expectedItem, "Should contain expected item")
assert.ElementsMatch(t, expected, actual, "Should contain same elements")

// Map checks
assert.Equal(t, expectedMap, actualMap, "Maps should be equal")
```

### Testing Guidelines

**🎯 IMPORTANT: This project uses testify as the standard testing framework. All new tests MUST use testify assertions.**

1. **Always use testify assertions** instead of native Go assertions (`t.Fatal`, `t.Error`, etc.)
2. **Provide descriptive messages** for all assertions to improve debugging
3. **Use `require` for setup and critical checks** that should stop test execution on failure
4. **Use `assert` for business logic validation** that should continue test execution
5. **Group related tests** using subtests (`t.Run()`) for better organization
6. **Clean up resources** using `defer` or `t.Cleanup()` to prevent test pollution
7. **Use table-driven tests** for multiple similar test cases
8. **Prefer specialized assertions** (e.g., `FileExists`, `Contains`) over generic ones

#### Migration Status

- ✅ `pkg/receiver/*_test.go` - Fully migrated to testify
- ✅ `api/*_test.go` - Already using testify
- 🔄 `pkg/crypto/*_test.go` - Partially migrated (in progress)
- ❓ Other packages - Need assessment

**For existing tests**: When modifying existing test files, please migrate any native assertions to testify. See [Testify Migration Guide](docs/testify_migration_guide.md) for detailed instructions.

#### Example Test Structure

```go
func TestFileReceiver_ProcessChunk(t *testing.T) {
    // Setup
    tempDir, err := os.MkdirTemp("", "test")
    require.NoError(t, err, "Failed to create temp directory")
    defer os.RemoveAll(tempDir)

    receiver := NewFileReceiver(tempDir, nil)

    t.Run("successful_processing", func(t *testing.T) {
        // Test implementation
        data := []byte("test data")
        err := receiver.ProcessChunk(data)
        assert.NoError(t, err, "Should process chunk successfully")
    })

    t.Run("invalid_data", func(t *testing.T) {
        // Test implementation
        err := receiver.ProcessChunk(nil)
        assert.Error(t, err, "Should fail with nil data")
    })
}
```

## Service Discovery Testing

### Testing Service Discovery Delay

#### Objective

To measure the time it takes for the `sender` to recognize that a `receiver` service has gone offline.

#### Prerequisites

- You need two separate terminal windows.
- You should be in the root directory of the `lanFileSharer` project.

#### Test Steps

1. **Start the Log Monitor**

   ```sh
   # For Windows (using PowerShell)
   Get-Content -Path debug.log -Wait

   # For Linux or macOS
   tail -f debug.log
   ```

2. **Start the Receiver**

   ```sh
   go run ./cmd/lanfilesharer receive
   ```

3. **Start the Sender**

   ```sh
   go run ./cmd/lanfilesharer send
   ```

4. **Observe the Discovery**

   Look for log messages like:

   ```
   Discovery Update: Found 1 services.
     - Service: My-PC-Receiver-xxxx, Addr: 192.168.1.10, Port: 8080
   ```

5. **Test Service Offline Detection**

   - Shut down the receiver with `Ctrl+C`
   - Measure time until log shows `Discovery Update: Found 0 services.`

## Transfer Status Management Testing

### Unit Testing

Run the transfer status management unit tests:

```sh
# Test the unified transfer manager
go test ./pkg/transfer -run TestUnifiedTransferManager -v

# Test the session status manager
go test ./pkg/transfer -run TestTransferStatusManager -v

# Test all transfer package components
go test ./pkg/transfer -v
```

### Integration Testing

Test the complete transfer workflow:

```sh
# Run integration tests
go test ./pkg/transfer -run TestIntegration -v

# Test with multiple files
go test ./pkg/transfer -run TestMultipleFiles -v
```

### Manual Transfer Testing

#### Single File Transfer Test

1. **Setup Test Environment**

   ```sh
   # Create test files
   mkdir -p test_files
   echo "Test content for single file" > test_files/single.txt
   ```

2. **Start Receiver**

   ```sh
   go run ./cmd/lanfilesharer receive
   ```

3. **Send File and Monitor Status**

   ```sh
   go run ./cmd/lanfilesharer send test_files/single.txt
   ```

4. **Verify Transfer Status**

   - Monitor real-time progress updates
   - Verify completion status
   - Check transfer metrics (rate, ETA)

#### Multi-File Transfer Test

1. **Create Multiple Test Files**

   ```sh
   mkdir -p test_files
   for i in {1..5}; do
     dd if=/dev/zero of=test_files/file$i.dat bs=1M count=$i 2>/dev/null
   done
   ```

2. **Test Session Management**

   ```sh
   go run ./cmd/lanfilesharer send test_files/
   ```

3. **Verify Session Status**

   - Check overall session progress
   - Monitor individual file status
   - Verify session completion

### Performance Testing

#### High-Throughput Testing

```sh
# Create large test file
dd if=/dev/zero of=large_test.dat bs=100M count=1

# Test transfer performance
go run ./cmd/lanfilesharer send large_test.dat
```

#### Concurrent Transfer Testing

```sh
# Test multiple concurrent sessions (if supported)
go test ./pkg/transfer -run TestConcurrent -v
```

### Error Handling Testing

#### Network Interruption Test

1. Start a file transfer
2. Disconnect network interface during transfer
3. Reconnect network
4. Verify transfer recovery

#### Disk Space Test

1. Fill up disk space on receiver
2. Attempt file transfer
3. Verify proper error handling

### Status Event Testing

#### Event Listener Test

```sh
# Test status event notifications
go test ./pkg/transfer -run TestStatusListener -v
```

#### Real-time Updates Test

1. Start a transfer with status monitoring
2. Verify events are emitted for:
   - Transfer start
   - Progress updates
   - Transfer completion
   - Error conditions

## Test Data Management

### Creating Test Files

```sh
# Small files for quick testing
echo "Small test file" > small.txt

# Medium files for progress testing
dd if=/dev/zero of=medium.dat bs=1M count=10

# Large files for performance testing
dd if=/dev/zero of=large.dat bs=100M count=1

# Binary files for integrity testing
dd if=/dev/urandom of=binary.dat bs=1M count=5
```

### Cleanup Test Environment

```sh
# Remove test files
rm -rf test_files/
rm -f *.dat *.txt debug.log
```

## Automated Testing

### Running All Tests

```sh
# Run all tests with coverage
go test ./... -cover

# Run tests with race detection
go test ./... -race

# Run tests with verbose output
go test ./... -v
```

### Continuous Integration Testing

The project includes automated testing for:

- Unit tests for all components
- Integration tests for transfer workflows
- Performance benchmarks
- Error handling scenarios

### Test Coverage

Check test coverage:

```sh
# Generate coverage report
go test ./pkg/transfer -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Testify Testing Framework

### Why Testify?

This project has adopted [testify](https://github.com/stretchr/testify) as the standard testing framework for the following reasons:

1. **Better Readability**: More expressive and readable assertions
2. **Rich Assertion Library**: Specialized assertions for common patterns
3. **Better Error Messages**: More informative failure messages
4. **Consistent API**: Uniform interface across different assertion types
5. **Industry Standard**: Widely adopted in the Go community

### Quick Start with Testify

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestExample(t *testing.T) {
    // Setup (use require for critical operations)
    data, err := setupTestData()
    require.NoError(t, err, "Failed to setup test data")
    require.NotNil(t, data, "Test data should not be nil")

    // Business logic testing (use assert for validations)
    result := processData(data)
    assert.Equal(t, "expected", result.Value, "Processing should return expected value")
    assert.True(t, result.IsValid, "Result should be valid")
    assert.Contains(t, result.Tags, "processed", "Should add processed tag")
}
```

### Common Testify Patterns in This Project

#### File Operations

```go
// File creation and verification
tempDir, err := os.MkdirTemp("", "test")
require.NoError(t, err, "Failed to create temp directory")
defer os.RemoveAll(tempDir)

testFile := filepath.Join(tempDir, "test.txt")
err = os.WriteFile(testFile, []byte("content"), 0644)
require.NoError(t, err, "Failed to create test file")

assert.FileExists(t, testFile, "Test file should exist")
```

#### Error Handling

```go
// Expected success
result, err := operation()
require.NoError(t, err, "Operation should succeed")
assert.NotNil(t, result, "Should return valid result")

// Expected failure
result, err := invalidOperation()
require.Error(t, err, "Operation should fail")
assert.Contains(t, err.Error(), "expected message", "Should have meaningful error")
```

#### Collection Testing

```go
items := []string{"a", "b", "c"}
assert.Len(t, items, 3, "Should have 3 items")
assert.Contains(t, items, "b", "Should contain 'b'")
assert.ElementsMatch(t, []string{"c", "a", "b"}, items, "Should contain same elements")
```

### Migration from Native Assertions

When updating existing tests, replace native assertions as follows:

```go
// OLD: Native assertions
if err != nil {
    t.Fatalf("Operation failed: %v", err)
}
if result != expected {
    t.Errorf("Expected %v, got %v", expected, result)
}

// NEW: Testify assertions
require.NoError(t, err, "Operation should succeed")
assert.Equal(t, expected, result, "Result should match expected value")
```

See [Testify Migration Guide](docs/testify_migration_guide.md) for comprehensive migration instructions.

## Expected Results

### Service Discovery

- Service discovery should complete within 2-5 seconds
- Service offline detection should occur within 10-30 seconds
- No false positives or negatives in service detection

### Transfer Status Management

- Real-time progress updates with <100ms latency
- Accurate transfer rate calculations (within 5% of actual)
- Proper state transitions and error handling
- Event notifications delivered reliably

### Performance

- Transfer rates should approach network bandwidth limits
- Memory usage should scale linearly with file count
- CPU usage should remain reasonable during transfers
- No memory leaks during long-running transfers

### Test Quality with Testify

- All assertions should have descriptive messages
- Tests should be self-documenting through clear assertion messages
- Failure messages should provide actionable debugging information
- Test setup should use `require` to fail fast on critical errors
- Business logic should use `assert` to collect multiple validation failures

## Concurrent Testing Best Practices

### Race Condition Prevention in Tests

When writing concurrent tests, it's crucial to avoid data races, especially when using testing framework functions from multiple goroutines.

#### The Problem: Data Races with \*testing.T

The `*testing.T` type and its methods (like `t.Errorf`, `t.Fatalf`) are **not thread-safe**. Calling these methods from multiple goroutines simultaneously can cause data races and unpredictable test behavior.

```go
// ❌ PROBLEMATIC: Data race with *testing.T
func TestConcurrent_BadExample(t *testing.T) {
    var wg sync.WaitGroup

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()

            // This causes data races!
            assert.NoError(t, someOperation(), "Operation %d failed", id)
            require.True(t, someCondition(), "Condition %d not met", id)
        }(i)
    }

    wg.Wait()
}
```

#### Solution 1: Error Collection with Channels

Collect errors from goroutines using channels and handle them in the main test goroutine:

```go
// ✅ SAFE: Using channels to collect errors
func TestConcurrent_WithChannels(t *testing.T) {
    const numGoroutines = 10
    var wg sync.WaitGroup

    // Buffered channel to collect errors
    errorChan := make(chan error, numGoroutines)

    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()

            // Collect errors instead of asserting directly
            if err := someOperation(); err != nil {
                errorChan <- fmt.Errorf("goroutine %d: operation failed: %w", id, err)
                return
            }

            if !someCondition() {
                errorChan <- fmt.Errorf("goroutine %d: condition not met", id)
                return
            }
        }(i)
    }

    wg.Wait()
    close(errorChan)

    // Handle all errors in the main goroutine (thread-safe)
    for err := range errorChan {
        t.Errorf("Concurrent test error: %v", err)
    }
}
```

#### Solution 2: Parallel Subtests (Recommended)

Use `t.Run` with `t.Parallel()` for idiomatic concurrent testing:

```go
// ✅ RECOMMENDED: Using parallel subtests
func TestConcurrent_WithSubtests(t *testing.T) {
    const numOperations = 10

    t.Run("ConcurrentOperations", func(t *testing.T) {
        for i := 0; i < numOperations; i++ {
            i := i // Capture loop variable
            t.Run(fmt.Sprintf("Operation_%d", i), func(t *testing.T) {
                t.Parallel() // Enable parallel execution

                // Each subtest has its own *testing.T instance - completely safe!
                err := someOperation()
                require.NoError(t, err, "Operation %d should succeed", i)

                result := getResult()
                assert.NotNil(t, result, "Result %d should not be nil", i)
            })
        }
    })
}
```

### Comparison: Channels vs Parallel Subtests

| Feature             | Error Channels | Parallel Subtests |
| ------------------- | -------------- | ----------------- |
| **Thread Safety**   | ✅ Safe        | ✅ Safe           |
| **Go Idioms**       | ⚠️ Acceptable  | ✅ Recommended    |
| **Error Reporting** | ✅ Detailed    | ✅ Very Detailed  |
| **Test Isolation**  | ⚠️ Partial     | ✅ Complete       |
| **Debugging**       | ⚠️ Moderate    | ✅ Excellent      |
| **Code Complexity** | ⚠️ Medium      | ✅ Simple         |

### Real-World Example: File Transfer Manager

Here's how we apply these patterns in the file transfer manager tests:

```go
func TestFileTransferManager_ConcurrentAccess(t *testing.T) {
    ftm := NewFileTransferManager()
    t.Cleanup(func() {
        ftm.Close()
    })

    tempDir := setupTestDir(t)
    const numGoroutines = 10
    var wg sync.WaitGroup

    // Use channels to collect errors from goroutines
    errorChan := make(chan error, numGoroutines*2)

    // Test concurrent additions
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(index int) {
            defer wg.Done()

            fileName := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", index))
            content := fmt.Sprintf("Content for file %d", index)

            if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
                errorChan <- fmt.Errorf("goroutine %d: failed to create file: %w", index, err)
                return
            }

            node, err := fileInfo.CreateNode(fileName)
            if err != nil {
                errorChan <- fmt.Errorf("goroutine %d: failed to create node: %w", index, err)
                return
            }

            err = ftm.AddFileNode(&node)
            if err != nil {
                errorChan <- fmt.Errorf("goroutine %d: AddFileNode failed: %w", index, err)
                return
            }
        }(i)
    }

    wg.Wait()
    close(errorChan)

    // Check for any errors from goroutines (thread-safe)
    for err := range errorChan {
        t.Errorf("Goroutine error: %v", err)
    }

    // Verify results
    assert.GreaterOrEqual(t, len(ftm.chunkers), numGoroutines, "Expected at least %d chunkers", numGoroutines)
}
```

### Testing with Race Detector

Always run concurrent tests with the race detector:

```bash
# Run tests with race detection
go test ./pkg/transfer -race -run TestConcurrent

# Run all tests with race detection
go test ./... -race
```

## Resource Management with t.Cleanup

### The Problem with defer

Traditional `defer` statements can cause resource cleanup issues, especially in tests with multiple resources that have dependencies.

#### Problematic defer Pattern

```go
// ❌ PROBLEMATIC: Wrong cleanup order with defer
func TestProblematic(t *testing.T) {
    tempDir, err := os.MkdirTemp("", "test")
    require.NoError(t, err)
    defer os.RemoveAll(tempDir)  // Called FIRST (wrong!)

    ftm := NewFileTransferManager()
    defer ftm.Close()            // Called SECOND

    // Add files to ftm - file handles are still open
    // Cleanup order: try to remove directory first (fails), then close file handles
}
```

### Solution: t.Cleanup for Proper Resource Management

`t.Cleanup` (available since Go 1.14) provides **LIFO (Last In, First Out)** cleanup order, which is exactly what we need for proper resource management.

#### Correct t.Cleanup Pattern

```go
// ✅ CORRECT: Proper cleanup order with t.Cleanup
func TestCorrect(t *testing.T) {
    tempDir, err := os.MkdirTemp("", "test")
    require.NoError(t, err)

    // Register temp dir cleanup FIRST (will be called LAST)
    t.Cleanup(func() {
        os.RemoveAll(tempDir)
    })

    ftm := NewFileTransferManager()
    // Register manager cleanup SECOND (will be called FIRST)
    t.Cleanup(func() {
        ftm.Close()
    })

    // Cleanup order: close file handles first, then remove directory
}
```

### Benefits of t.Cleanup

1. **Correct LIFO Order**: Resources are cleaned up in reverse order of creation
2. **Subtest Support**: Each subtest gets its own cleanup scope
3. **Error Safety**: Cleanup always runs, even if the test fails
4. **Better Organization**: Cleanup code is near resource creation
5. **No Panic Issues**: Cleanup runs even if the test panics

### Comparison: defer vs t.Cleanup

| Feature             | defer               | t.Cleanup          |
| ------------------- | ------------------- | ------------------ |
| **Cleanup Order**   | Function scope LIFO | Test scope LIFO    |
| **Subtest Support** | ❌ No               | ✅ Yes             |
| **Error Handling**  | ⚠️ Panic may skip   | ✅ Always executes |
| **Readability**     | ⚠️ Scattered        | ✅ Centralized     |
| **Test Failure**    | ⚠️ May not execute  | ✅ Always executes |
| **Go Version**      | All versions        | Go 1.14+           |

### Real-World Example: Cleanup Order Demonstration

```go
func TestFileTransferManager_CleanupOrder(t *testing.T) {
    ftm := NewFileTransferManager()

    // Create temp directory first
    tempDir, err := os.MkdirTemp("", "cleanup-order-test")
    require.NoError(t, err)

    // Register cleanup for temp directory FIRST (called LAST due to LIFO)
    t.Cleanup(func() {
        t.Logf("Step 3: Cleaning up temp directory: %s", tempDir)
        os.RemoveAll(tempDir)
    })

    // Create and add a file
    fileName := filepath.Join(tempDir, "cleanup_test.txt")
    err = os.WriteFile(fileName, []byte("test content"), 0644)
    require.NoError(t, err)

    node, err := fileInfo.CreateNode(fileName)
    require.NoError(t, err)

    err = ftm.AddFileNode(&node)
    require.NoError(t, err)

    // Register cleanup for file transfer manager SECOND (called FIRST due to LIFO)
    t.Cleanup(func() {
        t.Logf("Step 1: Closing FileTransferManager")
        ftm.Close()
    })

    // Register intermediate cleanup to show the order
    t.Cleanup(func() {
        t.Logf("Step 2: Intermediate cleanup step")
    })

    t.Logf("Test body completed, cleanup will now run in LIFO order")
    // Cleanup order:
    // 1. Close FileTransferManager (releases file handles)
    // 2. Intermediate cleanup
    // 3. Remove temp directory (now safe since file handles are closed)
}
```

### Helper Functions with t.Cleanup

Create reusable helper functions that handle their own cleanup:

```go
func setupTestDir(tb testing.TB) string {
    tb.Helper()

    tempDir, err := os.MkdirTemp("", "test")
    require.NoError(tb, err, "Failed to create temp directory")

    // Helper registers its own cleanup
    tb.Cleanup(func() {
        // Robust cleanup with retries for Windows
        var lastErr error
        for i := 0; i < 3; i++ {
            lastErr = os.RemoveAll(tempDir)
            if lastErr == nil {
                return
            }
            if i < 2 {
                time.Sleep(10 * time.Millisecond)
            }
        }
        if lastErr != nil {
            tb.Errorf("Failed to clean up temp dir after retries: %v", lastErr)
        }
    })

    return tempDir
}

func setupFileTransferManager(tb testing.TB) *FileTransferManager {
    tb.Helper()

    ftm := NewFileTransferManager()
    tb.Cleanup(func() {
        ftm.Close()
    })

    return ftm
}
```

### Best Practices for Resource Management

1. **Always use t.Cleanup** for resource management in tests
2. **Register cleanup immediately** after resource creation
3. **Use helper functions** for common setup patterns
4. **Consider dependencies** when registering cleanup functions
5. **Handle cleanup errors** gracefully (log, don't fail)

```go
func TestBestPractices(t *testing.T) {
    // Create resources and register cleanup immediately
    ftm := setupFileTransferManager(t)
    tempDir := setupTestDir(t)

    // Create test file
    fileName := filepath.Join(tempDir, "test.txt")
    err := os.WriteFile(fileName, []byte("content"), 0644)
    require.NoError(t, err, "Failed to create test file")

    // Test logic here...
    node, err := fileInfo.CreateNode(fileName)
    require.NoError(t, err, "Failed to create node")

    err = ftm.AddFileNode(&node)
    require.NoError(t, err, "Failed to add file node")

    // Verify results
    chunker, exists := ftm.GetChunker(fileName)
    assert.True(t, exists, "Chunker should exist")
    assert.NotNil(t, chunker, "Chunker should not be nil")

    // Cleanup happens automatically in correct order:
    // 1. ftm.Close() (from setupFileTransferManager)
    // 2. os.RemoveAll(tempDir) (from setupTestDir)
}
```

### Platform-Specific Considerations

On Windows, file handles can prevent directory deletion. Always ensure file handles are closed before attempting to remove directories:

```go
func setupTestDirWindows(tb testing.TB) string {
    tb.Helper()

    tempDir, err := os.MkdirTemp("", "test")
    require.NoError(tb, err)

    tb.Cleanup(func() {
        // On Windows, retry directory removal with delays
        var lastErr error
        for i := 0; i < 3; i++ {
            lastErr = os.RemoveAll(tempDir)
            if lastErr == nil {
                return
            }
            // Small delay before retry to allow file handles to be released
            time.Sleep(10 * time.Millisecond)
        }
        if lastErr != nil {
            tb.Errorf("Failed to clean up temp dir after retries: %v", lastErr)
        }
    })

    return tempDir
}
```

## Summary

### Concurrent Testing Guidelines

1. **Never call `*testing.T` methods from goroutines** - they are not thread-safe
2. **Use error channels** to collect errors from goroutines
3. **Prefer parallel subtests** (`t.Run` + `t.Parallel()`) for idiomatic concurrent testing
4. **Always run concurrent tests with `-race`** to detect data races
5. **Use buffered channels** with appropriate capacity for error collection

### Resource Management Guidelines

1. **Always use `t.Cleanup`** instead of `defer` for test resource management
2. **Register cleanup immediately** after resource creation
3. **Consider resource dependencies** when ordering cleanup registration
4. **Use helper functions** for common setup/cleanup patterns
5. **Handle platform-specific cleanup issues** (especially Windows file locking)

These practices ensure robust, reliable, and maintainable concurrent tests while preventing resource leaks and cleanup issues.

## Table-Driven Testing Best Practices

### Why Use Table-Driven Tests?

Table-driven tests are a powerful Go testing pattern that provides several advantages:

1. **Better Organization**: All test cases are clearly defined in a structured format
2. **Easy Maintenance**: Adding new test cases is as simple as adding a new entry to the table
3. **Consistent Structure**: All test cases follow the same execution pattern
4. **Better Coverage**: Encourages testing multiple scenarios systematically
5. **Readable Output**: Each test case runs as a separate subtest with clear names

### When to Use Table-Driven Tests

Table-driven tests are particularly effective for:

- **Error handling scenarios** with multiple input variations
- **Validation logic** with different boundary conditions
- **Edge cases** that need systematic coverage
- **Input/output transformations** with multiple examples
- **Configuration testing** with various parameter combinations

### Basic Table-Driven Test Structure

```go
func TestFunction_ErrorHandling(t *testing.T) {
    // Define test cases in a table
    testCases := []struct {
        name          string
        input         InputType
        expectError   bool
        errorContains string
        description   string
    }{
        {
            name:          "valid_input",
            input:         validInput,
            expectError:   false,
            errorContains: "",
            description:   "Should succeed with valid input",
        },
        {
            name:          "invalid_input",
            input:         invalidInput,
            expectError:   true,
            errorContains: "expected error message",
            description:   "Should fail with invalid input",
        },
    }

    // Execute test cases
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            result, err := functionUnderTest(tc.input)

            if tc.expectError {
                require.Error(t, err, tc.description)
                if tc.errorContains != "" {
                    assert.Contains(t, err.Error(), tc.errorContains)
                }
            } else {
                require.NoError(t, err, tc.description)
                // Additional assertions for successful cases
            }
        })
    }
}
```

### Advanced Table-Driven Test Patterns

#### Dynamic Test Setup with Functions

For complex test scenarios, use setup functions to create test data dynamically:

```go
func TestFileTransferManager_ErrorHandling(t *testing.T) {
    ftm := NewFileTransferManager()
    t.Cleanup(func() {
        ftm.Close()
    })

    tempDir := setupTestDir(t)

    testCases := []struct {
        name          string
        setupNode     func() *fileInfo.FileNode  // Dynamic setup
        expectError   bool
        errorContains string
        description   string
    }{
        {
            name: "file_does_not_exist",
            setupNode: func() *fileInfo.FileNode {
                return &fileInfo.FileNode{
                    Name:  "missing.txt",
                    IsDir: false,
                    Size:  100,
                    Path:  filepath.Join(tempDir, "missing.txt"),
                }
            },
            expectError:   true,
            errorContains: "",
            description:   "Should fail when file doesn't exist",
        },
        {
            name: "valid_existing_file",
            setupNode: func() *fileInfo.FileNode {
                fileName := filepath.Join(tempDir, "valid.txt")
                err := os.WriteFile(fileName, []byte("content"), 0644)
                require.NoError(t, err, "Failed to create test file")

                node, err := fileInfo.CreateNode(fileName)
                require.NoError(t, err, "Failed to create node")
                return &node
            },
            expectError:   false,
            errorContains: "",
            description:   "Should succeed with valid file",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            node := tc.setupNode()
            err := ftm.AddFileNode(node)

            if tc.expectError {
                require.Error(t, err, tc.description)
                if tc.errorContains != "" {
                    assert.Contains(t, err.Error(), tc.errorContains)
                }
            } else {
                require.NoError(t, err, tc.description)
            }
        })
    }
}
```

#### Validation and Edge Case Testing

```go
func TestFileTransferManager_ValidationCases(t *testing.T) {
    ftm := NewFileTransferManager()
    t.Cleanup(func() {
        ftm.Close()
    })

    testCases := []struct {
        name          string
        node          *fileInfo.FileNode
        expectError   bool
        errorContains string
        description   string
    }{
        {
            name:          "nil_node",
            node:          nil,
            expectError:   true,
            errorContains: "cannot be nil",
            description:   "Should reject nil node",
        },
        {
            name: "empty_path",
            node: &fileInfo.FileNode{
                Name:  "empty.txt",
                IsDir: false,
                Size:  50,
                Path:  "",
            },
            expectError:   true,
            errorContains: "",
            description:   "Should reject empty path",
        },
        {
            name: "negative_size",
            node: &fileInfo.FileNode{
                Name:  "negative.txt",
                IsDir: false,
                Size:  -100,
                Path:  "/tmp/negative.txt",
            },
            expectError:   true,
            errorContains: "",
            description:   "Should reject negative file size",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            err := ftm.AddFileNode(tc.node)

            if tc.expectError {
                require.Error(t, err, tc.description)
                if tc.errorContains != "" {
                    assert.Contains(t, err.Error(), tc.errorContains)
                }
            } else {
                require.NoError(t, err, tc.description)
            }
        })
    }
}
```

### Best Practices for Table-Driven Tests

#### 1. Use Descriptive Test Case Names

```go
// ✅ Good: Descriptive names
testCases := []struct {
    name string
    // ...
}{
    {name: "nil_input_should_fail"},
    {name: "empty_string_should_succeed"},
    {name: "invalid_format_should_return_error"},
}

// ❌ Bad: Generic names
testCases := []struct {
    name string
    // ...
}{
    {name: "test1"},
    {name: "test2"},
    {name: "error_case"},
}
```

#### 2. Include Meaningful Descriptions

```go
testCases := []struct {
    name        string
    input       string
    expectError bool
    description string  // Always include description
}{
    {
        name:        "valid_email",
        input:       "user@example.com",
        expectError: false,
        description: "Should accept valid email format",
    },
}
```

#### 3. Group Related Test Cases

```go
// Group by functionality
func TestValidator_EmailValidation(t *testing.T) { /* email tests */ }
func TestValidator_PhoneValidation(t *testing.T) { /* phone tests */ }
func TestValidator_ErrorHandling(t *testing.T)   { /* error tests */ }
```

#### 4. Use Subtests for Better Organization

```go
func TestComplexFunction(t *testing.T) {
    t.Run("SuccessCases", func(t *testing.T) {
        // Table-driven tests for success scenarios
    })

    t.Run("ErrorCases", func(t *testing.T) {
        // Table-driven tests for error scenarios
    })

    t.Run("EdgeCases", func(t *testing.T) {
        // Table-driven tests for edge cases
    })
}
```

#### 5. Handle Platform-Specific Behavior

```go
testCases := []struct {
    name          string
    input         string
    expectError   bool
    skipOnWindows bool  // Platform-specific handling
}{
    {
        name:          "unix_path",
        input:         "/tmp/file.txt",
        expectError:   false,
        skipOnWindows: true,
    },
}

for _, tc := range testCases {
    t.Run(tc.name, func(t *testing.T) {
        if tc.skipOnWindows && runtime.GOOS == "windows" {
            t.Skip("Skipping on Windows")
        }
        // Test logic...
    })
}
```

### Refactoring Legacy Tests to Table-Driven

#### Before: Multiple Individual Tests

```go
// ❌ Before: Repetitive individual tests
func TestAddFileNode_NilInput(t *testing.T) {
    ftm := NewFileTransferManager()
    defer ftm.Close()

    err := ftm.AddFileNode(nil)
    assert.Error(t, err, "Expected error with nil input")
}

func TestAddFileNode_NonExistentFile(t *testing.T) {
    ftm := NewFileTransferManager()
    defer ftm.Close()

    node := &fileInfo.FileNode{Path: "/nonexistent"}
    err := ftm.AddFileNode(node)
    assert.Error(t, err, "Expected error with nonexistent file")
}

func TestAddFileNode_EmptyPath(t *testing.T) {
    ftm := NewFileTransferManager()
    defer ftm.Close()

    node := &fileInfo.FileNode{Path: ""}
    err := ftm.AddFileNode(node)
    assert.Error(t, err, "Expected error with empty path")
}
```

#### After: Single Table-Driven Test

```go
// ✅ After: Organized table-driven test
func TestFileTransferManager_AddFileNode_ErrorHandling(t *testing.T) {
    ftm := NewFileTransferManager()
    t.Cleanup(func() {
        ftm.Close()
    })

    testCases := []struct {
        name          string
        node          *fileInfo.FileNode
        expectError   bool
        errorContains string
        description   string
    }{
        {
            name:          "nil_node",
            node:          nil,
            expectError:   true,
            errorContains: "cannot be nil",
            description:   "Should reject nil node",
        },
        {
            name: "non_existent_file",
            node: &fileInfo.FileNode{
                Path: "/nonexistent",
            },
            expectError:   true,
            errorContains: "",
            description:   "Should reject non-existent file",
        },
        {
            name: "empty_path",
            node: &fileInfo.FileNode{
                Path: "",
            },
            expectError:   true,
            errorContains: "",
            description:   "Should reject empty path",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            err := ftm.AddFileNode(tc.node)

            if tc.expectError {
                require.Error(t, err, tc.description)
                if tc.errorContains != "" {
                    assert.Contains(t, err.Error(), tc.errorContains)
                }
            } else {
                require.NoError(t, err, tc.description)
            }
        })
    }
}
```

### Benefits of the Refactoring

1. **Reduced Code Duplication**: Setup code is shared across all test cases
2. **Better Test Organization**: All related tests are in one place
3. **Easier Maintenance**: Adding new test cases requires minimal code changes
4. **Consistent Error Handling**: All test cases follow the same assertion pattern
5. **Better Test Output**: Each case runs as a named subtest for clear reporting
6. **Resource Management**: Single cleanup function handles all test cases

### Running Table-Driven Tests

```bash
# Run all error handling tests
go test ./pkg/transfer -v -run TestFileTransferManager_.*ErrorHandling

# Run specific test case
go test ./pkg/transfer -v -run TestFileTransferManager_AddFileNode_ErrorHandling/nil_node

# Run with race detection
go test ./pkg/transfer -race -run TestFileTransferManager_.*ErrorHandling
```

Table-driven tests make your test suite more maintainable, comprehensive, and easier to understand. They're particularly valuable for testing error conditions, validation logic, and edge cases where you need to verify behavior across multiple input scenarios.
