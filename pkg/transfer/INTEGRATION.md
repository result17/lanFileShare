# Transfer System Integration Guide

## Overview

The transfer package contains two complementary systems that work together:

1. **Protocol System** (`protocol.go`, `session.go`) - Network communication layer
2. **Status Management System** (`status.go`) - Transfer state tracking layer

## System Comparison

| Aspect        | Protocol System                | Status Management System             |
| ------------- | ------------------------------ | ------------------------------------ |
| **Purpose**   | Network communication          | State tracking & progress monitoring |
| **Scope**     | Message exchange between peers | Local transfer status management     |
| **Lifetime**  | Per message (transient)        | Per transfer session (persistent)    |
| **Data Flow** | Sender → Network → Receiver    | Internal application state           |

## Key Components

### Protocol System (Existing)

```go
// Network message types
type MessageType string
const (
    TransferBegin    MessageType = "transfer_begin"    // Network event
    TransferCancel   MessageType = "transfer_cancel"   // Network event
    TransferComplete MessageType = "transfer_complete" // Network event
    ProgressUpdate   MessageType = "progress_update"   // Network event
)

// Network session identifier
type TransferSession struct {
    ServiceID       string `json:"service_id"`
    SessionID       string `json:"session_id"`
    SessionCreateAt int64  `json:"session_create_at"`
}
```

### Status Management System (New)

```go
// Local transfer states
type TransferState int
const (
    TransferStatePending   TransferState = iota // Local state
    TransferStateActive                         // Local state
    TransferStateCancelled                      // Local state
    TransferStateCompleted                      // Local state
)

// Local status tracking (no network serialization needed)
type TransferStatus struct {
    FilePath       string
    State          TransferState
    BytesSent      int64
    TransferRate   float64
    // ... progress tracking fields
}
```

## Integration Patterns

### Pattern 1: Message → State Update

```go
// When sending a network message, update local state
func (ftm *FileTransferManager) startTransfer(filePath string) error {
    // 1. Send network message
    message := ChunkMessage{
        Type: TransferBegin,  // Protocol system
        // ...
    }
    err := ftm.sendMessage(message)
    if err != nil {
        return err
    }

    // 2. Update local status
    err = ftm.statusManager.StartTransfer(filePath, totalSize)  // Status system
    if err != nil {
        return err
    }

    return nil
}
```

### Pattern 2: State → Message Generation

```go
// When local state changes, send appropriate network message
func (ftm *FileTransferManager) cancelTransfer(filePath string) error {
    // 1. Update local status
    err := ftm.statusManager.CancelTransfer(filePath)  // Status system
    if err != nil {
        return err
    }

    // 2. Send network message
    message := ChunkMessage{
        Type: TransferCancel,  // Protocol system
        // ...
    }
    return ftm.sendMessage(message)
}
```

### Pattern 3: Progress Synchronization

```go
// During active transfer, sync progress between systems
func (ftm *FileTransferManager) sendChunk(filePath string, chunk []byte) error {
    // 1. Send data over network
    message := ChunkMessage{
        Type: ChunkData,  // Protocol system
        Data: chunk,
        // ...
    }
    err := ftm.sendMessage(message)
    if err != nil {
        return err
    }

    // 2. Update local progress
    err = ftm.statusManager.UpdateProgress(filePath, len(chunk))  // Status system
    if err != nil {
        return err
    }

    // 3. Optionally send progress update message
    if shouldSendProgressUpdate() {
        progressMsg := ChunkMessage{
            Type: ProgressUpdate,  // Protocol system
            // ... include current progress
        }
        ftm.sendMessage(progressMsg)
    }

    return nil
}
```

## Naming Conventions

To avoid confusion between the two systems:

### Protocol System (Network)

- `MessageType` - Types of network messages
- `TransferSession` - Network session identifier
- `ChunkMessage` - Network message structure

### Status System (Local)

- `TransferState` - Local transfer states
- `StatusSessionState` - Local session states (renamed to avoid conflict)
- `TransferStatus` - Local status tracking structure

## Migration Strategy

When integrating the new status management system:

1. **Phase 1**: Keep existing protocol system unchanged
2. **Phase 2**: Add status management alongside existing code
3. **Phase 3**: Integrate both systems using the patterns above
4. **Phase 4**: Add UI integration using status management data

## Example Integration

```go
type FileTransferManager struct {
    // Existing protocol components
    chunkers map[string]*Chunker
    session  *TransferSession  // Protocol system

    // New status management components
    statusManager *TransferStatusManager  // Status system
    config        *TransferConfig
}

func (ftm *FileTransferManager) TransferFile(filePath string) error {
    // Create protocol session
    session := NewTransferSession(ftm.serviceID)

    // Start status tracking
    totalSize := getFileSize(filePath)
    err := ftm.statusManager.StartTransfer(filePath, totalSize)
    if err != nil {
        return err
    }

    // Send begin message
    beginMsg := ChunkMessage{
        Type:    TransferBegin,
        Session: *session,
        // ...
    }
    err = ftm.sendMessage(beginMsg)
    if err != nil {
        ftm.statusManager.FailTransfer(filePath, err)
        return err
    }

    // Transfer chunks with progress updates
    for chunk := range ftm.getChunks(filePath) {
        // Send chunk
        chunkMsg := ChunkMessage{
            Type:    ChunkData,
            Session: *session,
            Data:    chunk,
            // ...
        }
        err = ftm.sendMessage(chunkMsg)
        if err != nil {
            ftm.statusManager.FailTransfer(filePath, err)
            return err
        }

        // Update progress
        ftm.statusManager.UpdateProgress(filePath, len(chunk))
    }

    // Send completion message
    completeMsg := ChunkMessage{
        Type:    TransferComplete,
        Session: *session,
        // ...
    }
    err = ftm.sendMessage(completeMsg)
    if err != nil {
        ftm.statusManager.FailTransfer(filePath, err)
        return err
    }

    // Mark as completed
    ftm.statusManager.CompleteTransfer(filePath)

    return nil
}
```

## Benefits of This Approach

1. **Separation of Concerns**: Network protocol vs. local state management
2. **Backward Compatibility**: Existing protocol system remains unchanged
3. **Enhanced Functionality**: Rich progress tracking and status management
4. **UI Integration**: Status system provides data for user interfaces
5. **Testability**: Each system can be tested independently
6. **Maintainability**: Clear boundaries between network and state logic
