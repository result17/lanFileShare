package transfer

import (
	"errors"
	"testing"
	"time"
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
		if got := test.state.String(); got != test.expected {
			t.Errorf("TransferState(%d).String() = %q, want %q", test.state, got, test.expected)
		}
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
		if got := test.state.IsTerminal(); got != test.expected {
			t.Errorf("TransferState(%s).IsTerminal() = %v, want %v", test.state, got, test.expected)
		}
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
		if got := test.from.CanTransitionTo(test.to); got != test.expected {
			t.Errorf("TransferState(%s).CanTransitionTo(%s) = %v, want %v",
				test.from, test.to, got, test.expected)
		}
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
			if got := status.GetProgressPercentage(); got != test.expected {
				t.Errorf("GetProgressPercentage() = %f, want %f", got, test.expected)
			}
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
			if got := status.GetRemainingBytes(); got != test.expected {
				t.Errorf("GetRemainingBytes() = %d, want %d", got, test.expected)
			}
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
			if got := status.IsComplete(); got != test.expected {
				t.Errorf("IsComplete() = %v, want %v", got, test.expected)
			}
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
	if status.BytesSent != 500 {
		t.Errorf("BytesSent = %d, want 500", status.BytesSent)
	}
	if status.ChunksSent != 5 {
		t.Errorf("ChunksSent = %d, want 5", status.ChunksSent)
	}
	if status.LastUpdateTime.Before(startTime) {
		t.Error("LastUpdateTime should be updated to current time")
	}

	// Check that transfer rate was calculated
	if status.TransferRate <= 0 {
		t.Error("TransferRate should be calculated and positive")
	}

	// Check that ETA was calculated
	if status.ETA <= 0 {
		t.Error("ETA should be calculated and positive")
	}
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
			if got := progress.GetCompletionPercentage(); got != test.expected {
				t.Errorf("GetCompletionPercentage() = %f, want %f", got, test.expected)
			}
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
		if got := test.state.String(); got != test.expected {
			t.Errorf("StatusSessionState(%d).String() = %q, want %q", test.state, got, test.expected)
		}
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
		if got := policy.GetRetryDelay(test.retryCount); got != test.expected {
			t.Errorf("GetRetryDelay(%d) = %v, want %v", test.retryCount, got, test.expected)
		}
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
			if got := policy.IsRetryable(test.err); got != test.expected {
				t.Errorf("IsRetryable(%v) = %v, want %v", test.err, got, test.expected)
			}
		})
	}
}

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy == nil {
		t.Fatal("DefaultRetryPolicy() returned nil")
	}

	if policy.MaxRetries <= 0 {
		t.Error("MaxRetries should be positive")
	}

	if policy.InitialDelay <= 0 {
		t.Error("InitialDelay should be positive")
	}

	if policy.BackoffFactor <= 1.0 {
		t.Error("BackoffFactor should be greater than 1.0")
	}

	if policy.MaxDelay <= policy.InitialDelay {
		t.Error("MaxDelay should be greater than InitialDelay")
	}

	if len(policy.RetryableErrors) == 0 {
		t.Error("RetryableErrors should not be empty")
	}
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
		if err == nil {
			t.Errorf("Error constant at index %d is nil", i)
		}
		if err.Error() == "" {
			t.Errorf("Error constant at index %d has empty message", i)
		}
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
		if got := contains(test.s, test.substr); got != test.expected {
			t.Errorf("contains(%q, %q) = %v, want %v", test.s, test.substr, got, test.expected)
		}
	}
}
