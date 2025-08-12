package transfer

import (
	"fmt"
	"sync"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// UnifiedTransferManager combines file structure management with transfer control
// It uses FileStructureManager for file organization and provides transfer management
type UnifiedTransferManager struct {
	// Core components
	session   *TransferSession        // Session management
	config    *TransferConfig         // Configuration
	structure *FileStructureManager   // File structure organization
	
	// Chunking management
	chunkers map[string]*Chunker      // File path -> Chunker
	filesMu  sync.RWMutex
	
	// Transfer queue management
	pendingFiles   []string // Files waiting to be transferred
	completedFiles []string // Files that have been transferred
	failedFiles    []string // Files that failed transfer
	queueMu        sync.RWMutex
	
	// Session status tracking
	sessionStatus *SessionTransferStatus
	statusMu      sync.RWMutex
	
	// Event system
	listeners []StatusListener
	eventsMu  sync.RWMutex
}

// ManagedFile is no longer needed since we use FileStructureManager
// and separate chunkers map



// StatusListener interface for transfer status events
type StatusListener interface {
	// OnFileStatusChanged is called when an individual file's status changes
	OnFileStatusChanged(filePath string, oldStatus, newStatus *TransferStatus)
	
	// OnSessionStatusChanged is called when the overall session status changes
	OnSessionStatusChanged(oldStatus, newStatus *SessionTransferStatus)
}

// NewUnifiedTransferManager creates a new unified manager
func NewUnifiedTransferManager(serviceID string) *UnifiedTransferManager {
	return NewUnifiedTransferManagerWithConfig(serviceID, DefaultTransferConfig())
}

// NewUnifiedTransferManagerWithConfig creates a manager with custom config
func NewUnifiedTransferManagerWithConfig(serviceID string, config *TransferConfig) *UnifiedTransferManager {
	session := NewTransferSession(serviceID)
	
	// Initialize session status
	sessionStatus := &SessionTransferStatus{
		SessionID:       serviceID,
		TotalFiles:      0,
		CompletedFiles:  0,
		FailedFiles:     0,
		PendingFiles:    0,
		TotalBytes:      0,
		BytesCompleted:  0,
		OverallProgress: 0.0,
		StartTime:       time.Now(),
		LastUpdateTime:  time.Now(),
		State:           StatusSessionStateActive,
	}
	
	return &UnifiedTransferManager{
		session:        session,
		config:         config,
		structure:      NewFileStructureManager(),
		chunkers:       make(map[string]*Chunker),
		pendingFiles:   make([]string, 0),
		completedFiles: make([]string, 0),
		failedFiles:    make([]string, 0),
		sessionStatus:  sessionStatus,
		listeners:      make([]StatusListener, 0),
	}
}

// AddFile adds a file to the transfer queue (focused on file management only)
// AddFile adds a file or directory to the transfer queue
func (utm *UnifiedTransferManager) AddFile(node *fileInfo.FileNode) error {
	if node == nil {
		return fmt.Errorf("file node cannot be nil")
	}
	
	// Handle directories by adding all their files
	if node.IsDir {
		return utm.addDirectory(node)
	}
	
	// Handle regular files
	return utm.addSingleFile(node)
}

// addSingleFile adds a single file to the transfer queue
func (utm *UnifiedTransferManager) addSingleFile(node *fileInfo.FileNode) error {
	utm.filesMu.Lock()
	utm.queueMu.Lock()
	defer utm.filesMu.Unlock()
	defer utm.queueMu.Unlock()
	
	// Check if file already exists in structure
	if _, exists := utm.structure.GetFile(node.Path); exists {
		return ErrTransferAlreadyExists
	}
	
	// Add file to structure manager
	if err := utm.structure.AddFileNode(node); err != nil {
		return fmt.Errorf("failed to add file to structure: %w", err)
	}
	
	// Create chunker for the file
	chunker, err := NewChunkerFromFileNode(node, utm.config.ChunkSize)
	if err != nil {
		return fmt.Errorf("failed to create chunker: %w", err)
	}
	
	// Store chunker
	utm.chunkers[node.Path] = chunker
	utm.pendingFiles = append(utm.pendingFiles, node.Path)
	
	// Update session status
	utm.statusMu.Lock()
	utm.sessionStatus.TotalFiles = utm.GetFileCount()
	utm.sessionStatus.PendingFiles = len(utm.pendingFiles)
	utm.sessionStatus.TotalBytes += node.Size
	utm.sessionStatus.LastUpdateTime = time.Now()
	utm.statusMu.Unlock()
	
	return nil
}

func  (utm *UnifiedTransferManager) GetFileCount() int {
	return  utm.structure.GetFileCount()
}

// addDirectory recursively adds all files in a directory to the transfer queue
func (utm *UnifiedTransferManager) addDirectory(dirNode *fileInfo.FileNode) error {
	if !dirNode.IsDir {
		return fmt.Errorf("node is not a directory: %s", dirNode.Path)
	}
	
	// Recursively add all files in the directory
	for _, child := range dirNode.Children {
		if err := utm.AddFile(&child); err != nil {
			return fmt.Errorf("failed to add child %s: %w", child.Path, err)
		}
	}
	
	return nil
}

// GetNextPendingFile returns the next file to transfer (queue management)
func (utm *UnifiedTransferManager) GetNextPendingFile() (*fileInfo.FileNode, bool) {
	utm.queueMu.RLock()
	defer utm.queueMu.RUnlock()
	
	if len(utm.pendingFiles) == 0 {
		return nil, false
	}
	
	filePath := utm.pendingFiles[0]
	
	// Get file from structure manager
	fileNode, exists := utm.structure.GetFile(filePath)
	if !exists {
		return nil, false
	}
	
	return fileNode, true
}

// MarkFileCompleted moves a file from pending to completed
func (utm *UnifiedTransferManager) MarkFileCompleted(filePath string) error {
	utm.queueMu.Lock()
	defer utm.queueMu.Unlock()
	
	// Remove from pending
	for i, path := range utm.pendingFiles {
		if path == filePath {
			utm.pendingFiles = append(utm.pendingFiles[:i], utm.pendingFiles[i+1:]...)
			break
		}
	}
	
	// Add to completed
	utm.completedFiles = append(utm.completedFiles, filePath)
	
	// Update session status
	utm.statusMu.Lock()
	utm.sessionStatus.CompletedFiles = len(utm.completedFiles)
	utm.sessionStatus.PendingFiles = len(utm.pendingFiles)
	utm.sessionStatus.LastUpdateTime = time.Now()
	utm.statusMu.Unlock()
	
	return nil
}

// MarkFileFailed moves a file from pending to failed
func (utm *UnifiedTransferManager) MarkFileFailed(filePath string) error {
	utm.queueMu.Lock()
	defer utm.queueMu.Unlock()
	
	// Remove from pending
	for i, path := range utm.pendingFiles {
		if path == filePath {
			utm.pendingFiles = append(utm.pendingFiles[:i], utm.pendingFiles[i+1:]...)
			break
		}
	}
	
	// Add to failed
	utm.failedFiles = append(utm.failedFiles, filePath)
	
	// Update session status
	utm.statusMu.Lock()
	utm.sessionStatus.FailedFiles = len(utm.failedFiles)
	utm.sessionStatus.PendingFiles = len(utm.pendingFiles)
	utm.sessionStatus.LastUpdateTime = time.Now()
	utm.statusMu.Unlock()
	
	return nil
}

// GetQueueStatus returns current queue statistics
func (utm *UnifiedTransferManager) GetQueueStatus() (pending, completed, failed int) {
	utm.queueMu.RLock()
	defer utm.queueMu.RUnlock()
	
	return len(utm.pendingFiles), len(utm.completedFiles), len(utm.failedFiles)
}

// GetChunker returns the chunker for a file (maintains compatibility with existing code)
func (utm *UnifiedTransferManager) GetChunker(filePath string) (*Chunker, bool) {
	utm.filesMu.RLock()
	defer utm.filesMu.RUnlock()
	
	chunker, exists := utm.chunkers[filePath]
	return chunker, exists
}

// GetAllFiles returns all file nodes (maintains compatibility)
func (utm *UnifiedTransferManager) GetAllFiles() []*fileInfo.FileNode {
	return utm.structure.GetAllFiles()
}

// GetTotalBytes returns the total bytes of all files
func (utm *UnifiedTransferManager) GetTotalBytes() int64 {
	return utm.structure.GetTotalSize()
}

// Close cleans up all resources
func (utm *UnifiedTransferManager) Close() error {
	utm.filesMu.Lock()
	utm.queueMu.Lock()
	defer utm.filesMu.Unlock()
	defer utm.queueMu.Unlock()
	
	// Close all chunkers
	for _, chunker := range utm.chunkers {
		if chunker != nil {
			chunker.Close()
		}
	}
	
	// Clear all data
	utm.structure.Clear()
	utm.chunkers = make(map[string]*Chunker)
	utm.pendingFiles = make([]string, 0)
	utm.completedFiles = make([]string, 0)
	utm.failedFiles = make([]string, 0)
	
	return nil
}

// GetSessionStatus returns a copy of the current session status
func (utm *UnifiedTransferManager) GetSessionStatus() *SessionTransferStatus {
	utm.statusMu.RLock()
	defer utm.statusMu.RUnlock()
	
	// Return a deep copy
	statusCopy := *utm.sessionStatus
	if utm.sessionStatus.CurrentFile != nil {
		currentFileCopy := *utm.sessionStatus.CurrentFile
		statusCopy.CurrentFile = &currentFileCopy
	}
	
	return &statusCopy
}

func (utm *UnifiedTransferManager) GetFile(path string) (*fileInfo.FileNode, bool) {
	return utm.structure.GetFile(path)
}

// StartTransfer starts transferring a file and updates session status
func (utm *UnifiedTransferManager) StartTransfer(filePath string) error {
	utm.filesMu.RLock()
	managedFile, exists := utm.GetFile(filePath)
	utm.filesMu.RUnlock()
	
	if !exists {
		return ErrTransferNotFound
	}
	
	utm.statusMu.Lock()
	defer utm.statusMu.Unlock()
	
	// Create transfer status for current file
	currentFile := &TransferStatus{
		FilePath:       filePath,
		SessionID:      utm.sessionStatus.SessionID,
		State:          TransferStateActive,
		TotalBytes:     managedFile.Size,
		FileSize:       managedFile.Size,
		StartTime:      time.Now(),
		LastUpdateTime: time.Now(),
		MaxRetries:     utm.config.DefaultRetryPolicy.MaxRetries,
	}
	
	oldSessionStatus := *utm.sessionStatus
	oldCurrentFile := utm.sessionStatus.CurrentFile
	
	utm.sessionStatus.CurrentFile = currentFile
	utm.sessionStatus.LastUpdateTime = time.Now()
	
	// Update session totals if this is the first time we're seeing this file
	utm.updateSessionTotals()
	
	// Create copy for session status notification to avoid race conditions
	newSessionStatus := *utm.sessionStatus
	
	// Notify listeners with copies
	go utm.notifyFileStatusChanged(filePath, oldCurrentFile, currentFile)
	go utm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)
	
	return nil
}

func (utm *UnifiedTransferManager) GetTotalSize() int64 {
	return utm.structure.GetTotalSize()
}

// UpdateProgress updates the progress of the current file transfer
func (utm *UnifiedTransferManager) UpdateProgress(filePath string, bytesSent int64) error {
	utm.statusMu.Lock()
	defer utm.statusMu.Unlock()
	
	if utm.sessionStatus.CurrentFile == nil || utm.sessionStatus.CurrentFile.FilePath != filePath {
		return ErrTransferNotFound
	}
	
	oldSessionStatus := *utm.sessionStatus
	oldFileStatus := *utm.sessionStatus.CurrentFile
	
	// Update current file progress
	utm.sessionStatus.CurrentFile.BytesSent = bytesSent
	utm.sessionStatus.CurrentFile.LastUpdateTime = time.Now()
	utm.sessionStatus.CurrentFile.calculateMetrics()
	
	// Update overall progress
	utm.sessionStatus.OverallProgress = utm.sessionStatus.GetSessionProgressPercentage()
	utm.sessionStatus.LastUpdateTime = time.Now()
	
	// Create copies for notification to avoid race conditions
	newFileStatus := *utm.sessionStatus.CurrentFile
	newSessionStatus := *utm.sessionStatus
	
	// Notify listeners with copies
	go utm.notifyFileStatusChanged(filePath, &oldFileStatus, &newFileStatus)
	go utm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)
	
	return nil
}

// CompleteTransfer marks the current file transfer as completed
func (utm *UnifiedTransferManager) CompleteTransfer(filePath string) error {
	utm.statusMu.Lock()
	defer utm.statusMu.Unlock()
	
	if utm.sessionStatus.CurrentFile == nil || utm.sessionStatus.CurrentFile.FilePath != filePath {
		return ErrTransferNotFound
	}
	
	oldSessionStatus := *utm.sessionStatus
	oldFileStatus := *utm.sessionStatus.CurrentFile
	
	// Mark current file as completed
	utm.sessionStatus.CurrentFile.State = TransferStateCompleted
	now := time.Now()
	utm.sessionStatus.CurrentFile.CompletionTime = &now
	
	// Update session counters
	utm.sessionStatus.CompletedFiles++
	utm.sessionStatus.PendingFiles--
	utm.sessionStatus.BytesCompleted += utm.sessionStatus.CurrentFile.TotalBytes
	
	completedFile := utm.sessionStatus.CurrentFile
	utm.sessionStatus.CurrentFile = nil // No current file until next one starts
	utm.sessionStatus.LastUpdateTime = now
	
	// Update overall progress
	utm.sessionStatus.OverallProgress = utm.sessionStatus.GetSessionProgressPercentage()
	
	// Check if session is complete
	if utm.sessionStatus.CompletedFiles+utm.sessionStatus.FailedFiles >= utm.sessionStatus.TotalFiles {
		utm.sessionStatus.CompletionTime = &now
		utm.sessionStatus.State = StatusSessionStateCompleted
	}
	
	// Also update the queue (avoid calling MarkFileCompleted to prevent deadlock)
	utm.queueMu.Lock()
	// Remove from pending
	for i, path := range utm.pendingFiles {
		if path == filePath {
			utm.pendingFiles = append(utm.pendingFiles[:i], utm.pendingFiles[i+1:]...)
			break
		}
	}
	// Add to completed
	utm.completedFiles = append(utm.completedFiles, filePath)
	utm.queueMu.Unlock()
	
	// Create copy for session status notification to avoid race conditions
	newSessionStatus := *utm.sessionStatus
	
	// Notify listeners with copies
	go utm.notifyFileStatusChanged(filePath, &oldFileStatus, completedFile)
	go utm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)
	
	return nil
}

// FailTransfer marks the current file transfer as failed
func (utm *UnifiedTransferManager) FailTransfer(filePath string, err error) error {
	utm.statusMu.Lock()
	defer utm.statusMu.Unlock()
	
	if utm.sessionStatus.CurrentFile == nil || utm.sessionStatus.CurrentFile.FilePath != filePath {
		return ErrTransferNotFound
	}
	
	oldSessionStatus := *utm.sessionStatus
	oldFileStatus := *utm.sessionStatus.CurrentFile
	
	// Mark current file as failed
	utm.sessionStatus.CurrentFile.State = TransferStateFailed
	utm.sessionStatus.CurrentFile.LastError = err
	
	// Update session counters
	utm.sessionStatus.FailedFiles++
	utm.sessionStatus.PendingFiles--
	
	failedFile := utm.sessionStatus.CurrentFile
	utm.sessionStatus.CurrentFile = nil // No current file until next one starts
	utm.sessionStatus.LastUpdateTime = time.Now()
	
	// Update overall progress
	utm.sessionStatus.OverallProgress = utm.sessionStatus.GetSessionProgressPercentage()
	
	// Check if session should be marked as failed (all files failed)
	if utm.sessionStatus.FailedFiles >= utm.sessionStatus.TotalFiles {
		now := time.Now()
		utm.sessionStatus.CompletionTime = &now
		utm.sessionStatus.State = StatusSessionStateFailed
	}
	
	// Also update the queue (avoid calling MarkFileFailed to prevent deadlock)
	utm.queueMu.Lock()
	// Remove from pending
	for i, path := range utm.pendingFiles {
		if path == filePath {
			utm.pendingFiles = append(utm.pendingFiles[:i], utm.pendingFiles[i+1:]...)
			break
		}
	}
	// Add to failed
	utm.failedFiles = append(utm.failedFiles, filePath)
	utm.queueMu.Unlock()
	
	// Create copy for session status notification to avoid race conditions
	newSessionStatus := *utm.sessionStatus
	
	// Notify listeners with copies
	go utm.notifyFileStatusChanged(filePath, &oldFileStatus, failedFile)
	go utm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)
	
	return nil
}

// PauseTransfer pauses the current file transfer
func (utm *UnifiedTransferManager) PauseTransfer(filePath string) error {
	utm.statusMu.Lock()
	defer utm.statusMu.Unlock()
	
	if utm.sessionStatus.CurrentFile == nil || utm.sessionStatus.CurrentFile.FilePath != filePath {
		return ErrTransferNotFound
	}
	
	if utm.sessionStatus.CurrentFile.State != TransferStateActive {
		return ErrInvalidStateTransition
	}
	
	oldSessionStatus := *utm.sessionStatus
	oldFileStatus := *utm.sessionStatus.CurrentFile
	
	utm.sessionStatus.CurrentFile.State = TransferStatePaused
	utm.sessionStatus.CurrentFile.LastUpdateTime = time.Now()
	utm.sessionStatus.LastUpdateTime = time.Now()
	
	// Create copies for notification to avoid race conditions
	newFileStatus := *utm.sessionStatus.CurrentFile
	newSessionStatus := *utm.sessionStatus
	
	// Notify listeners with copies
	go utm.notifyFileStatusChanged(filePath, &oldFileStatus, &newFileStatus)
	go utm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)
	
	return nil
}

// ResumeTransfer resumes a paused file transfer
func (utm *UnifiedTransferManager) ResumeTransfer(filePath string) error {
	utm.statusMu.Lock()
	defer utm.statusMu.Unlock()
	
	if utm.sessionStatus.CurrentFile == nil || utm.sessionStatus.CurrentFile.FilePath != filePath {
		return ErrTransferNotFound
	}
	
	if utm.sessionStatus.CurrentFile.State != TransferStatePaused {
		return ErrInvalidStateTransition
	}
	
	oldSessionStatus := *utm.sessionStatus
	oldFileStatus := *utm.sessionStatus.CurrentFile
	
	utm.sessionStatus.CurrentFile.State = TransferStateActive
	utm.sessionStatus.CurrentFile.LastUpdateTime = time.Now()
	utm.sessionStatus.LastUpdateTime = time.Now()
	
	// Create copies for notification to avoid race conditions
	newFileStatus := *utm.sessionStatus.CurrentFile
	newSessionStatus := *utm.sessionStatus
	
	// Notify listeners with copies
	go utm.notifyFileStatusChanged(filePath, &oldFileStatus, &newFileStatus)
	go utm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)
	
	return nil
}

// GetFileStatus returns the status of a specific file (for compatibility)
func (utm *UnifiedTransferManager) GetFileStatus(filePath string) (*TransferStatus, error) {
	utm.statusMu.RLock()
	defer utm.statusMu.RUnlock()
	
	// Check if it's the current file
	if utm.sessionStatus.CurrentFile != nil && utm.sessionStatus.CurrentFile.FilePath == filePath {
		statusCopy := *utm.sessionStatus.CurrentFile
		return &statusCopy, nil
	}
	
	// Check if it's in completed files
	for _, completedPath := range utm.completedFiles {
		if completedPath == filePath {
			// Return a completed status
			return &TransferStatus{
				FilePath:   filePath,
				SessionID:  utm.sessionStatus.SessionID,
				State:      TransferStateCompleted,
				BytesSent:  0, // We don't track individual file progress after completion
				TotalBytes: 0, // Would need to look up from managedFile if needed
			}, nil
		}
	}
	
	// Check if it's in failed files
	for _, failedPath := range utm.failedFiles {
		if failedPath == filePath {
			// Return a failed status
			return &TransferStatus{
				FilePath:  filePath,
				SessionID: utm.sessionStatus.SessionID,
				State:     TransferStateFailed,
			}, nil
		}
	}
	
	// Check if it's in pending files
	for _, pendingPath := range utm.pendingFiles {
		if pendingPath == filePath {
			// Return a pending status
			return &TransferStatus{
				FilePath:  filePath,
				SessionID: utm.sessionStatus.SessionID,
				State:     TransferStatePending,
			}, nil
		}
	}
	
	return nil, ErrTransferNotFound
}

// AddStatusListener adds a status change listener
func (utm *UnifiedTransferManager) AddStatusListener(listener StatusListener) {
	utm.eventsMu.Lock()
	defer utm.eventsMu.Unlock()
	
	utm.listeners = append(utm.listeners, listener)
}

// Helper methods

func (utm *UnifiedTransferManager) updateSessionTotals() {
	// This method assumes statusMu is already locked
	utm.sessionStatus.TotalFiles = utm.GetFileCount()
	utm.sessionStatus.PendingFiles = len(utm.pendingFiles)
	utm.sessionStatus.TotalBytes = utm.GetTotalBytes()
}

func (utm *UnifiedTransferManager) notifyFileStatusChanged(filePath string, oldStatus, newStatus *TransferStatus) {
	utm.eventsMu.RLock()
	defer utm.eventsMu.RUnlock()
	
	for _, listener := range utm.listeners {
		// Run in goroutine to prevent blocking (already in goroutine, but being explicit)
		listener.OnFileStatusChanged(filePath, oldStatus, newStatus)
	}
}

func (utm *UnifiedTransferManager) notifySessionStatusChanged(oldStatus, newStatus *SessionTransferStatus) {
	utm.eventsMu.RLock()
	defer utm.eventsMu.RUnlock()
	
	for _, listener := range utm.listeners {
		// Run in goroutine to prevent blocking (already in goroutine, but being explicit)
		listener.OnSessionStatusChanged(oldStatus, newStatus)
	}
}