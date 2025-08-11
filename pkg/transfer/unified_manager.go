package transfer

import (
	"fmt"
	"sync"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// UnifiedTransferManager focuses on transfer management only
// It manages file queue and chunkers, and tracks session status
type UnifiedTransferManager struct {
	// Core components
	session *TransferSession // Reuse existing session from session.go
	config  *TransferConfig  // Reuse existing config from config.go
	
	// File and chunk management (no status here)
	files    map[string]*ManagedFile // Only FileNode + Chunker
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

// ManagedFile only contains file and chunking information
type ManagedFile struct {
	// File information (from fileInfo.FileNode)
	Node *fileInfo.FileNode
	
	// Chunking (from Chunker)
	Chunker *Chunker
}



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
		files:          make(map[string]*ManagedFile),
		pendingFiles:   make([]string, 0),
		completedFiles: make([]string, 0),
		failedFiles:    make([]string, 0),
		sessionStatus:  sessionStatus,
		listeners:      make([]StatusListener, 0),
	}
}

// AddFile adds a file to the transfer queue (focused on file management only)
func (utm *UnifiedTransferManager) AddFile(node *fileInfo.FileNode) error {
	if node == nil {
		return fmt.Errorf("file node cannot be nil")
	}
	
	utm.filesMu.Lock()
	utm.queueMu.Lock()
	defer utm.filesMu.Unlock()
	defer utm.queueMu.Unlock()
	
	// Check if file already exists
	if _, exists := utm.files[node.Path]; exists {
		return ErrTransferAlreadyExists
	}
	
	// Create chunker for the file
	chunker, err := NewChunkerFromFileNode(node, utm.config.ChunkSize)
	if err != nil {
		return fmt.Errorf("failed to create chunker: %w", err)
	}
	
	// Create managed file (no status tracking here)
	managedFile := &ManagedFile{
		Node:    node,
		Chunker: chunker,
	}
	
	utm.files[node.Path] = managedFile
	utm.pendingFiles = append(utm.pendingFiles, node.Path)
	
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
	
	utm.filesMu.RLock()
	defer utm.filesMu.RUnlock()
	
	managedFile, exists := utm.files[filePath]
	if !exists {
		return nil, false
	}
	
	return managedFile.Node, true
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
	
	managedFile, exists := utm.files[filePath]
	if !exists {
		return nil, false
	}
	
	return managedFile.Chunker, true
}

// GetAllFiles returns all file nodes (maintains compatibility)
func (utm *UnifiedTransferManager) GetAllFiles() []*fileInfo.FileNode {
	utm.filesMu.RLock()
	defer utm.filesMu.RUnlock()
	
	files := make([]*fileInfo.FileNode, 0, len(utm.files))
	for _, managedFile := range utm.files {
		files = append(files, managedFile.Node)
	}
	
	return files
}

// GetTotalBytes returns the total bytes of all files
func (utm *UnifiedTransferManager) GetTotalBytes() int64 {
	utm.filesMu.RLock()
	defer utm.filesMu.RUnlock()
	
	var total int64
	for _, managedFile := range utm.files {
		total += managedFile.Node.Size
	}
	
	return total
}

// Close cleans up all resources
func (utm *UnifiedTransferManager) Close() error {
	utm.filesMu.Lock()
	utm.queueMu.Lock()
	defer utm.filesMu.Unlock()
	defer utm.queueMu.Unlock()
	
	// Close all chunkers
	for _, managedFile := range utm.files {
		if managedFile.Chunker != nil {
			managedFile.Chunker.Close()
		}
	}
	
	// Clear all data
	utm.files = make(map[string]*ManagedFile)
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

// StartTransfer starts transferring a file and updates session status
func (utm *UnifiedTransferManager) StartTransfer(filePath string) error {
	utm.filesMu.RLock()
	managedFile, exists := utm.files[filePath]
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
		TotalBytes:     managedFile.Node.Size,
		FileSize:       managedFile.Node.Size,
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
	
	// Notify listeners
	go utm.notifyFileStatusChanged(filePath, oldCurrentFile, currentFile)
	go utm.notifySessionStatusChanged(&oldSessionStatus, utm.sessionStatus)
	
	return nil
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
	
	// Notify listeners
	go utm.notifyFileStatusChanged(filePath, &oldFileStatus, utm.sessionStatus.CurrentFile)
	go utm.notifySessionStatusChanged(&oldSessionStatus, utm.sessionStatus)
	
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
	
	// Also update the queue
	utm.MarkFileCompleted(filePath)
	
	// Notify listeners
	go utm.notifyFileStatusChanged(filePath, &oldFileStatus, completedFile)
	go utm.notifySessionStatusChanged(&oldSessionStatus, utm.sessionStatus)
	
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
	
	// Also update the queue
	utm.MarkFileFailed(filePath)
	
	// Notify listeners
	go utm.notifyFileStatusChanged(filePath, &oldFileStatus, failedFile)
	go utm.notifySessionStatusChanged(&oldSessionStatus, utm.sessionStatus)
	
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
	
	// Notify listeners
	go utm.notifyFileStatusChanged(filePath, &oldFileStatus, utm.sessionStatus.CurrentFile)
	go utm.notifySessionStatusChanged(&oldSessionStatus, utm.sessionStatus)
	
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
	
	// Notify listeners
	go utm.notifyFileStatusChanged(filePath, &oldFileStatus, utm.sessionStatus.CurrentFile)
	go utm.notifySessionStatusChanged(&oldSessionStatus, utm.sessionStatus)
	
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
	utm.sessionStatus.TotalFiles = len(utm.files)
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