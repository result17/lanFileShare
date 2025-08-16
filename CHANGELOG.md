# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Stateless File Transfer Architecture**: Refactored sender application to use stateless file preparation for improved reliability and concurrency

  - **Per-Transfer FileStructureManager**: Each file transfer now creates its own `FileStructureManager` instance instead of sharing a global one
  - **Eliminated State Pollution**: Removed shared mutable state between transfers, preventing interference between concurrent or sequential transfers
  - **Memory Management**: FileStructureManager instances are automatically garbage collected after transfer completion
  - **Concurrency Safety**: Removed mutex locks and shared state, enabling true concurrent file transfers
  - **Test Isolation**: Each transfer operates in complete isolation, making testing more reliable and predictable

- **Out-of-Order Chunk Writing**: Implemented offset-based chunk writing system to eliminate memory accumulation issues
  - **Offset Support**: Added `Offset` field to `ChunkMessage` and `Chunk` structures to support direct file positioning
  - **Zero Memory Accumulation**: Chunks are now written directly to disk at their correct file offset, eliminating the need for in-memory buffering
  - **Concurrent Write Safety**: Added `sync.RWMutex` protection for concurrent chunk writes to the same file
  - **Duplicate Detection**: Implemented chunk deduplication using `ReceivedChunks map[uint32]bool` to track received sequences
  - **Network Resilience**: File transfer now works efficiently regardless of chunk arrival order, improving performance on unreliable networks

### Changed

- **Sender Application Architecture**: Redesigned sender app to eliminate stateful file management

  - **Removed**: `fileStructure *transfer.FileStructureManager` field from App struct
  - **Removed**: `structureMu sync.RWMutex` mutex for protecting shared file structure
  - **Replaced**: `PrepareFiles()` method with stateless `prepareFilesForTransfer()` function
  - **Removed**: `GetFileStructure()` method as it's no longer needed in stateless design
  - **Enhanced**: `StartSendProcess()` now creates and manages its own FileStructureManager instance

- **File Reception Architecture**: Completely refactored chunk processing from sequential buffering to offset-based direct writes
  - **Removed**: `Chunks map[uint32][]byte` memory cache that could cause unlimited memory growth
  - **Added**: `ReceivedChunks map[uint32]bool` for lightweight chunk tracking
  - **Improved**: `writeChunkAtOffset()` method replaces `writeSequentialChunks()` for better performance
- **Chunk Generation**: Enhanced chunker to automatically calculate file offsets during chunk creation
- **Protocol Enhancement**: Extended JSON serialization to support the new offset field
- **Test Coverage**: Updated all test cases to use offset-based chunk messages

### Technical Details

- **Sender Architecture Changes**:

  - **Stateless Design**: Each `StartSendProcess()` call creates its own FileStructureManager instance
  - **Memory Management**: FileStructureManager is scoped to individual transfers and automatically cleaned up
  - **API Compatibility**: Public API remains unchanged, ensuring backward compatibility
  - **Performance**: Minimal overhead from object creation, significant gains from eliminated lock contention
  - **Thread Safety**: Achieved through isolation rather than synchronization primitives

- **Receiver Architecture Changes**:
  - **Memory Impact**: Eliminated potential out-of-memory issues with large files or poor network conditions
  - **Performance**: Chunks can now be processed immediately upon arrival, reducing latency
  - **Concurrency**: Multiple chunks can be written simultaneously to different file positions
  - **Reliability**: Added file sync operations to ensure data integrity
  - **Backward Compatibility**: Protocol changes are additive and maintain compatibility

### Architecture Benefits

- **Sender-Side Improvements**:

  - **True Concurrency**: Multiple file transfers can now run simultaneously without state conflicts
  - **Memory Efficiency**: FileStructureManager instances are created on-demand and garbage collected automatically
  - **Error Isolation**: Failures in one transfer don't affect other transfers due to complete state isolation
  - **Simplified Testing**: Each test can run independently without worrying about shared state cleanup
  - **Reduced Complexity**: Eliminated need for mutex locks and complex state management

- **Receiver-Side Improvements**:
  - **Scalability**: No longer limited by available memory for chunk buffering
  - **Network Adaptability**: Handles packet reordering, loss, and retransmission gracefully
  - **Concurrent Processing**: Supports parallel chunk processing for improved throughput
  - **Resource Efficiency**: Minimal memory footprint regardless of file size or network conditions

### Fixed

- **Critical Data Race Issues**: Fixed multiple data race conditions that were causing test failures in CI/CD pipeline
  - **UnifiedTransferManager Race Conditions**: Fixed race conditions in status notification system where multiple goroutines were accessing shared state concurrently
    - Modified `UpdateProgress`, `CompleteTransfer`, `StartTransfer`, `FailTransfer`, `PauseTransfer`, and `ResumeTransfer` methods to create copies of status objects before passing to notification goroutines
    - This prevents concurrent access to the same memory locations that could cause data corruption or crashes
  - **Test Infrastructure Race Conditions**: Fixed race conditions in `testStatusListener` used in unit tests
    - Added `sync.Mutex` to protect concurrent access to event slices (`fileEvents`, `sessionEvents`)
    - Implemented thread-safe accessor methods: `GetFileEvents()`, `GetSessionEvents()`, `GetFileEventCount()`, `GetSessionEventCount()`
  - **FileTransferManager Concurrent Access**: Fixed race condition in file count checking
    - Added proper read lock protection when checking file limits in `AddFileNode` method
  - **Impact**: All tests now pass with `-race` flag enabled, ensuring thread safety for production use

### Technical Details

- **Root Cause**: The primary issue was passing pointers to shared mutable state to goroutines for asynchronous notifications
- **Solution Strategy**: Create deep copies of state objects before passing to goroutines, ensuring each goroutine has its own copy of the data
- **Testing**: All 60+ tests in transfer package now pass with race detection enabled
- **Performance Impact**: Minimal - copying small status structures is negligible compared to file transfer operations

### Architecture Improvements

- Enhanced thread safety across the entire transfer management system
- Improved reliability of the event notification system
- Better separation of concerns between state management and event notification
- Maintained backward compatibility while fixing concurrency issues

---

## Previous Changes

### [Major Refactoring] - Transfer Package Architecture Overhaul

- **Unified Architecture**: Consolidated 3 separate managers (FileTransferManager, FileStructureManager, TransferStatusManager) into a cohesive system
- **Single Session Design**: Simplified from multi-session to single-session architecture matching actual usage patterns
- **Performance Optimization**: Reduced file count by 25%, eliminated code duplication
- **API Simplification**: Unified interface design with consistent patterns
- **Enhanced Features**: Added file folder support, real-time progress tracking, comprehensive error handling
- **Test Coverage**: Comprehensive test suite with 74.9% coverage
- **Documentation**: Complete API documentation and usage examples

### Core Components

- **UnifiedTransferManager**: Central file transfer coordination
- **TransferStatusManager**: Session-level status tracking
- **FileStructureManager**: File organization and fast lookup
- **Event System**: Real-time status notifications
- **Configuration System**: Unified transfer settings

### Benefits

- **Developer Experience**: Simpler API, better documentation
- **Reliability**: Thread-safe operations, comprehensive error handling
- **Performance**: Optimized memory usage, faster file operations
- **Maintainability**: Clear separation of concerns, reduced complexity
