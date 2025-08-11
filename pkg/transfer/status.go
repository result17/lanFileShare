package transfer

import (
	"errors"
	"time"
)

// This file defines data structures for transfer STATUS MANAGEMENT.
// It is separate from and complementary to the existing transfer protocol (protocol.go):
//
// protocol.go (MessageType):     Network protocol layer - defines messages sent over the wire
// status.go (TransferState):     Status management layer - tracks persistent transfer state
//
// Example:
// - MessageType.TransferBegin:   "I'm sending a message to start transfer" (network event)
// - TransferState.Active:        "This transfer is currently active" (persistent state)
//
// The two systems work together:
// 1. Send TransferBegin message → Update state to TransferStateActive
// 2. Send ProgressUpdate message → Update TransferStatus.BytesSent
// 3. Send TransferComplete message → Update state to TransferStateCompleted

// TransferState represents the current state of a file transfer
type TransferState int

const (
	// TransferStatePending indicates the transfer is queued but not yet started
	TransferStatePending TransferState = iota
	// TransferStateActive indicates the transfer is currently in progress
	TransferStateActive
	// TransferStatePaused indicates the transfer has been paused by user or system
	TransferStatePaused
	// TransferStateCompleted indicates the transfer finished successfully
	TransferStateCompleted
	// TransferStateFailed indicates the transfer failed due to an error
	TransferStateFailed
	// TransferStateCancelled indicates the transfer was cancelled by user
	TransferStateCancelled
)

// String returns a human-readable string representation of the transfer state
func (ts TransferState) String() string {
	switch ts {
	case TransferStatePending:
		return "pending"
	case TransferStateActive:
		return "active"
	case TransferStatePaused:
		return "paused"
	case TransferStateCompleted:
		return "completed"
	case TransferStateFailed:
		return "failed"
	case TransferStateCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// IsTerminal returns true if the transfer state is final (completed, failed, or cancelled)
func (ts TransferState) IsTerminal() bool {
	return ts == TransferStateCompleted || ts == TransferStateFailed || ts == TransferStateCancelled
}

// CanTransitionTo checks if a state transition is valid
func (ts TransferState) CanTransitionTo(newState TransferState) bool {
	// Terminal states cannot transition to other states
	if ts.IsTerminal() {
		return false
	}

	switch ts {
	case TransferStatePending:
		return newState == TransferStateActive || newState == TransferStateCancelled
	case TransferStateActive:
		return newState == TransferStatePaused || newState == TransferStateCompleted || 
			   newState == TransferStateFailed || newState == TransferStateCancelled
	case TransferStatePaused:
		return newState == TransferStateActive || newState == TransferStateCancelled
	default:
		return false
	}
}

// TransferStatus represents the current status and progress of a file transfer
type TransferStatus struct {
	// Basic identification
	FilePath  string `json:"file_path"`
	SessionID string `json:"session_id"`
	State     TransferState `json:"state"`

	// Progress information
	BytesSent   int64 `json:"bytes_sent"`
	TotalBytes  int64 `json:"total_bytes"`
	ChunksSent  int   `json:"chunks_sent"`
	TotalChunks int   `json:"total_chunks"`

	// Performance metrics
	TransferRate float64       `json:"transfer_rate"` // bytes per second
	ETA          time.Duration `json:"eta"`           // estimated time to completion

	// Lifecycle timestamps
	StartTime      time.Time  `json:"start_time"`
	LastUpdateTime time.Time  `json:"last_update_time"`
	CompletionTime *time.Time `json:"completion_time,omitempty"`

	// Error handling
	LastError  error `json:"last_error,omitempty"`
	RetryCount int   `json:"retry_count"`
	MaxRetries int   `json:"max_retries"`

	// File metadata
	FileSize     int64  `json:"file_size"`
	FileChecksum string `json:"file_checksum"`
	Priority     int    `json:"priority"`
}

// GetProgressPercentage calculates the completion percentage (0-100)
func (ts *TransferStatus) GetProgressPercentage() float64 {
	if ts.TotalBytes == 0 {
		return 0.0
	}
	return float64(ts.BytesSent) / float64(ts.TotalBytes) * 100.0
}

// GetRemainingBytes returns the number of bytes left to transfer
func (ts *TransferStatus) GetRemainingBytes() int64 {
	remaining := ts.TotalBytes - ts.BytesSent
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsComplete returns true if the transfer is 100% complete
func (ts *TransferStatus) IsComplete() bool {
	return ts.BytesSent >= ts.TotalBytes && ts.State == TransferStateCompleted
}

// UpdateProgress updates the transfer progress and recalculates metrics
func (ts *TransferStatus) UpdateProgress(bytesSent int64, chunksSent int) {
	ts.BytesSent = bytesSent
	ts.ChunksSent = chunksSent
	ts.LastUpdateTime = time.Now()
	
	// Recalculate transfer rate and ETA
	ts.calculateMetrics()
}

// calculateMetrics recalculates transfer rate and ETA based on current progress
func (ts *TransferStatus) calculateMetrics() {
	if ts.State != TransferStateActive {
		return
	}

	elapsed := time.Since(ts.StartTime)
	if elapsed.Seconds() > 0 {
		ts.TransferRate = float64(ts.BytesSent) / elapsed.Seconds()
		
		if ts.TransferRate > 0 {
			remainingBytes := ts.GetRemainingBytes()
			ts.ETA = time.Duration(float64(remainingBytes)/ts.TransferRate) * time.Second
		}
	}
}

// OverallProgress represents aggregated progress across multiple transfers
type OverallProgress struct {
	// Transfer counts
	TotalTransfers     int `json:"total_transfers"`
	ActiveTransfers    int `json:"active_transfers"`
	CompletedTransfers int `json:"completed_transfers"`
	FailedTransfers    int `json:"failed_transfers"`
	CancelledTransfers int `json:"cancelled_transfers"`

	// Byte progress
	TotalBytes     int64 `json:"total_bytes"`
	BytesSent      int64 `json:"bytes_sent"`
	BytesRemaining int64 `json:"bytes_remaining"`

	// Aggregated metrics
	OverallPercentage float64       `json:"overall_percentage"`
	AverageRate       float64       `json:"average_rate"` // bytes per second
	EstimatedETA      time.Duration `json:"estimated_eta"`

	// Session information
	SessionStartTime time.Time `json:"session_start_time"`
	LastUpdateTime   time.Time `json:"last_update_time"`
}

// GetCompletionPercentage calculates the overall completion percentage
func (op *OverallProgress) GetCompletionPercentage() float64 {
	if op.TotalBytes == 0 {
		return 0.0
	}
	return float64(op.BytesSent) / float64(op.TotalBytes) * 100.0
}

// StatusSessionState represents the state of a status tracking session
// (renamed to avoid conflict with existing TransferSession)
type StatusSessionState int

const (
	// StatusSessionStateActive indicates the session has active transfers
	StatusSessionStateActive StatusSessionState = iota
	// StatusSessionStatePaused indicates all transfers in the session are paused
	StatusSessionStatePaused
	// StatusSessionStateCompleted indicates all transfers completed successfully
	StatusSessionStateCompleted
	// StatusSessionStateFailed indicates the session failed due to critical errors
	StatusSessionStateFailed
	// StatusSessionStateCancelled indicates the session was cancelled by user
	StatusSessionStateCancelled
)

// String returns a human-readable string representation of the session state
func (ss StatusSessionState) String() string {
	switch ss {
	case StatusSessionStateActive:
		return "active"
	case StatusSessionStatePaused:
		return "paused"
	case StatusSessionStateCompleted:
		return "completed"
	case StatusSessionStateFailed:
		return "failed"
	case StatusSessionStateCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// RetryPolicy defines the retry behavior for failed transfers
type RetryPolicy struct {
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	BackoffFactor   float64       `json:"backoff_factor"`
	MaxDelay        time.Duration `json:"max_delay"`
	RetryableErrors []string      `json:"retryable_errors"` // Error message patterns
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:    3,
		InitialDelay:  time.Second,
		BackoffFactor: 2.0,
		MaxDelay:      30 * time.Second,
		RetryableErrors: []string{
			"connection timeout",
			"temporary failure",
			"network unreachable",
			"connection reset",
		},
	}
}

// GetRetryDelay calculates the delay before the next retry attempt
func (rp *RetryPolicy) GetRetryDelay(retryCount int) time.Duration {
	if retryCount <= 0 {
		return rp.InitialDelay
	}

	delay := rp.InitialDelay
	for i := 0; i < retryCount; i++ {
		delay = time.Duration(float64(delay) * rp.BackoffFactor)
		if delay > rp.MaxDelay {
			return rp.MaxDelay
		}
	}
	return delay
}

// IsRetryable checks if an error should trigger a retry
func (rp *RetryPolicy) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	for _, pattern := range rp.RetryableErrors {
		if contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// TransferConfig is now defined in config.go to avoid duplication

// Error types for transfer status management
var (
	// ErrTransferNotFound is returned when a requested transfer doesn't exist
	ErrTransferNotFound = errors.New("transfer not found")
	
	// ErrInvalidStateTransition is returned when an invalid state transition is attempted
	ErrInvalidStateTransition = errors.New("invalid state transition")
	
	// ErrTransferAlreadyExists is returned when trying to start a transfer that already exists
	ErrTransferAlreadyExists = errors.New("transfer already exists")
	
	// ErrSessionNotFound is returned when a requested session doesn't exist
	ErrSessionNotFound = errors.New("session not found")
	
	// ErrSessionAlreadyExists is returned when trying to create a session that already exists
	ErrSessionAlreadyExists = errors.New("session already exists")
	
	// ErrMaxTransfersExceeded is returned when the maximum number of concurrent transfers is reached
	ErrMaxTransfersExceeded = errors.New("maximum concurrent transfers exceeded")
	
	// ErrInvalidConfiguration is returned when configuration validation fails
	ErrInvalidConfiguration = errors.New("invalid configuration")
	
	// ErrTransferCancelled is returned when a transfer is cancelled
	ErrTransferCancelled = errors.New("transfer cancelled")
)

// SessionTransferStatus represents the status of an entire transfer session
// It tracks both overall progress and the current file being transferred
type SessionTransferStatus struct {
	// Session identification
	SessionID string `json:"session_id"`
	
	// File counts
	TotalFiles     int `json:"total_files"`
	CompletedFiles int `json:"completed_files"`
	FailedFiles    int `json:"failed_files"`
	PendingFiles   int `json:"pending_files"`
	
	// Byte progress
	TotalBytes      int64   `json:"total_bytes"`
	BytesCompleted  int64   `json:"bytes_completed"`
	OverallProgress float64 `json:"overall_progress"` // 0-100 percentage
	
	// Current file being transferred
	CurrentFile *TransferStatus `json:"current_file,omitempty"`
	
	// Session timing
	StartTime      time.Time  `json:"start_time"`
	LastUpdateTime time.Time  `json:"last_update_time"`
	CompletionTime *time.Time `json:"completion_time,omitempty"`
	
	// Session state
	State StatusSessionState `json:"state"`
}

// GetSessionProgressPercentage calculates the overall session progress percentage
func (sts *SessionTransferStatus) GetSessionProgressPercentage() float64 {
	if sts.TotalBytes == 0 {
		return 0.0
	}
	
	currentFileBytes := int64(0)
	if sts.CurrentFile != nil {
		currentFileBytes = sts.CurrentFile.BytesSent
	}
	
	totalCompleted := sts.BytesCompleted + currentFileBytes
	return float64(totalCompleted) / float64(sts.TotalBytes) * 100.0
}

// GetRemainingFiles returns the number of files left to transfer
func (sts *SessionTransferStatus) GetRemainingFiles() int {
	return sts.TotalFiles - sts.CompletedFiles - sts.FailedFiles
}

// GetRemainingBytes returns the number of bytes left to transfer
func (sts *SessionTransferStatus) GetRemainingBytes() int64 {
	currentFileBytes := int64(0)
	if sts.CurrentFile != nil {
		currentFileBytes = sts.CurrentFile.BytesSent
	}
	
	totalCompleted := sts.BytesCompleted + currentFileBytes
	remaining := sts.TotalBytes - totalCompleted
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsSessionComplete returns true if all files have been processed (completed or failed)
func (sts *SessionTransferStatus) IsSessionComplete() bool {
	return sts.CompletedFiles + sts.FailedFiles >= sts.TotalFiles
}

// HasActiveTransfer returns true if there's currently a file being transferred
func (sts *SessionTransferStatus) HasActiveTransfer() bool {
	return sts.CurrentFile != nil && sts.CurrentFile.State == TransferStateActive
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	// Simple case-insensitive substring check
	// In a real implementation, you might want to use strings.Contains with strings.ToLower
	return len(s) >= len(substr) && 
		   (s == substr || (len(s) > len(substr) && 
		   (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		   containsAt(s, substr, 1))))
}

// Helper function for substring search
func containsAt(s, substr string, start int) bool {
	if start >= len(s) {
		return false
	}
	if start+len(substr) > len(s) {
		return containsAt(s, substr, start+1)
	}
	if s[start:start+len(substr)] == substr {
		return true
	}
	return containsAt(s, substr, start+1)
}