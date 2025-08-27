package transfer

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRetryScheduler(t *testing.T) {
	manager := createTestUnifiedTransferManager()
	errorHandler := NewDefaultErrorHandler(nil)

	scheduler := NewRetryScheduler(manager, errorHandler)

	require.NotNil(t, scheduler)
	assert.Equal(t, manager, scheduler.manager)
	assert.Equal(t, errorHandler, scheduler.errorHandler)
	assert.NotNil(t, scheduler.retryQueue)
	assert.NotNil(t, scheduler.ctx)
}

func TestRetryScheduler_ScheduleRetry(t *testing.T) {
	manager := createTestUnifiedTransferManager()
	errorHandler := NewDefaultErrorHandler(nil)
	scheduler := NewRetryScheduler(manager, errorHandler)

	t.Run("schedule_recoverable_error", func(t *testing.T) {
		err := errors.New("connection timeout")
		filePath := "test.txt"
		retryCount := 1

		scheduled := scheduler.ScheduleRetry(filePath, err, retryCount)

		assert.True(t, scheduled)

		task, exists := scheduler.GetRetryStatus(filePath)
		require.True(t, exists)
		assert.Equal(t, filePath, task.FilePath)
		assert.Equal(t, retryCount, task.RetryCount)
		assert.Equal(t, err, task.LastError)
		assert.True(t, task.NextAttempt.After(time.Now()))
	})

	t.Run("reject_non_recoverable_error", func(t *testing.T) {
		err := errors.New("file not found")
		filePath := "missing.txt"
		retryCount := 1

		scheduled := scheduler.ScheduleRetry(filePath, err, retryCount)

		assert.False(t, scheduled)

		_, exists := scheduler.GetRetryStatus(filePath)
		assert.False(t, exists)
	})

	t.Run("reject_max_retries_exceeded", func(t *testing.T) {
		err := errors.New("connection timeout")
		filePath := "test.txt"
		retryCount := 5 // Exceeds default max retries (3)

		scheduled := scheduler.ScheduleRetry(filePath, err, retryCount)

		assert.False(t, scheduled)
	})

	t.Run("update_existing_task", func(t *testing.T) {
		err1 := errors.New("connection timeout")
		err2 := errors.New("network unreachable")
		filePath := "test.txt"

		// Schedule first retry
		scheduled1 := scheduler.ScheduleRetry(filePath, err1, 1)
		assert.True(t, scheduled1)

		// Schedule second retry (should update existing task)
		scheduled2 := scheduler.ScheduleRetry(filePath, err2, 2)
		assert.True(t, scheduled2)

		task, exists := scheduler.GetRetryStatus(filePath)
		require.True(t, exists)
		assert.Equal(t, 2, task.RetryCount)
		assert.Equal(t, err2, task.LastError)
		assert.Len(t, task.ErrorContext.ErrorHistory, 2) // Should have both different errors
	})
}

func TestRetryScheduler_CancelRetry(t *testing.T) {
	manager := createTestUnifiedTransferManager()
	errorHandler := NewDefaultErrorHandler(nil)
	scheduler := NewRetryScheduler(manager, errorHandler)

	err := errors.New("connection timeout")
	filePath := "test.txt"

	// Schedule a retry
	scheduled := scheduler.ScheduleRetry(filePath, err, 1)
	require.True(t, scheduled)

	// Verify it exists
	_, exists := scheduler.GetRetryStatus(filePath)
	assert.True(t, exists)

	// Cancel the retry
	scheduler.CancelRetry(filePath)

	// Verify it's removed
	_, exists = scheduler.GetRetryStatus(filePath)
	assert.False(t, exists)
}

func TestRetryScheduler_GetAllRetryTasks(t *testing.T) {
	manager := createTestUnifiedTransferManager()
	errorHandler := NewDefaultErrorHandler(nil)
	scheduler := NewRetryScheduler(manager, errorHandler)

	// Schedule multiple retries (use retry counts within limit)
	files := []string{"file1.txt", "file2.txt", "file3.txt"}
	err := errors.New("connection timeout")
	retryCounts := []int{1, 2, 1} // All within the default max retries (3)

	for i, filePath := range files {
		scheduled := scheduler.ScheduleRetry(filePath, err, retryCounts[i])
		require.True(t, scheduled)
	}

	// Get all tasks
	allTasks := scheduler.GetAllRetryTasks()

	assert.Len(t, allTasks, 3)
	for _, filePath := range files {
		task, exists := allTasks[filePath]
		require.True(t, exists)
		assert.Equal(t, filePath, task.FilePath)
	}
}

func TestRetryScheduler_GetRetryStatistics(t *testing.T) {
	manager := createTestUnifiedTransferManager()
	errorHandler := NewDefaultErrorHandler(nil)
	scheduler := NewRetryScheduler(manager, errorHandler)

	// Schedule some retries with different retry counts (within limit)
	err := errors.New("connection timeout")
	scheduler.ScheduleRetry("file1.txt", err, 1)
	scheduler.ScheduleRetry("file2.txt", err, 2)
	scheduler.ScheduleRetry("file3.txt", err, 1) // Changed from 3 to 1

	stats := scheduler.GetRetryStatistics()

	assert.Equal(t, 3, stats.TotalScheduled)
	assert.Equal(t, 3, stats.PendingRetries) // All should be pending
	assert.Equal(t, 0, stats.OverdueRetries)
	assert.Equal(t, 2, stats.MaxRetryCount)           // Max is now 2
	assert.Equal(t, 4, stats.TotalRetryCount)         // 1+2+1 = 4
	assert.Equal(t, 4.0/3.0, stats.AverageRetryCount) // 4/3
}

func TestRetryScheduler_StartStop(t *testing.T) {
	manager := createTestUnifiedTransferManager()
	errorHandler := NewDefaultErrorHandler(nil)
	scheduler := NewRetryScheduler(manager, errorHandler)

	// Start the scheduler
	scheduler.Start()

	// Verify it's running by checking context
	select {
	case <-scheduler.ctx.Done():
		t.Fatal("Scheduler context should not be done after start")
	default:
		// Good, context is not done
	}

	// Stop the scheduler
	scheduler.Stop()

	// Verify it's stopped
	select {
	case <-scheduler.ctx.Done():
		// Good, context is done
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Scheduler context should be done after stop")
	}
}

func TestRetryTask_ErrorContext(t *testing.T) {
	t.Run("add_errors", func(t *testing.T) {
		ctx := &ErrorContext{}

		err1 := errors.New("error 1")
		err2 := errors.New("error 2")

		ctx.AddError(err1)
		ctx.AddError(err2)

		assert.Len(t, ctx.ErrorHistory, 2)
		assert.Equal(t, err1, ctx.ErrorHistory[0])
		assert.Equal(t, err2, ctx.ErrorHistory[1])
	})

	t.Run("error_patterns", func(t *testing.T) {
		tests := []struct {
			name     string
			errors   []error
			expected string
		}{
			{
				name:     "no_errors",
				errors:   []error{},
				expected: "no_errors",
			},
			{
				name:     "single_error",
				errors:   []error{errors.New("test")},
				expected: "single_error",
			},
			{
				name: "repeated_error",
				errors: []error{
					errors.New("same"),
					errors.New("same"),
					errors.New("same"),
				},
				expected: "repeated_error",
			},
			{
				name: "mixed_errors",
				errors: []error{
					errors.New("error1"),
					errors.New("error2"),
					errors.New("error3"),
				},
				expected: "mixed_errors",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				ctx := &ErrorContext{}
				for _, err := range test.errors {
					ctx.AddError(err)
				}

				pattern := ctx.GetErrorPattern()
				assert.Equal(t, test.expected, pattern)
			})
		}
	})

	t.Run("should_escalate", func(t *testing.T) {
		tests := []struct {
			name       string
			errors     []error
			retryCount int
			maxRetries int
			expected   bool
		}{
			{
				name: "repeated_errors_should_escalate",
				errors: []error{
					errors.New("same"),
					errors.New("same"),
					errors.New("same"),
				},
				retryCount: 2,
				maxRetries: 5,
				expected:   true,
			},
			{
				name: "mixed_errors_at_limit",
				errors: []error{
					errors.New("error1"),
					errors.New("error2"),
				},
				retryCount: 5,
				maxRetries: 5,
				expected:   true,
			},
			{
				name: "mixed_errors_below_limit",
				errors: []error{
					errors.New("error1"),
					errors.New("error2"),
				},
				retryCount: 2,
				maxRetries: 5,
				expected:   false,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				ctx := &ErrorContext{
					RetryCount: test.retryCount,
					MaxRetries: test.maxRetries,
				}

				for _, err := range test.errors {
					ctx.AddError(err)
				}

				result := ctx.ShouldEscalate()
				assert.Equal(t, test.expected, result)
			})
		}
	})
}

// Helper function to create a test UnifiedTransferManager
func createTestUnifiedTransferManager() *UnifiedTransferManager {
	session := &TransferSession{
		ServiceID:       "test-service",
		SessionID:       "test-session",
		SessionCreateAt: time.Now().Unix(),
	}

	config := DefaultTransferConfig()

	// Create a minimal manager without starting the retry scheduler
	utm := &UnifiedTransferManager{
		session:        session,
		config:         config,
		structure:      NewFileStructureManager(),
		chunkers:       make(map[string]*Chunker),
		pendingFiles:   make(map[string]bool),
		completedFiles: make(map[string]bool),
		failedFiles:    make(map[string]bool),
		sessionStatus: &SessionTransferStatus{
			SessionID:      session.ServiceID,
			State:          StatusSessionStateActive,
			StartTime:      time.Now(),
			LastUpdateTime: time.Now(),
		},
		listeners: make([]StatusListener, 0),
	}

	utm.errorHandler = NewDefaultErrorHandler(config.DefaultRetryPolicy)

	return utm
}
