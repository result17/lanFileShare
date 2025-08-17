package transfer

import (
	"fmt"
	"sync"
	"time"
)

// TransferStatusManager manages a single SessionTransferStatus
// It provides thread-safe operations for tracking session progress,
// managing session states, and handling current file transfers.
type TransferStatusManager struct {
	// sessionStatus holds the current session status
	sessionStatus *SessionTransferStatus

	// config holds the configuration for transfer management
	config *TransferConfig

	// mu provides thread-safe access to the session status
	mu sync.RWMutex

	// Event system
	listeners []StatusListener
	eventsMu  sync.RWMutex
}

// NewTransferStatusManager creates a new TransferStatusManager with default configuration
func NewTransferStatusManager() *TransferStatusManager {
	return NewTransferStatusManagerWithConfig(DefaultTransferConfig())
}

// NewTransferStatusManagerWithConfig creates a new TransferStatusManager with custom configuration
func NewTransferStatusManagerWithConfig(config *TransferConfig) *TransferStatusManager {
	if config == nil {
		config = DefaultTransferConfig()
	}

	return &TransferStatusManager{
		sessionStatus: nil, // Will be initialized when session is created
		config:        config,
		listeners:     make([]StatusListener, 0),
	}
}

// InitializeSession initializes the transfer session
func (tsm *TransferStatusManager) InitializeSession(sessionID string, totalFiles int, totalBytes int64) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if totalFiles < 0 {
		return fmt.Errorf("total files cannot be negative")
	}

	if totalBytes < 0 {
		return fmt.Errorf("total bytes cannot be negative")
	}

	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	// Check if session already exists
	if tsm.sessionStatus != nil {
		return ErrSessionAlreadyExists
	}

	// Create new session status
	tsm.sessionStatus = &SessionTransferStatus{
		SessionID:       sessionID,
		TotalFiles:      totalFiles,
		CompletedFiles:  0,
		FailedFiles:     0,
		PendingFiles:    totalFiles,
		TotalBytes:      totalBytes,
		BytesCompleted:  0,
		OverallProgress: 0.0,
		CurrentFile:     nil,
		StartTime:       time.Now(),
		LastUpdateTime:  time.Now(),
		State:           StatusSessionStateActive,
	}

	return nil
}

// GetSessionStatus retrieves the current session status
func (tsm *TransferStatusManager) GetSessionStatus() (*SessionTransferStatus, error) {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	if tsm.sessionStatus == nil {
		return nil, ErrSessionNotFound
	}

	// Return a deep copy to prevent external modification
	statusCopy := *tsm.sessionStatus
	if tsm.sessionStatus.CurrentFile != nil {
		currentFileCopy := *tsm.sessionStatus.CurrentFile
		statusCopy.CurrentFile = &currentFileCopy
	}

	return &statusCopy, nil
}

// StartFileTransfer starts transferring a file
func (tsm *TransferStatusManager) StartFileTransfer(filePath string, fileSize int64) (*TransferStatus, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	if fileSize < 0 {
		return nil, fmt.Errorf("file size cannot be negative")
	}

	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	if tsm.sessionStatus == nil {
		return nil, ErrSessionNotFound
	}

	// Check if there's already a current file transfer
	if tsm.sessionStatus.CurrentFile != nil && tsm.sessionStatus.CurrentFile.State == TransferStateActive {
		return nil, fmt.Errorf("session already has an active file transfer: %s", tsm.sessionStatus.CurrentFile.FilePath)
	}

	oldSessionStatus := *tsm.sessionStatus

	// Create new transfer status for current file
	currentFile := &TransferStatus{
		FilePath:       filePath,
		SessionID:      tsm.sessionStatus.SessionID,
		State:          TransferStateActive,
		TotalBytes:     fileSize,
		FileSize:       fileSize,
		StartTime:      time.Now(),
		LastUpdateTime: time.Now(),
		MaxRetries:     tsm.config.DefaultRetryPolicy.MaxRetries,
	}

	oldCurrentFile := tsm.sessionStatus.CurrentFile
	tsm.sessionStatus.CurrentFile = currentFile
	tsm.sessionStatus.LastUpdateTime = time.Now()

	// Create copy for session status notification to avoid race conditions
	newSessionStatus := *tsm.sessionStatus

	// Notify listeners with copies to prevent data races
	tsm.notifyFileStatusChanged(filePath, oldCurrentFile, currentFile)
	tsm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)

	return currentFile, nil
}

// UpdateFileProgress updates the progress of the current file transfer
func (tsm *TransferStatusManager) UpdateFileProgress(bytesSent int64) error {
	if bytesSent < 0 {
		return fmt.Errorf("bytes sent cannot be negative")
	}

	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	if tsm.sessionStatus == nil {
		return ErrSessionNotFound
	}

	if tsm.sessionStatus.CurrentFile == nil {
		return ErrTransferNotFound
	}

	oldSessionStatus := *tsm.sessionStatus
	oldFileStatus := *tsm.sessionStatus.CurrentFile

	// Update current file progress
	tsm.sessionStatus.CurrentFile.BytesSent = bytesSent
	tsm.sessionStatus.CurrentFile.LastUpdateTime = time.Now()
	tsm.sessionStatus.CurrentFile.calculateMetrics()

	// Update overall session progress
	tsm.sessionStatus.OverallProgress = tsm.sessionStatus.GetSessionProgressPercentage()
	tsm.sessionStatus.LastUpdateTime = time.Now()

	// Create copies for notification to avoid race conditions
	newFileStatus := *tsm.sessionStatus.CurrentFile
	newSessionStatus := *tsm.sessionStatus

	// Notify listeners with copies to prevent data races
	tsm.notifyFileStatusChanged(tsm.sessionStatus.CurrentFile.FilePath, &oldFileStatus, &newFileStatus)
	tsm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)

	return nil
}

// CompleteCurrentFile marks the current file transfer as completed
func (tsm *TransferStatusManager) CompleteCurrentFile() error {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	if tsm.sessionStatus == nil {
		return ErrSessionNotFound
	}

	if tsm.sessionStatus.CurrentFile == nil {
		return ErrTransferNotFound
	}

	oldSessionStatus := *tsm.sessionStatus
	oldFileStatus := *tsm.sessionStatus.CurrentFile

	// Mark current file as completed
	tsm.sessionStatus.CurrentFile.State = TransferStateCompleted
	now := time.Now()
	tsm.sessionStatus.CurrentFile.CompletionTime = &now

	// Update session counters
	tsm.sessionStatus.CompletedFiles++
	tsm.sessionStatus.PendingFiles--
	tsm.sessionStatus.BytesCompleted += tsm.sessionStatus.CurrentFile.TotalBytes

	completedFile := tsm.sessionStatus.CurrentFile
	tsm.sessionStatus.CurrentFile = nil // No current file until next one starts
	tsm.sessionStatus.LastUpdateTime = now

	// Update overall progress
	tsm.sessionStatus.OverallProgress = tsm.sessionStatus.GetSessionProgressPercentage()

	// Check if session is complete
	if tsm.sessionStatus.IsSessionComplete() {
		tsm.sessionStatus.CompletionTime = &now
		tsm.sessionStatus.State = StatusSessionStateCompleted
	}

	// Create copy for session status notification to avoid race conditions
	newSessionStatus := *tsm.sessionStatus

	// Notify listeners with copies to prevent data races
	tsm.notifyFileStatusChanged(completedFile.FilePath, &oldFileStatus, completedFile)
	tsm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)

	return nil
}

// FailCurrentFile marks the current file transfer as failed
func (tsm *TransferStatusManager) FailCurrentFile(err error) error {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	if tsm.sessionStatus == nil {
		return ErrSessionNotFound
	}

	if tsm.sessionStatus.CurrentFile == nil {
		return ErrTransferNotFound
	}

	oldSessionStatus := *tsm.sessionStatus
	oldFileStatus := *tsm.sessionStatus.CurrentFile

	// Mark current file as failed
	tsm.sessionStatus.CurrentFile.State = TransferStateFailed
	tsm.sessionStatus.CurrentFile.LastError = err

	// Update session counters
	tsm.sessionStatus.FailedFiles++
	tsm.sessionStatus.PendingFiles--

	failedFile := tsm.sessionStatus.CurrentFile
	tsm.sessionStatus.CurrentFile = nil // No current file until next one starts
	tsm.sessionStatus.LastUpdateTime = time.Now()

	// Update overall progress
	tsm.sessionStatus.OverallProgress = tsm.sessionStatus.GetSessionProgressPercentage()

	// Check if session should be marked as failed (all files failed)
	if tsm.sessionStatus.FailedFiles >= tsm.sessionStatus.TotalFiles {
		now := time.Now()
		tsm.sessionStatus.CompletionTime = &now
		tsm.sessionStatus.State = StatusSessionStateFailed
	}

	// Create copy for session status notification to avoid race conditions
	newSessionStatus := *tsm.sessionStatus

	// Notify listeners with copies to prevent data races
	tsm.notifyFileStatusChanged(failedFile.FilePath, &oldFileStatus, failedFile)
	tsm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)

	return nil
}

// PauseCurrentFile pauses the current file transfer
func (tsm *TransferStatusManager) PauseCurrentFile() error {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	if tsm.sessionStatus == nil {
		return ErrSessionNotFound
	}

	if tsm.sessionStatus.CurrentFile == nil {
		return ErrTransferNotFound
	}

	if tsm.sessionStatus.CurrentFile.State != TransferStateActive {
		return ErrInvalidStateTransition
	}

	oldSessionStatus := *tsm.sessionStatus
	oldFileStatus := *tsm.sessionStatus.CurrentFile

	tsm.sessionStatus.CurrentFile.State = TransferStatePaused
	tsm.sessionStatus.CurrentFile.LastUpdateTime = time.Now()
	tsm.sessionStatus.LastUpdateTime = time.Now()

	// Create copies for notification to avoid race conditions
	newFileStatus := *tsm.sessionStatus.CurrentFile
	newSessionStatus := *tsm.sessionStatus

	// Notify listeners with copies to prevent data races
	tsm.notifyFileStatusChanged(tsm.sessionStatus.CurrentFile.FilePath, &oldFileStatus, &newFileStatus)
	tsm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)

	return nil
}

// ResumeCurrentFile resumes a paused file transfer
func (tsm *TransferStatusManager) ResumeCurrentFile() error {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	if tsm.sessionStatus == nil {
		return ErrSessionNotFound
	}

	if tsm.sessionStatus.CurrentFile == nil {
		return ErrTransferNotFound
	}

	if tsm.sessionStatus.CurrentFile.State != TransferStatePaused {
		return ErrInvalidStateTransition
	}

	oldSessionStatus := *tsm.sessionStatus
	oldFileStatus := *tsm.sessionStatus.CurrentFile

	tsm.sessionStatus.CurrentFile.State = TransferStateActive
	tsm.sessionStatus.CurrentFile.LastUpdateTime = time.Now()
	tsm.sessionStatus.LastUpdateTime = time.Now()

	// Create copies for notification to avoid race conditions
	newFileStatus := *tsm.sessionStatus.CurrentFile
	newSessionStatus := *tsm.sessionStatus

	// Notify listeners with copies to prevent data races
	tsm.notifyFileStatusChanged(tsm.sessionStatus.CurrentFile.FilePath, &oldFileStatus, &newFileStatus)
	tsm.notifySessionStatusChanged(&oldSessionStatus, &newSessionStatus)

	return nil
}

// GetCurrentFile returns the current file being transferred
func (tsm *TransferStatusManager) GetCurrentFile() (*TransferStatus, error) {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	if tsm.sessionStatus == nil {
		return nil, ErrSessionNotFound
	}

	if tsm.sessionStatus.CurrentFile == nil {
		return nil, ErrTransferNotFound
	}

	// Return a copy to prevent external modification
	statusCopy := *tsm.sessionStatus.CurrentFile
	return &statusCopy, nil
}

// IsSessionActive returns true if there's an active session
func (tsm *TransferStatusManager) IsSessionActive() bool {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	return tsm.sessionStatus != nil && !tsm.sessionStatus.IsSessionComplete()
}

// ResetSession clears the current session (for cleanup or new session)
func (tsm *TransferStatusManager) ResetSession() {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	tsm.sessionStatus = nil
}

// AddStatusListener adds a status change listener
func (tsm *TransferStatusManager) AddStatusListener(listener StatusListener) {
	tsm.eventsMu.Lock()
	defer tsm.eventsMu.Unlock()

	tsm.listeners = append(tsm.listeners, listener)
}

// Helper methods

func (tsm *TransferStatusManager) notifyFileStatusChanged(filePath string, oldStatus, newStatus *TransferStatus) {
	tsm.eventsMu.RLock()
	defer tsm.eventsMu.RUnlock()

	for _, listener := range tsm.listeners {
		// Run in goroutine to prevent blocking
		go func(l StatusListener) {
			l.OnFileStatusChanged(filePath, oldStatus, newStatus)
		}(listener)
	}
}

func (tsm *TransferStatusManager) notifySessionStatusChanged(oldStatus, newStatus *SessionTransferStatus) {
	tsm.eventsMu.RLock()
	defer tsm.eventsMu.RUnlock()

	for _, listener := range tsm.listeners {
		// Run in goroutine to prevent blocking
		go func(l StatusListener) {
			l.OnSessionStatusChanged(oldStatus, newStatus)
		}(listener)
	}
}

func (tsm *TransferStatusManager) RemoveStatusListener(listenerID string) {
    tsm.eventsMu.Lock()
    defer tsm.eventsMu.Unlock()

    newListeners := make([]StatusListener, 0, len(tsm.listeners))
    for _, l := range tsm.listeners {
        if l.ID() != listenerID {
            newListeners = append(newListeners, l)
        }
    }
    tsm.listeners = newListeners
}
