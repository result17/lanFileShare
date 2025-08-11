package transfer

import (
	"fmt"
	"sync"
	"time"
)

// TransferStatusManager manages the status of all file transfers
// It provides thread-safe operations for tracking transfer progress,
// managing transfer states, and querying transfer information.
type TransferStatusManager struct {
	// transfers maps file paths to their current transfer status
	transfers map[string]*TransferStatus
	
	// config holds the configuration for transfer management
	config *TransferConfig
	
	// mu provides thread-safe access to the transfers map
	mu sync.RWMutex
	
	// nextSessionID is used to generate unique session IDs
	nextSessionID int64
	
	// sessionMu protects nextSessionID
	sessionMu sync.Mutex
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
		transfers:     make(map[string]*TransferStatus),
		config:        config,
		nextSessionID: time.Now().Unix(),
	}
}

// GetConfig returns a copy of the current configuration
func (tsm *TransferStatusManager) GetConfig() *TransferConfig {
	// Return a copy to prevent external modification
	configCopy := *tsm.config
	return &configCopy
}

// UpdateConfig updates the manager's configuration
// Returns an error if the new configuration is invalid
func (tsm *TransferStatusManager) UpdateConfig(config *TransferConfig) error {
	if config == nil {
		return ErrInvalidConfiguration
	}
	
	if err := config.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfiguration, err)
	}
	
	tsm.config = config
	return nil
}

// StartTransfer initializes a new transfer with the given parameters
// Returns an error if a transfer with the same file path already exists
func (tsm *TransferStatusManager) StartTransfer(filePath string, totalSize int64) (*TransferStatus, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}
	
	if totalSize < 0 {
		return nil, fmt.Errorf("total size cannot be negative")
	}
	
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	// Check if transfer already exists
	if _, exists := tsm.transfers[filePath]; exists {
		return nil, ErrTransferAlreadyExists
	}
	
	// Check concurrent transfer limit (count all non-terminal transfers)
	nonTerminalCount := tsm.getNonTerminalTransferCountUnsafe()
	if nonTerminalCount >= tsm.config.MaxConcurrentTransfers {
		return nil, ErrMaxTransfersExceeded
	}
	
	// Generate session ID
	sessionID := tsm.generateSessionID()
	
	// Create new transfer status
	status := &TransferStatus{
		FilePath:    filePath,
		SessionID:   sessionID,
		State:       TransferStatePending,
		BytesSent:   0,
		TotalBytes:  totalSize,
		ChunksSent:  0,
		TotalChunks: int(totalSize / int64(tsm.config.ChunkSize)),
		StartTime:   time.Now(),
		LastUpdateTime: time.Now(),
		RetryCount:  0,
		MaxRetries:  tsm.config.DefaultRetryPolicy.MaxRetries,
		FileSize:    totalSize,
		Priority:    0, // Default priority
	}
	
	// Adjust total chunks for remainder
	if totalSize%int64(tsm.config.ChunkSize) != 0 {
		status.TotalChunks++
	}
	
	// Store the transfer
	tsm.transfers[filePath] = status
	
	return status, nil
}

// GetTransferStatus retrieves the current status of a transfer
// Returns ErrTransferNotFound if the transfer doesn't exist
func (tsm *TransferStatusManager) GetTransferStatus(filePath string) (*TransferStatus, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}
	
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()
	
	status, exists := tsm.transfers[filePath]
	if !exists {
		return nil, ErrTransferNotFound
	}
	
	// Return a copy to prevent external modification
	statusCopy := *status
	return &statusCopy, nil
}

// GetAllTransfers returns a slice of all current transfer statuses
// Returns copies of the statuses to prevent external modification
func (tsm *TransferStatusManager) GetAllTransfers() []*TransferStatus {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()
	
	transfers := make([]*TransferStatus, 0, len(tsm.transfers))
	for _, status := range tsm.transfers {
		// Create a copy to prevent external modification
		statusCopy := *status
		transfers = append(transfers, &statusCopy)
	}
	
	return transfers
}

// UpdateProgress updates the progress of a transfer
// Returns an error if the transfer doesn't exist or if the update is invalid
func (tsm *TransferStatusManager) UpdateProgress(filePath string, bytesSent int64) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	
	if bytesSent < 0 {
		return fmt.Errorf("bytes sent cannot be negative")
	}
	
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	status, exists := tsm.transfers[filePath]
	if !exists {
		return ErrTransferNotFound
	}
	
	// Validate that we're not going backwards (unless it's a retry)
	if bytesSent < status.BytesSent && status.State != TransferStatePaused {
		return fmt.Errorf("bytes sent cannot decrease from %d to %d", status.BytesSent, bytesSent)
	}
	
	// Validate that we're not exceeding total size
	if bytesSent > status.TotalBytes {
		return fmt.Errorf("bytes sent (%d) cannot exceed total size (%d)", bytesSent, status.TotalBytes)
	}
	
	// Calculate chunks sent
	chunksSent := int(bytesSent / int64(tsm.config.ChunkSize))
	if bytesSent%int64(tsm.config.ChunkSize) != 0 {
		chunksSent++
	}
	
	// Update progress
	status.UpdateProgress(bytesSent, chunksSent)
	
	return nil
}

// CompleteTransfer marks a transfer as completed
// Returns an error if the transfer doesn't exist or cannot be completed
func (tsm *TransferStatusManager) CompleteTransfer(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	status, exists := tsm.transfers[filePath]
	if !exists {
		return ErrTransferNotFound
	}
	
	// Validate state transition
	if !status.State.CanTransitionTo(TransferStateCompleted) {
		return fmt.Errorf("%w: cannot transition from %s to completed", 
			ErrInvalidStateTransition, status.State)
	}
	
	// Update state
	status.State = TransferStateCompleted
	now := time.Now()
	status.CompletionTime = &now
	status.LastUpdateTime = now
	
	// Ensure progress is at 100%
	status.BytesSent = status.TotalBytes
	status.ChunksSent = status.TotalChunks
	
	return nil
}

// FailTransfer marks a transfer as failed with the given error
// Returns an error if the transfer doesn't exist or cannot be failed
func (tsm *TransferStatusManager) FailTransfer(filePath string, transferError error) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	status, exists := tsm.transfers[filePath]
	if !exists {
		return ErrTransferNotFound
	}
	
	// Validate state transition
	if !status.State.CanTransitionTo(TransferStateFailed) {
		return fmt.Errorf("%w: cannot transition from %s to failed", 
			ErrInvalidStateTransition, status.State)
	}
	
	// Update state
	status.State = TransferStateFailed
	status.LastError = transferError
	now := time.Now()
	status.CompletionTime = &now
	status.LastUpdateTime = now
	
	return nil
}

// CancelTransfer marks a transfer as cancelled
// Returns an error if the transfer doesn't exist or cannot be cancelled
func (tsm *TransferStatusManager) CancelTransfer(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	status, exists := tsm.transfers[filePath]
	if !exists {
		return ErrTransferNotFound
	}
	
	// Validate state transition
	if !status.State.CanTransitionTo(TransferStateCancelled) {
		return fmt.Errorf("%w: cannot transition from %s to cancelled", 
			ErrInvalidStateTransition, status.State)
	}
	
	// Update state
	status.State = TransferStateCancelled
	status.LastError = ErrTransferCancelled
	now := time.Now()
	status.CompletionTime = &now
	status.LastUpdateTime = now
	
	return nil
}

// PauseTransfer pauses an active transfer
// Returns an error if the transfer doesn't exist or cannot be paused
func (tsm *TransferStatusManager) PauseTransfer(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	status, exists := tsm.transfers[filePath]
	if !exists {
		return ErrTransferNotFound
	}
	
	// Validate state transition
	if !status.State.CanTransitionTo(TransferStatePaused) {
		return fmt.Errorf("%w: cannot transition from %s to paused", 
			ErrInvalidStateTransition, status.State)
	}
	
	// Update state
	status.State = TransferStatePaused
	status.LastUpdateTime = time.Now()
	
	return nil
}

// ResumeTransfer resumes a paused transfer
// Returns an error if the transfer doesn't exist or cannot be resumed
func (tsm *TransferStatusManager) ResumeTransfer(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	status, exists := tsm.transfers[filePath]
	if !exists {
		return ErrTransferNotFound
	}
	
	// Validate state transition
	if !status.State.CanTransitionTo(TransferStateActive) {
		return fmt.Errorf("%w: cannot transition from %s to active", 
			ErrInvalidStateTransition, status.State)
	}
	
	// Update state
	status.State = TransferStateActive
	status.LastUpdateTime = time.Now()
	
	return nil
}

// RemoveTransfer removes a completed, failed, or cancelled transfer from tracking
// Returns an error if the transfer doesn't exist or is still active
func (tsm *TransferStatusManager) RemoveTransfer(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	status, exists := tsm.transfers[filePath]
	if !exists {
		return ErrTransferNotFound
	}
	
	// Only allow removal of terminal states
	if !status.State.IsTerminal() {
		return fmt.Errorf("cannot remove active transfer (current state: %s)", status.State)
	}
	
	delete(tsm.transfers, filePath)
	return nil
}

// GetOverallProgress calculates and returns aggregated progress across all transfers
func (tsm *TransferStatusManager) GetOverallProgress() *OverallProgress {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()
	
	progress := &OverallProgress{
		LastUpdateTime: time.Now(),
	}
	
	var totalBytes, bytesSent int64
	var totalRate float64
	var activeTransfers int
	
	// Find the earliest start time
	var earliestStart time.Time
	firstTransfer := true
	
	for _, status := range tsm.transfers {
		progress.TotalTransfers++
		
		// Track earliest start time
		if firstTransfer || status.StartTime.Before(earliestStart) {
			earliestStart = status.StartTime
			firstTransfer = false
		}
		
		// Accumulate bytes
		totalBytes += status.TotalBytes
		bytesSent += status.BytesSent
		
		// Count by state
		switch status.State {
		case TransferStateActive:
			progress.ActiveTransfers++
			activeTransfers++
			totalRate += status.TransferRate
		case TransferStateCompleted:
			progress.CompletedTransfers++
		case TransferStateFailed:
			progress.FailedTransfers++
		case TransferStateCancelled:
			progress.CancelledTransfers++
		}
	}
	
	progress.TotalBytes = totalBytes
	progress.BytesSent = bytesSent
	progress.BytesRemaining = totalBytes - bytesSent
	progress.SessionStartTime = earliestStart
	
	// Calculate overall percentage
	if totalBytes > 0 {
		progress.OverallPercentage = float64(bytesSent) / float64(totalBytes) * 100.0
	}
	
	// Calculate average rate and ETA
	if activeTransfers > 0 {
		progress.AverageRate = totalRate / float64(activeTransfers)
		if progress.AverageRate > 0 && progress.BytesRemaining > 0 {
			progress.EstimatedETA = time.Duration(float64(progress.BytesRemaining)/progress.AverageRate) * time.Second
		}
	}
	
	return progress
}

// GetActiveTransferCount returns the number of currently active transfers
func (tsm *TransferStatusManager) GetActiveTransferCount() int {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()
	
	return tsm.getActiveTransferCountUnsafe()
}

// getActiveTransferCountUnsafe returns the number of active transfers without locking
// This method assumes the caller already holds the appropriate lock
func (tsm *TransferStatusManager) getActiveTransferCountUnsafe() int {
	count := 0
	for _, status := range tsm.transfers {
		if status.State == TransferStateActive {
			count++
		}
	}
	return count
}

// getNonTerminalTransferCountUnsafe returns the number of non-terminal transfers without locking
// This includes pending, active, and paused transfers
func (tsm *TransferStatusManager) getNonTerminalTransferCountUnsafe() int {
	count := 0
	for _, status := range tsm.transfers {
		if !status.State.IsTerminal() {
			count++
		}
	}
	return count
}

// generateSessionID generates a unique session ID
func (tsm *TransferStatusManager) generateSessionID() string {
	tsm.sessionMu.Lock()
	defer tsm.sessionMu.Unlock()
	
	tsm.nextSessionID++
	return fmt.Sprintf("status-session-%d", tsm.nextSessionID)
}

// Clear removes all transfers from the manager
// This is primarily useful for testing and cleanup
func (tsm *TransferStatusManager) Clear() {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	tsm.transfers = make(map[string]*TransferStatus)
}

// GetTransferCount returns the total number of transfers being tracked
func (tsm *TransferStatusManager) GetTransferCount() int {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()
	
	return len(tsm.transfers)
}