# lanFileSharer

A tool for sharing files directly between devices on a local network with comprehensive transfer status management and real-time progress tracking.

## How It Works

The connection process is designed to be robust and efficient, combining direct IP communication with the flexibility of mDNS hostname resolution. It works in two main phases:

### Phase 1: Discovery and Initial Handshake (DNS-SD + HTTP)

1.  **Service Discovery**: When a receiver application starts, it broadcasts its presence on the local network using mDNS/DNS-SD.
2.  **IP Resolution**: A sender application discovers the receiver via this broadcast and resolves its `.local` hostname to a specific IP address (e.g., `192.168.1.55`).
3.  **HTTP Handshake**: The sender then initiates a direct HTTP connection to the receiver using the discovered IP address. This connection is used as the primary signaling channel to exchange essential metadata, such as the structure of the files to be transferred and the initial WebRTC session information (SDP Offer/Answer).

### Phase 2: High-Speed Data Transfer (WebRTC)

1.  **PeerConnection Establishment**: Once the handshake is complete, both devices proceed to establish a WebRTC `PeerConnection`.
2.  **P2P Transfer**: This connection is used for the actual high-speed, peer-to-peer transfer of file data, leveraging the performance of WebRTC's data channels.

### Robustness Through `SetMulticastDNSMode`

To make the connection process resilient against potential IP address changes (e.g., due to DHCP lease renewals), the underlying WebRTC `SettingEngine` is configured with `MulticastDNSModeEnabled`.

- **What it does**: This setting empowers the ICE agent within WebRTC to resolve `.local` hostnames directly during the connectivity check phase.
- **Why it's important**: While the initial connection uses a pre-resolved IP, the ICE negotiation includes candidates with `.local` hostnames. If the initial IP has become stale or invalid, the ICE agent can use mDNS to re-resolve the hostname to the device's current IP address. This acts as a powerful fallback mechanism, ensuring a connection can still be established, making the application significantly more robust.

This dual approach leverages the speed of direct IP communication for the initial handshake while retaining the flexibility and resilience of mDNS for the underlying P2P data connection.

## Transfer Management Architecture

The application features a unified transfer management system that provides real-time status tracking and session management:

### Core Components

- **UnifiedTransferManager**: Manages file queues, chunkers, and coordinates with session status tracking
- **TransferStatusManager**: Handles session-level status tracking with support for single-session transfers
- **SessionTransferStatus**: Tracks overall session progress and current file transfer status
- **TransferStatus**: Detailed status information for individual file transfers

### Key Features

- **Real-time Progress Tracking**: Monitor transfer progress with bytes sent, transfer rates, and ETA calculations
- **Session Management**: Single-session architecture optimized for typical use cases
- **Event System**: Real-time notifications for status changes and transfer events
- **Error Handling**: Comprehensive error handling with retry mechanisms and failure recovery
- **State Management**: Support for pause/resume operations and transfer lifecycle management

## Architecture Refactoring Summary

The transfer management system has been significantly refactored to provide a cleaner, more maintainable architecture:

### 🎯 **Before vs After**

| Aspect              | Before (Old Architecture)                                                              | After (New Architecture)                                      |
| ------------------- | -------------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| **Managers**        | 3 separate managers (FileTransferManager, FileStructureManager, TransferStatusManager) | 1 unified manager (UnifiedTransferManager) + 1 status manager |
| **File Count**      | 20+ files with overlapping functionality                                               | 12 core files with clear responsibilities                     |
| **Session Model**   | Multi-session complexity                                                               | Single-session optimization                                   |
| **API Complexity**  | Multiple APIs with different patterns                                                  | Unified, consistent API                                       |
| **Status Tracking** | Per-file status management                                                             | Session-level with current file tracking                      |
| **Memory Usage**    | Duplicate data structures                                                              | Optimized single structures                                   |

### 🏗️ **New Architecture Benefits**

#### **1. Simplified Design**

- **Single Session Focus**: Optimized for the common use case of transferring files in one session
- **Sequential Processing**: Files are processed one at a time within a session
- **Unified API**: All transfer operations through one consistent interface

#### **2. Better Performance**

- **Reduced Memory Overhead**: Eliminated duplicate data structures
- **Efficient Status Updates**: Session-level tracking with current file details
- **Streamlined Events**: Direct method-based notifications instead of complex event objects

#### **3. Improved Maintainability**

- **Clear Separation of Concerns**: UnifiedTransferManager handles files/queues, TransferStatusManager handles status
- **Consistent Error Handling**: Unified error handling patterns across all components
- **Simplified Testing**: Fewer components with clearer responsibilities

#### **4. Business Logic Alignment**

- **Real-world Usage**: Matches how users actually transfer files (one session at a time)
- **Simplified Queue Management**: Pending → Active → Completed/Failed flow
- **Intuitive Status Model**: Overall session progress + current file details

### 📊 **Migration Impact**

The refactoring maintains backward compatibility while providing these improvements:

```go
// Simple, unified API
manager := transfer.NewUnifiedTransferManager("session-id")

// Add files to session
manager.AddFile(fileNode)

// Monitor session progress
sessionStatus := manager.GetSessionStatus()
fmt.Printf("Session: %.1f%% (%d/%d files)",
    sessionStatus.OverallProgress,
    sessionStatus.CompletedFiles,
    sessionStatus.TotalFiles)

// Monitor current file
if sessionStatus.CurrentFile != nil {
    fmt.Printf("Current: %s (%.1f%%)",
        sessionStatus.CurrentFile.FilePath,
        sessionStatus.CurrentFile.GetProgressPercentage())
}
```

### 🔧 **Technical Improvements**

- **Thread Safety**: All components are thread-safe with proper mutex usage
- **Event System**: Asynchronous, non-blocking event delivery
- **Error Recovery**: Comprehensive error handling with retry mechanisms
- **Configuration**: Unified configuration system across all components
- **Testing**: Comprehensive test coverage for all new components
