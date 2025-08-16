# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
