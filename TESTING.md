# Testing Guide

This document outlines testing procedures for the lanFileSharer project, including service discovery, transfer status management, and end-to-end transfer workflows.

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
