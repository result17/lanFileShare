# Transfer Status Management Design

## Overview

This document outlines the design for implementing comprehensive transfer status management in the lanFileSharer project. The system will provide real-time transfer monitoring, state management, and progress tracking capabilities that integrate seamlessly with the existing file transfer infrastructure.

## Architecture

### High-Level Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   UI Layer      │    │  Transfer        │    │  File Transfer  │
│                 │◄──►│  Status Manager  │◄──►│  Manager        │
│ - Progress Bars │    │                  │    │                 │
│ - Status Display│    │ - Status Tracking│    │ - Chunking      │
│ - Controls      │    │ - Event Emission │    │ - Transmission  │
└─────────────────┘    │ - State Mgmt     │    │ - Connection    │
                       └──────────────────┘    └─────────────────┘
                                │
                       ┌──────────────────┐
                       │  Transfer        │
                       │  History Store   │
                       │                  │
                       │ - Persistence    │
                       │ - Querying       │
                       │ - Cleanup        │
                       └──────────────────┘
```

### Core Components

#### 1. TransferStatusManager

Central component responsible for:

- Maintaining transfer status for all active transfers
- Calculating progress metrics and statistics
- Managing transfer state transitions
- Emitting status change events
- Coordinating with FileTransferManager

#### 2. TransferStatus

Data structure representing the current state of a file transfer:

- Progress information (bytes sent, total size, percentage)
- State information (pending, active, paused, completed, failed)
- Performance metrics (transfer rate, ETA)
- Error information and retry counts
- Timestamps for lifecycle events

#### 3. TransferSession

Container for managing multiple related file transfers:

- Session-level progress aggregation
- Concurrent transfer coordination
- Session state management
- Resource allocation and limits

#### 4. StatusEventEmitter

Event system for real-time status updates:

- Observer pattern implementation
- Type-safe event definitions
- Asynchronous event delivery
- Error handling for failed deliveries

## Components and Interfaces

### TransferStatusManager Interface

```go
type TransferStatusManager interface {
    // Transfer lifecycle management
    StartTransfer(filePath string, totalSize int64) (*TransferStatus, error)
    UpdateProgress(filePath string, bytesSent int64) error
    CompleteTransfer(filePath string) error
    FailTransfer(filePath string, err error) error
    CancelTransfer(filePath string) error

    // State management
    PauseTransfer(filePath string) error
    ResumeTransfer(filePath string) error

    // Status querying
    GetTransferStatus(filePath string) (*TransferStatus, error)
    GetAllTransfers() ([]*TransferStatus, error)
    GetOverallProgress() (*OverallProgress, error)

    // Session management
    CreateSession(sessionID string) (*TransferSession, error)
    GetSession(sessionID string) (*TransferSession, error)
    CloseSession(sessionID string) error

    // Event management
    Subscribe(listener StatusEventListener) error
    Unsubscribe(listener StatusEventListener) error

    // History and persistence
    GetTransferHistory(filter *HistoryFilter) ([]*TransferRecord, error)
    CleanupHistory(olderThan time.Time) error
}
```

### TransferStatus Data Structure

```go
type TransferStatus struct {
    FilePath        string
    SessionID       string
    State           TransferState

    // Progress information
    BytesSent       int64
    TotalBytes      int64
    ChunksSent      int
    TotalChunks     int

    // Performance metrics
    TransferRate    float64  // bytes per second
    ETA             time.Duration

    // Lifecycle timestamps
    StartTime       time.Time
    LastUpdateTime  time.Time
    CompletionTime  *time.Time

    // Error handling
    LastError       error
    RetryCount      int
    MaxRetries      int

    // Metadata
    FileSize        int64
    FileChecksum    string
    Priority        int
}

type TransferState int
const (
    TransferStatePending TransferState = iota
    TransferStateActive
    TransferStatePaused
    TransferStateCompleted
    TransferStateFailed
    TransferStateCancelled
)
```

### TransferSession Management

```go
type TransferSession struct {
    SessionID       string
    CreatedAt       time.Time
    UpdatedAt       time.Time

    // Session configuration
    MaxConcurrent   int
    RetryPolicy     *RetryPolicy
    Priority        int

    // Transfer tracking
    Transfers       map[string]*TransferStatus
    ActiveCount     int
    CompletedCount  int
    FailedCount     int

    // Aggregated metrics
    TotalBytes      int64
    BytesSent       int64
    OverallRate     float64
    EstimatedETA    time.Duration

    // State
    State           SessionState
    LastError       error
}

type SessionState int
const (
    SessionStateActive SessionState = iota
    SessionStatePaused
    SessionStateCompleted
    SessionStateFailed
    SessionStateCancelled
)
```

### Event System Design

```go
type StatusEvent struct {
    Type        EventType
    Timestamp   time.Time
    FilePath    string
    SessionID   string
    OldStatus   *TransferStatus
    NewStatus   *TransferStatus
    Error       error
}

type EventType int
const (
    EventTransferStarted EventType = iota
    EventTransferProgress
    EventTransferPaused
    EventTransferResumed
    EventTransferCompleted
    EventTransferFailed
    EventTransferCancelled
    EventSessionCompleted
)

type StatusEventListener interface {
    OnStatusEvent(event *StatusEvent) error
}
```

## Data Models

### Transfer History Schema

```go
type TransferRecord struct {
    ID              string
    SessionID       string
    FilePath        string
    FileName        string
    FileSize        int64
    FileChecksum    string

    // Transfer details
    StartTime       time.Time
    EndTime         time.Time
    Duration        time.Duration
    BytesTransferred int64
    AverageRate     float64

    // Final state
    FinalState      TransferState
    ErrorMessage    string
    RetryCount      int

    // Metadata
    RecipientID     string
    TransferMethod  string
    CreatedAt       time.Time
}
```

### Configuration Schema

```go
type TransferConfig struct {
    // Concurrency limits
    MaxConcurrentTransfers int
    MaxConcurrentChunks    int

    // Performance settings
    ChunkSize              int32
    BufferSize             int
    RateCalculationWindow  time.Duration

    // Retry policy
    DefaultRetryPolicy     *RetryPolicy

    // History settings
    HistoryRetentionDays   int
    MaxHistoryRecords      int

    // Event settings
    EventBufferSize        int
    EventDeliveryTimeout   time.Duration
}

type RetryPolicy struct {
    MaxRetries      int
    InitialDelay    time.Duration
    BackoffFactor   float64
    MaxDelay        time.Duration
    RetryableErrors []error
}
```

## Error Handling

### Error Categories

1. **Recoverable Errors**

   - Network timeouts
   - Temporary connection issues
   - Rate limiting
   - Temporary file access issues

2. **Non-Recoverable Errors**

   - File not found
   - Permission denied
   - Disk full
   - Invalid file format
   - Authentication failures

3. **System Errors**
   - Out of memory
   - Database connection failures
   - Configuration errors

### Error Handling Strategy

```go
type ErrorHandler interface {
    HandleError(filePath string, err error) ErrorAction
    IsRetryable(err error) bool
    GetRetryDelay(retryCount int, policy *RetryPolicy) time.Duration
}

type ErrorAction int
const (
    ErrorActionRetry ErrorAction = iota
    ErrorActionFail
    ErrorActionPause
    ErrorActionCancel
)
```

## Testing Strategy

### Unit Testing

- Test individual components in isolation
- Mock dependencies for focused testing
- Test error conditions and edge cases
- Verify state transitions and data consistency

### Integration Testing

- Test component interactions
- Verify event flow and delivery
- Test concurrent operations
- Validate persistence and recovery

### Performance Testing

- Test with large numbers of concurrent transfers
- Measure memory usage and resource consumption
- Test transfer rate calculations and accuracy
- Validate system behavior under load

### End-to-End Testing

- Test complete transfer workflows
- Verify UI integration and real-time updates
- Test error recovery and retry mechanisms
- Validate transfer history and persistence

## Implementation Phases

### Phase 1: Core Status Management (Week 1)

- Implement TransferStatus data structure
- Create basic TransferStatusManager
- Add progress tracking and state management
- Implement basic event emission

### Phase 2: Session Management (Week 1-2)

- Implement TransferSession management
- Add concurrent transfer coordination
- Implement overall progress calculation
- Add session-level state management

### Phase 3: Event System (Week 2)

- Implement robust event emission system
- Add event listener management
- Implement asynchronous event delivery
- Add error handling for event failures

### Phase 4: Persistence and History (Week 2-3)

- Implement transfer history storage
- Add history querying and filtering
- Implement history cleanup mechanisms
- Add configuration management

### Phase 5: Error Handling and Recovery (Week 3)

- Implement comprehensive error handling
- Add retry mechanisms with exponential backoff
- Implement transfer recovery after failures
- Add error categorization and routing

### Phase 6: Performance Optimization (Week 3-4)

- Optimize for high-throughput scenarios
- Implement efficient progress calculations
- Add memory usage optimization
- Implement performance monitoring

## Integration Points

### FileTransferManager Integration

- Modify FileTransferManager to use TransferStatusManager
- Add status update calls during chunk transmission
- Integrate error handling with status management
- Coordinate transfer lifecycle events

### WebRTC Connection Integration

- Add connection state monitoring
- Integrate network performance metrics
- Handle connection failures and recovery
- Monitor data channel statistics

### UI Integration

- Provide real-time status updates to UI components
- Support progress bar and status display updates
- Handle user control actions (pause, resume, cancel)
- Display transfer history and statistics

### API Integration

- Expose transfer status through REST API
- Support remote monitoring and control
- Integrate with authentication and authorization
- Provide transfer statistics and reporting

## Security Considerations

### Data Protection

- Ensure transfer status doesn't leak sensitive file information
- Protect transfer history from unauthorized access
- Implement secure event delivery mechanisms
- Validate all input parameters and file paths

### Resource Protection

- Implement rate limiting for status queries
- Prevent resource exhaustion from excessive transfers
- Protect against denial of service attacks
- Implement proper cleanup of abandoned transfers

### Privacy Considerations

- Allow users to control transfer history retention
- Provide options to disable transfer logging
- Ensure transfer metadata doesn't expose private information
- Implement secure deletion of transfer records
