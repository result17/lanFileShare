package transfer

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransferState_String(t *testing.T) {
	tests := []struct {
		state    TransferState
		expected string
	}{
		{TransferStatePending, "pending"},
		{TransferStateActive, "active"},
		{TransferStatePaused, "paused"},
		{TransferStateCompleted, "completed"},
		{TransferStateFailed, "failed"},
		{TransferStateCancelled, "canceled"},
		{TransferState(999), "unknown"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, test.state.String(), "TransferState(%d).String() mismatch", test.state)
	}
}

func TestTransferState_IsTerminal(t *testing.T) {
	tests := []struct {
		state    TransferState
		expected bool
	}{
		{TransferStatePending, false},
		{TransferStateActive, false},
		{TransferStatePaused, false},
		{TransferStateCompleted, true},
		{TransferStateFailed, true},
		{TransferStateCancelled, true},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, test.state.IsTerminal(), "TransferState(%s).IsTerminal() mismatch", test.state)
	}
}

func TestTransferState_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from     TransferState
		to       TransferState
		expected bool
	}{
		// From pending
		{TransferStatePending, TransferStateActive, true},
		{TransferStatePending, TransferStateCancelled, true},
		{TransferStatePending, TransferStatePaused, false},
		{TransferStatePending, TransferStateCompleted, false},

		// From active
		{TransferStateActive, TransferStatePaused, true},
		{TransferStateActive, TransferStateCompleted, true},
		{TransferStateActive, TransferStateFailed, true},
		{TransferStateActive, TransferStateCancelled, true},
		{TransferStateActive, TransferStatePending, false},

		// From paused
		{TransferStatePaused, TransferStateActive, true},
		{TransferStatePaused, TransferStateCancelled, true},
		{TransferStatePaused, TransferStateCompleted, false},
		{TransferStatePaused, TransferStateFailed, false},

		// From terminal states (should all be false)
		{TransferStateCompleted, TransferStateActive, false},
		{TransferStateFailed, TransferStateActive, false},
		{TransferStateCancelled, TransferStateActive, false},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, test.from.CanTransitionTo(test.to), 
			"TransferState(%s).CanTransitionTo(%s) mismatch", test.from, test.to)
	}
}

func TestTransferStatus_GetProgressPercentage(t *testing.T) {
	tests := []struct {
		name       string
		bytesSent  int64
		totalBytes int64
		expected   float64
	}{
		{"zero total", 0, 0, 0.0},
		{"zero progress", 0, 1000, 0.0},
		{"half complete", 500, 1000, 50.0},
		{"fully complete", 1000, 1000, 100.0},
		{"over complete", 1200, 1000, 120.0}, // Edge case
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := &TransferStatus{
				BytesSent:  test.bytesSent,
				TotalBytes: test.totalBytes,
			}
			assert.Equal(t, test.expected, status.GetProgressPercentage(), "GetProgressPercentage() mismatch")
		})
	}
}

func TestTransferStatus_GetRemainingBytes(t *testing.T) {
	tests := []struct {
		name       string
		bytesSent  int64
		totalBytes int64
		expected   int64
	}{
		{"no progress", 0, 1000, 1000},
		{"half complete", 500, 1000, 500},
		{"fully complete", 1000, 1000, 0},
		{"over complete", 1200, 1000, 0}, // Should not go negative
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := &TransferStatus{
				BytesSent:  test.bytesSent,
				TotalBytes: test.totalBytes,
			}
			assert.Equal(t, test.expected, status.GetRemainingBytes(), "GetRemainingBytes() mismatch")
		})
	}
}

func TestTransferStatus_IsComplete(t *testing.T) {
	tests := []struct {
		name       string
		bytesSent  int64
		totalBytes int64
		state      TransferState
		expected   bool
	}{
		{"complete and correct state", 1000, 1000, TransferStateCompleted, true},
		{"complete but wrong state", 1000, 1000, TransferStateActive, false},
		{"incomplete but correct state", 500, 1000, TransferStateCompleted, false},
		{"incomplete and wrong state", 500, 1000, TransferStateActive, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := &TransferStatus{
				BytesSent:  test.bytesSent,
				TotalBytes: test.totalBytes,
				State:      test.state,
			}
			assert.Equal(t, test.expected, status.IsComplete(), "IsComplete() mismatch")
		})
	}
}

func TestTransferStatus_UpdateProgress(t *testing.T) {
	startTime := time.Now().Add(-10 * time.Second)
	status := &TransferStatus{
		FilePath:   "/test/file.txt",
		TotalBytes: 1000,
		State:      TransferStateActive,
		StartTime:  startTime,
	}

	// Update progress
	status.UpdateProgress(500, 5)

	// Check that values were updated
	assert.Equal(t, int64(500), status.BytesSent, "BytesSent should be updated")
	assert.Equal(t, 5, status.ChunksSent, "ChunksSent should be updated")
	assert.True(t, status.LastUpdateTime.After(startTime), "LastUpdateTime should be updated to current time")

	// Check that transfer rate was calculated
	assert.Greater(t, status.TransferRate, float64(0), "TransferRate should be calculated and positive")

	// Check that ETA was calculated
	assert.Greater(t, status.ETA, time.Duration(0), "ETA should be calculated and positive")
}

func TestOverallProgress_GetCompletionPercentage(t *testing.T) {
	tests := []struct {
		name       string
		bytesSent  int64
		totalBytes int64
		expected   float64
	}{
		{"zero total", 0, 0, 0.0},
		{"zero progress", 0, 1000, 0.0},
		{"half complete", 500, 1000, 50.0},
		{"fully complete", 1000, 1000, 100.0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			progress := &OverallProgress{
				BytesSent:  test.bytesSent,
				TotalBytes: test.totalBytes,
			}
			assert.Equal(t, test.expected, progress.GetCompletionPercentage(), "GetCompletionPercentage() mismatch")
		})
	}
}

func TestStatusSessionState_String(t *testing.T) {
	tests := []struct {
		state    StatusSessionState
		expected string
	}{
		{StatusSessionStateActive, "active"},
		{StatusSessionStatePaused, "paused"},
		{StatusSessionStateCompleted, "completed"},
		{StatusSessionStateFailed, "failed"},
		{StatusSessionStateCancelled, "canceled"},
		{StatusSessionState(999), "unknown"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, test.state.String(), "StatusSessionState(%d).String() mismatch", test.state)
	}
}

func TestRetryPolicy_GetRetryDelay(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay:  time.Second,
		BackoffFactor: 2.0,
		MaxDelay:      10 * time.Second,
	}

	tests := []struct {
		retryCount int
		expected   time.Duration
	}{
		{0, time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 10 * time.Second}, // Should be capped at MaxDelay
		{5, 10 * time.Second}, // Should remain at MaxDelay
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, policy.GetRetryDelay(test.retryCount), 
			"GetRetryDelay(%d) mismatch", test.retryCount)
	}
}

func TestRetryPolicy_IsRetryable(t *testing.T) {
	policy := &RetryPolicy{
		RetryableErrors: []string{
			"connection timeout",
			"temporary failure",
			"network unreachable",
		},
	}

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"retryable error", errors.New("connection timeout occurred"), true},
		{"another retryable error", errors.New("temporary failure detected"), true},
		{"non-retryable error", errors.New("file not found"), false},
		{"partial match", errors.New("network"), false}, // Should not match partial strings
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, policy.IsRetryable(test.err), 
				"IsRetryable(%v) mismatch", test.err)
		})
	}
}

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	require.NotNil(t, policy, "DefaultRetryPolicy() returned nil")
	assert.Greater(t, policy.MaxRetries, 0, "MaxRetries should be positive")
	assert.Greater(t, policy.InitialDelay, time.Duration(0), "InitialDelay should be positive")
	assert.Greater(t, policy.BackoffFactor, 1.0, "BackoffFactor should be greater than 1.0")
	assert.Greater(t, policy.MaxDelay, policy.InitialDelay, "MaxDelay should be greater than InitialDelay")
	assert.NotEmpty(t, policy.RetryableErrors, "RetryableErrors should not be empty")
}

func TestErrorConstants(t *testing.T) {
	// Test that error constants are defined and not nil
	errors := []error{
		ErrTransferNotFound,
		ErrInvalidStateTransition,
		ErrTransferAlreadyExists,
		ErrSessionNotFound,
		ErrSessionAlreadyExists,
		ErrMaxTransfersExceeded,
		ErrInvalidConfiguration,
		ErrTransferCancelled,
	}

	for i, err := range errors {
		assert.NotNil(t, err, "Error constant at index %d is nil", i)
		assert.NotEmpty(t, err.Error(), "Error constant at index %d has empty message", i)
	}
}

// Test helper function for contains
func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "hello", true},
		{"hello world", "world", true},
		{"hello world", "lo wo", true},
		{"hello world", "xyz", false},
		{"", "", true},
		{"hello", "", true},
		{"", "hello", false},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, contains(test.s, test.substr), 
			"contains(%q, %q) mismatch", test.s, test.substr)
	}
}
