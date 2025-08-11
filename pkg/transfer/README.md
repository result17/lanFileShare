# Transfer Package

The transfer package provides unified file transfer management with chunking, status tracking, and progress monitoring.

## Architecture Overview

After refactoring, the package now has a clean, unified architecture:

```
┌─────────────────────────────────────────────────────────┐
│                UnifiedTransferManager                   │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────┐   │
│  │   Files     │ │   Queue     │ │     Events      │   │
│  │ Management  │ │  Management │ │  Notification   │   │
│  └─────────────┘ └─────────────┘ └─────────────────┘   │
└─────────────────────────────────────────────────────────┘
           │                │                │
    ┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐
    │   Chunker   │  │TransferStatus│  │StatusListener│
    │  (chunking) │  │ Manager      │  │  (events)   │
    └─────────────┘  └─────────────┘  └─────────────┘
                            │
                     ┌──────▼──────┐
                     │SessionTransfer│
                     │   Status     │
                     └─────────────┘
```

## Core Components

### 1. UnifiedTransferManager

The main component that manages file transfers:

- **File Management**: Handles file nodes and chunkers
- **Queue Management**: Manages pending, completed, and failed files
- **Status Integration**: Coordinates with TransferStatusManager
- **Event System**: Notifies listeners of status changes

### 2. TransferStatusManager

Manages session-level status tracking:

- **Single Session**: Optimized for single-session transfers
- **Current File Tracking**: Tracks the currently transferring file
- **Session Progress**: Provides overall session progress
- **Event Emission**: Notifies listeners of status changes

### 3. SessionTransferStatus

Represents the status of an entire transfer session:

- **Session Info**: Session ID and timing information
- **File Counts**: Total, completed, failed, and pending files
- **Progress Tracking**: Overall progress and bytes transferred
- **Current File**: Status of the currently transferring file

### 4. TransferStatus

Represents the status of an individual file transfer:

- **Progress Info**: Bytes sent, total bytes, percentage complete
- **Performance Metrics**: Transfer rate, ETA calculations
- **State Management**: Transfer state and transitions
- **Error Handling**: Error information and retry counts

## Key Features

### ✅ **Unified API**

```go
// Single manager for all transfer operations
manager := transfer.NewUnifiedTransferManager("service-id")

// Add files
manager.AddFile(fileNode)

// Control transfers
manager.StartTransfer(filePath)
manager.PauseTransfer(filePath)
manager.UpdateProgress(filePath, bytes)

// Get status
sessionStatus := manager.GetSessionStatus()
fileStatus, _ := manager.GetFileStatus(filePath)
```

### ✅ **Session-Level Status Tracking**

```go
// Session-level: Overall progress for UI
sessionStatus := manager.GetSessionStatus()
fmt.Printf("Overall: %.1f%% (%d/%d files)",
    sessionStatus.OverallProgress,
    sessionStatus.CompletedFiles,
    sessionStatus.TotalFiles)

// Current file: Detailed progress
if sessionStatus.CurrentFile != nil {
    fmt.Printf("Current: %s (%.1f%%)",
        sessionStatus.CurrentFile.FilePath,
        sessionStatus.CurrentFile.GetProgressPercentage())
}
```

### ✅ **Event System**

```go
type MyStatusListener struct{}

func (l *MyStatusListener) OnFileStatusChanged(filePath string, old, new *TransferStatus) {
    fmt.Printf("File %s: %s -> %s", filePath, old.State, new.State)
}

func (l *MyStatusListener) OnSessionStatusChanged(old, new *SessionTransferStatus) {
    fmt.Printf("Session: %.1f%% -> %.1f%%", old.OverallProgress, new.OverallProgress)
}

manager.AddStatusListener(&MyStatusListener{})
```

### ✅ **Queue Management**

```go
// Check queue status
pending, completed, failed := manager.GetQueueStatus()

// Get next file to transfer
nextFile, hasNext := manager.GetNextPendingFile()

// Mark file as completed/failed
manager.MarkFileCompleted(filePath)
manager.MarkFileFailed(filePath)
```

## File Structure

```
pkg/transfer/
├── chunker.go              # File chunking functionality
├── chunker_test.go         # Chunker tests
├── config.go               # Unified configuration
├── config_test.go          # Configuration tests
├── json.go                 # JSON serialization support
├── protocol.go             # Network protocol definitions
├── protocol_test.go        # Protocol tests
├── session.go              # Transfer session identification
├── status.go               # Status data structures
├── status_test.go          # Status tests
├── status_manager.go       # Session status manager
├── status_manager_test.go  # Status manager tests
├── unified_manager.go      # Main unified manager
├── unified_manager_test.go # Unified manager tests
└── README.md               # This file
```

## Migration Guide

### From Old Architecture

The package has been refactored from multiple fragmented managers to a unified architecture:

**Before (Removed)**:

- `FileTransferManager` - File and chunker management
- `FileStructureManager` - File structure management
- `TransferStatusManager` (old) - Multi-transfer status management

**After (Current)**:

- `UnifiedTransferManager` - File, queue, and coordination management
- `TransferStatusManager` (new) - Single-session status management
- `SessionTransferStatus` - Session-level status tracking

### API Changes

```go
// Old API (removed)
ftm := transfer.NewFileTransferManager()
tsm := transfer.NewTransferStatusManager()
fsm := transfer.NewFileStructureManager()

// New API (current)
manager := transfer.NewUnifiedTransferManager("service-id")
// All functionality is now unified in one manager
```

## Benefits of Refactoring

### 🎯 **Eliminated Redundancy**

- **Before**: 3 separate managers with overlapping responsibilities
- **After**: 1 unified manager with clear separation of concerns

### 📦 **Reduced Complexity**

- **Before**: 20+ files with multiple conflicting implementations
- **After**: 12 core files with unified functionality
- **Removed**: 8 redundant/conflicting files

### 🔧 **Improved Maintainability**

- Single source of truth for file transfer operations
- Consistent API across all transfer operations
- Simplified testing and debugging

### 🚀 **Better Performance**

- Reduced memory overhead from duplicate data structures
- More efficient status updates
- Streamlined event notifications

### 📋 **Business Logic Alignment**

- Optimized for single-session transfers (typical use case)
- Sequential file processing within sessions
- Simplified queue management

## Usage Examples

### Basic File Transfer

```go
manager := transfer.NewUnifiedTransferManager("my-app")

// Add file
node, _ := fileInfo.CreateNode("/path/to/file.txt")
manager.AddFile(&node)

// Start transfer
manager.StartTransfer("/path/to/file.txt")

// Update progress (called during actual transfer)
manager.UpdateProgress("/path/to/file.txt", 1024)

// Complete transfer
manager.CompleteTransfer("/path/to/file.txt")

// Check status
sessionStatus := manager.GetSessionStatus()
fmt.Printf("Progress: %.1f%%", sessionStatus.OverallProgress)
```

### Multiple Files with Session Tracking

```go
manager := transfer.NewUnifiedTransferManager("batch-transfer")

// Add multiple files
files := []string{"/doc1.pdf", "/doc2.txt", "/image.jpg"}
for _, filePath := range files {
    node, _ := fileInfo.CreateNode(filePath)
    manager.AddFile(&node)
}

// Files will be processed sequentially
// Monitor overall progress
sessionStatus := manager.GetSessionStatus()
fmt.Printf("Transferring %d files: %.1f%% complete",
    sessionStatus.TotalFiles,
    sessionStatus.OverallProgress)
```

### Event-Driven UI Updates

```go
type UIUpdater struct {
    progressBar *ProgressBar
    fileList    *FileList
}

func (ui *UIUpdater) OnFileStatusChanged(filePath string, old, new *TransferStatus) {
    ui.fileList.UpdateFile(filePath, new.GetProgressPercentage())
}

func (ui *UIUpdater) OnSessionStatusChanged(old, new *SessionTransferStatus) {
    ui.progressBar.SetProgress(new.OverallProgress)
}

manager.AddStatusListener(&UIUpdater{progressBar, fileList})
```

## Testing

Run all tests:

```bash
go test ./pkg/transfer -v
```

Run specific component tests:

```bash
go test ./pkg/transfer -run TestUnifiedTransferManager -v
go test ./pkg/transfer -run TestTransferStatusManager -v
go test ./pkg/transfer -run TestTransferConfig -v
```

## Configuration

The package uses a unified configuration system:

```go
config := transfer.DefaultTransferConfig()
config.ChunkSize = 64 * 1024  // 64KB chunks
config.MaxConcurrentTransfers = 1  // Sequential processing

manager := transfer.NewUnifiedTransferManagerWithConfig("service-id", config)
```

## Error Handling

The package provides comprehensive error handling:

```go
// Transfer errors
if err := manager.StartTransfer(filePath); err != nil {
    if errors.Is(err, transfer.ErrTransferNotFound) {
        // Handle file not found
    }
}

// Status errors
status, err := manager.GetSessionStatus()
if err != nil {
    if errors.Is(err, transfer.ErrSessionNotFound) {
        // Handle session not initialized
    }
}
```

## Thread Safety

All components are thread-safe and can be used concurrently:

- `UnifiedTransferManager` uses RWMutex for file and queue operations
- `TransferStatusManager` uses RWMutex for status operations
- Event delivery is asynchronous and non-blocking
- All public methods are safe for concurrent use
