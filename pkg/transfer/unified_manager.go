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
	session   *TransferSession      // Session management
	config    *TransferConfig       // Configuration
	structure *FileStructureManager // File structure organization

	// Chunking management
	chunkers map[string]*Chunker // File path -> Chunker
	filesMu  sync.RWMutex

	// Transfer queue management - using maps for O(1) operations
	pendingFiles   map[string]bool // Set of pending file paths
	completedFiles map[string]bool // Set of completed file paths
	failedFiles    map[string]bool // Set of failed file paths
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
	ID() string
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
		pendingFiles:   make(map[string]bool),
		completedFiles: make(map[string]bool),
		failedFiles:    make(map[string]bool),
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
	utm.addFileToQueue(node.Path, FileQueueStatePending)

	// Update session status
	utm.statusMu.Lock()
	utm.sessionStatus.TotalFiles = utm.GetFileCount()
	utm.sessionStatus.PendingFiles = len(utm.pendingFiles)
	utm.sessionStatus.TotalBytes += node.Size
	utm.sessionStatus.LastUpdateTime = time.Now()
	utm.statusMu.Unlock()

	return nil
}

func (utm *UnifiedTransferManager) GetFileCount() int {
	return utm.structure.GetFileCount()
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

	filePath, hasPending := utm.getFirstPendingFile()
	if !hasPending {
		return nil, false
	}

	// Get file from structure manager
	fileNode, exists := utm.structure.GetFile(filePath)
	if !exists {
		return nil, false
	}

	return fileNode, true
}

// MarkFileCompleted moves a file to completed state from any current state
func (utm *UnifiedTransferManager) MarkFileCompleted(filePath string) error {
	utm.queueMu.Lock()
	defer utm.queueMu.Unlock()

	// Try to move from any state to completed
	moved := utm.moveFileInQueue(filePath, FileQueueStatePending, FileQueueStateCompleted) ||
		utm.moveFileInQueue(filePath, FileQueueStateFailed, FileQueueStateCompleted)

	// If file wasn't in any known state, add it to completed
	if !moved {
		utm.addFileToQueue(filePath, FileQueueStateCompleted)
	}

	// Update session status
	utm.statusMu.Lock()
	pending, completed, failed := utm.getQueueCounts()
	utm.sessionStatus.CompletedFiles = completed
	utm.sessionStatus.PendingFiles = pending
	utm.sessionStatus.FailedFiles = failed
	utm.sessionStatus.LastUpdateTime = time.Now()
	utm.statusMu.Unlock()

	return nil
}

// MarkFileFailed moves a file to failed state from any current state
func (utm *UnifiedTransferManager) MarkFileFailed(filePath string) error {
	utm.queueMu.Lock()
	defer utm.queueMu.Unlock()

	// Try to move from any state to failed
	moved := utm.moveFileInQueue(filePath, FileQueueStatePending, FileQueueStateFailed) ||
		utm.moveFileInQueue(filePath, FileQueueStateCompleted, FileQueueStateFailed)

	// If file wasn't in any known state, add it to failed
	if !moved {
		utm.addFileToQueue(filePath, FileQueueStateFailed)
	}

	// Update session status
	utm.statusMu.Lock()
	pending, completed, failed := utm.getQueueCounts()
	utm.sessionStatus.CompletedFiles = completed
	utm.sessionStatus.PendingFiles = pending
	utm.sessionStatus.FailedFiles = failed
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
	utm.pendingFiles = make(map[string]bool)
	utm.completedFiles = make(map[string]bool)
	utm.failedFiles = make(map[string]bool)

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
// Uses consistent lock ordering: queueMu -> statusMu to prevent deadlock
func (utm *UnifiedTransferManager) CompleteTransfer(filePath string) error {
	// Lock in consistent order: queueMu first, then statusMu
	utm.queueMu.Lock()
	utm.statusMu.Lock()
	defer utm.statusMu.Unlock()
	defer utm.queueMu.Unlock()

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

	// Update the queue (queueMu already locked in consistent order)
	utm.moveFileInQueue(filePath, FileQueueStatePending, FileQueueStateCompleted)

	// Create copy for session status notification to avoid race conditions
	newSessionStatus := *utm.sessionStatus

	// Notify listeners with copies
	go utm.notifyFileStatusChanged(filePath, &oldFileStatus, completedFile)
	go utm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)

	return nil
}

// FailTransfer marks the current file transfer as failed
// Uses consistent lock ordering: queueMu -> statusMu to prevent deadlock
func (utm *UnifiedTransferManager) FailTransfer(filePath string, err error) error {
	// Lock in consistent order: queueMu first, then statusMu
	utm.queueMu.Lock()
	utm.statusMu.Lock()
	defer utm.statusMu.Unlock()
	defer utm.queueMu.Unlock()

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

	// Update the queue (queueMu already locked in consistent order)
	utm.moveFileInQueue(filePath, FileQueueStatePending, FileQueueStateFailed)

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

// GetFileStatus returns the complete status of a specific file
func (utm *UnifiedTransferManager) GetFileStatus(filePath string) (*TransferStatus, error) {
	utm.statusMu.RLock()
	defer utm.statusMu.RUnlock()

	// Check if it's the current file (active transfer)
	if utm.sessionStatus.CurrentFile != nil && utm.sessionStatus.CurrentFile.FilePath == filePath {
		statusCopy := *utm.sessionStatus.CurrentFile
		return &statusCopy, nil
	}

	// For non-active files, we need to construct complete status information
	// First, get the file information from the structure manager
	fileNode, exists := utm.structure.GetFile(filePath)
	if !exists {
		return nil, ErrTransferNotFound
	}

	// Create base status with complete file information
	baseStatus := &TransferStatus{
		FilePath:       filePath,
		SessionID:      utm.sessionStatus.SessionID,
		TotalBytes:     fileNode.Size,
		LastUpdateTime: utm.sessionStatus.LastUpdateTime,
	}

	// Determine state and set appropriate fields
	if utm.isFileInQueue(filePath, FileQueueStateCompleted) {
		baseStatus.State = TransferStateCompleted
		baseStatus.BytesSent = fileNode.Size // Completed files have all bytes sent

		// Estimate completion time based on session data
		// This is an approximation since we don't store individual file completion times
		if utm.sessionStatus.CompletionTime != nil {
			baseStatus.CompletionTime = utm.sessionStatus.CompletionTime
		}

		// Calculate estimated transfer rate if we have session data
		if !utm.sessionStatus.StartTime.IsZero() && utm.sessionStatus.LastUpdateTime.After(utm.sessionStatus.StartTime) {
			elapsed := utm.sessionStatus.LastUpdateTime.Sub(utm.sessionStatus.StartTime)
			if elapsed > 0 {
				baseStatus.TransferRate = float64(fileNode.Size) / elapsed.Seconds()
			}
		}

		return baseStatus, nil
	}

	if utm.isFileInQueue(filePath, FileQueueStateFailed) {
		baseStatus.State = TransferStateFailed
		baseStatus.BytesSent = 0 // Failed files typically have 0 bytes sent

		// For failed files, we don't have completion time
		// LastError would need to be tracked separately if needed

		return baseStatus, nil
	}

	if utm.isFileInQueue(filePath, FileQueueStatePending) {
		baseStatus.State = TransferStatePending
		baseStatus.BytesSent = 0 // Pending files haven't started

		// Set start time to session start time if available
		if !utm.sessionStatus.StartTime.IsZero() {
			baseStatus.StartTime = utm.sessionStatus.StartTime
		}

		return baseStatus, nil
	}

	// File exists in structure but not in any queue - this shouldn't happen normally
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
	pending, completed, failed := utm.getQueueCounts()
	utm.sessionStatus.PendingFiles = pending
	utm.sessionStatus.CompletedFiles = completed
	utm.sessionStatus.FailedFiles = failed
	utm.sessionStatus.TotalBytes = utm.GetTotalBytes()
}

func (utm *UnifiedTransferManager) notifyFileStatusChanged(filePath string, oldStatus, newStatus *TransferStatus) {
	// Copy listeners under lock to minimize lock holding time
	utm.eventsMu.RLock()
	listenersCopy := make([]StatusListener, len(utm.listeners))
	copy(listenersCopy, utm.listeners)
	utm.eventsMu.RUnlock()

	// Notify each listener in its own goroutine to prevent blocking
	for _, listener := range listenersCopy {
		go func(l StatusListener) {
			// Use defer to recover from panics in listener code
			defer func() {
				if r := recover(); r != nil {
					// Log the panic but don't crash the entire system
					// In a real application, you might want to use a proper logger
					// log.Printf("Panic in file status listener: %v", r)
				}
			}()
			l.OnFileStatusChanged(filePath, oldStatus, newStatus)
		}(listener)
	}
}

func (utm *UnifiedTransferManager) notifySessionStatusChanged(oldStatus, newStatus *SessionTransferStatus) {
	// Copy listeners under lock to minimize lock holding time
	utm.eventsMu.RLock()
	listenersCopy := make([]StatusListener, len(utm.listeners))
	copy(listenersCopy, utm.listeners)
	utm.eventsMu.RUnlock()

	// Notify each listener in its own goroutine to prevent blocking
	for _, listener := range listenersCopy {
		go func(l StatusListener) {
			// Use defer to recover from panics in listener code
			defer func() {
				if r := recover(); r != nil {
					// Log the panic but don't crash the entire system
					// In a real application, you might want to use a proper logger
					// log.Printf("Panic in session status listener: %v", r)
				}
			}()
			l.OnSessionStatusChanged(oldStatus, newStatus)
		}(listener)
	}
}

// FileQueueState represents the state of a file in the queue
type FileQueueState int

const (
	FileQueueStatePending FileQueueState = iota
	FileQueueStateCompleted
	FileQueueStateFailed
)

// moveFileInQueue efficiently moves a file between queue states
// This method assumes queueMu is already locked by the caller
func (utm *UnifiedTransferManager) moveFileInQueue(filePath string, fromState, toState FileQueueState) bool {
	// Remove from source state
	var removed bool
	switch fromState {
	case FileQueueStatePending:
		if utm.pendingFiles[filePath] {
			delete(utm.pendingFiles, filePath)
			removed = true
		}
	case FileQueueStateCompleted:
		if utm.completedFiles[filePath] {
			delete(utm.completedFiles, filePath)
			removed = true
		}
	case FileQueueStateFailed:
		if utm.failedFiles[filePath] {
			delete(utm.failedFiles, filePath)
			removed = true
		}
	}

	// If file wasn't in source state, return false
	if !removed {
		return false
	}

	// Add to target state
	switch toState {
	case FileQueueStatePending:
		utm.pendingFiles[filePath] = true
	case FileQueueStateCompleted:
		utm.completedFiles[filePath] = true
	case FileQueueStateFailed:
		utm.failedFiles[filePath] = true
	}

	return true
}

// addFileToQueue adds a file to the specified queue state
// This method assumes queueMu is already locked by the caller
func (utm *UnifiedTransferManager) addFileToQueue(filePath string, state FileQueueState) {
	switch state {
	case FileQueueStatePending:
		utm.pendingFiles[filePath] = true
	case FileQueueStateCompleted:
		utm.completedFiles[filePath] = true
	case FileQueueStateFailed:
		utm.failedFiles[filePath] = true
	}
}

// removeFileFromQueue removes a file from the specified queue state
// This method assumes queueMu is already locked by the caller
func (utm *UnifiedTransferManager) removeFileFromQueue(filePath string, state FileQueueState) bool {
	switch state {
	case FileQueueStatePending:
		if utm.pendingFiles[filePath] {
			delete(utm.pendingFiles, filePath)
			return true
		}
	case FileQueueStateCompleted:
		if utm.completedFiles[filePath] {
			delete(utm.completedFiles, filePath)
			return true
		}
	case FileQueueStateFailed:
		if utm.failedFiles[filePath] {
			delete(utm.failedFiles, filePath)
			return true
		}
	}
	return false
}

// getQueueCounts returns the count of files in each queue state
// This method assumes queueMu is already locked by the caller
func (utm *UnifiedTransferManager) getQueueCounts() (pending, completed, failed int) {
	return len(utm.pendingFiles), len(utm.completedFiles), len(utm.failedFiles)
}

// getFirstPendingFile returns the first pending file path (for compatibility)
// Since we're using a map, we'll return any pending file
// This method assumes queueMu is already locked by the caller
func (utm *UnifiedTransferManager) getFirstPendingFile() (string, bool) {
	for filePath := range utm.pendingFiles {
		return filePath, true
	}
	return "", false
}

// isFileInQueue checks if a file is in the specified queue state
// This method assumes queueMu is already locked by the caller
func (utm *UnifiedTransferManager) isFileInQueue(filePath string, state FileQueueState) bool {
	switch state {
	case FileQueueStatePending:
		return utm.pendingFiles[filePath]
	case FileQueueStateCompleted:
		return utm.completedFiles[filePath]
	case FileQueueStateFailed:
		return utm.failedFiles[filePath]
	}
	return false
}
