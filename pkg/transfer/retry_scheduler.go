package transfer

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// RetryScheduler manages automatic retry scheduling for failed transfers
type RetryScheduler struct {
	mu           sync.RWMutex
	retryQueue   map[string]*RetryTask
	errorHandler ErrorHandler
	manager      *UnifiedTransferManager
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// RetryTask represents a scheduled retry operation
type RetryTask struct {
	FilePath     string
	RetryCount   int
	NextAttempt  time.Time
	LastError    error
	ErrorContext *ErrorContext
	Timer        *time.Timer
}

// NewRetryScheduler creates a new retry scheduler
func NewRetryScheduler(manager *UnifiedTransferManager, errorHandler ErrorHandler) *RetryScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &RetryScheduler{
		retryQueue:   make(map[string]*RetryTask),
		errorHandler: errorHandler,
		manager:      manager,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins the retry scheduler background processing
func (rs *RetryScheduler) Start() {
	rs.wg.Add(1)
	go rs.processRetries()
}

// Stop gracefully shuts down the retry scheduler
func (rs *RetryScheduler) Stop() {
	rs.cancel()

	// Cancel all pending timers
	rs.mu.Lock()
	for _, task := range rs.retryQueue {
		if task.Timer != nil {
			task.Timer.Stop()
		}
	}
	rs.mu.Unlock()

	rs.wg.Wait()
}

// ScheduleRetry schedules a retry for a failed transfer
func (rs *RetryScheduler) ScheduleRetry(filePath string, err error, retryCount int) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Check if we should retry this error
	action := rs.errorHandler.HandleError(filePath, err, retryCount)
	if action != ErrorActionRetry {
		rs.errorHandler.LogError(filePath, err, action, retryCount)
		return false
	}

	// Get retry delay
	retryPolicy := rs.manager.config.DefaultRetryPolicy
	delay := rs.errorHandler.GetRetryDelay(retryCount, retryPolicy)
	nextAttempt := time.Now().Add(delay)

	// Create or update retry task
	task, exists := rs.retryQueue[filePath]
	if !exists {
		task = &RetryTask{
			FilePath: filePath,
			ErrorContext: &ErrorContext{
				FilePath:   filePath,
				SessionID:  rs.manager.sessionStatus.SessionID,
				MaxRetries: retryPolicy.MaxRetries,
			},
		}
		rs.retryQueue[filePath] = task
	}

	// Update task
	task.RetryCount = retryCount
	task.NextAttempt = nextAttempt
	task.LastError = err

	// Only add error if it's different from the last one to avoid duplicates
	if len(task.ErrorContext.ErrorHistory) == 0 ||
		task.ErrorContext.ErrorHistory[len(task.ErrorContext.ErrorHistory)-1].Error() != err.Error() {
		task.ErrorContext.AddError(err)
	}
	task.ErrorContext.RetryCount = retryCount

	// Cancel existing timer if any
	if task.Timer != nil {
		task.Timer.Stop()
	}

	// Schedule new timer
	task.Timer = time.AfterFunc(delay, func() {
		rs.executeRetry(filePath)
	})

	slog.Info("Scheduled retry",
		"file", filePath,
		"retry_count", retryCount,
		"delay", delay,
		"next_attempt", nextAttempt)

	return true
}

// CancelRetry cancels a scheduled retry for a file
func (rs *RetryScheduler) CancelRetry(filePath string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	task, exists := rs.retryQueue[filePath]
	if !exists {
		return
	}

	if task.Timer != nil {
		task.Timer.Stop()
	}

	delete(rs.retryQueue, filePath)

	slog.Info("Cancelled retry", "file", filePath)
}

// GetRetryStatus returns the retry status for a file
func (rs *RetryScheduler) GetRetryStatus(filePath string) (*RetryTask, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	task, exists := rs.retryQueue[filePath]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	taskCopy := *task
	return &taskCopy, true
}

// GetAllRetryTasks returns all currently scheduled retry tasks
func (rs *RetryScheduler) GetAllRetryTasks() map[string]*RetryTask {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	result := make(map[string]*RetryTask)
	for filePath, task := range rs.retryQueue {
		taskCopy := *task
		result[filePath] = &taskCopy
	}

	return result
}

// executeRetry executes a scheduled retry
func (rs *RetryScheduler) executeRetry(filePath string) {
	rs.mu.Lock()
	task, exists := rs.retryQueue[filePath]
	if !exists {
		rs.mu.Unlock()
		return
	}

	// Remove from queue (will be re-added if retry fails again)
	delete(rs.retryQueue, filePath)
	rs.mu.Unlock()

	slog.Info("Executing retry",
		"file", filePath,
		"retry_count", task.RetryCount,
		"last_error", task.LastError)

	// Check if we should escalate based on error patterns
	if task.ErrorContext.ShouldEscalate() {
		slog.Warn("Escalating retry due to error pattern",
			"file", filePath,
			"pattern", task.ErrorContext.GetErrorPattern(),
			"retry_count", task.RetryCount)

		// Mark as failed instead of retrying
		rs.manager.FailTransfer(filePath, task.LastError)
		return
	}

	// Attempt to restart the transfer
	err := rs.manager.StartTransfer(filePath)
	if err != nil {
		// Retry failed, schedule another retry if appropriate
		newRetryCount := task.RetryCount + 1
		if !rs.ScheduleRetry(filePath, err, newRetryCount) {
			// No more retries, mark as failed
			rs.manager.FailTransfer(filePath, err)
		}
		return
	}

	slog.Info("Retry successful", "file", filePath, "retry_count", task.RetryCount)
}

// processRetries is the main background processing loop
func (rs *RetryScheduler) processRetries() {
	defer rs.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Cleanup interval
	defer ticker.Stop()

	for {
		select {
		case <-rs.ctx.Done():
			return

		case <-ticker.C:
			rs.cleanupExpiredTasks()
		}
	}
}

// cleanupExpiredTasks removes tasks that have been in the queue too long
func (rs *RetryScheduler) cleanupExpiredTasks() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	now := time.Now()
	maxAge := 24 * time.Hour // Tasks older than 24 hours are cleaned up

	for filePath, task := range rs.retryQueue {
		if task.ErrorContext.LastAttempt.Add(maxAge).Before(now) {
			if task.Timer != nil {
				task.Timer.Stop()
			}
			delete(rs.retryQueue, filePath)

			slog.Info("Cleaned up expired retry task",
				"file", filePath,
				"last_attempt", task.ErrorContext.LastAttempt)
		}
	}
}

// GetRetryStatistics returns statistics about the retry scheduler
func (rs *RetryScheduler) GetRetryStatistics() RetryStatistics {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	stats := RetryStatistics{
		TotalScheduled: len(rs.retryQueue),
	}

	now := time.Now()
	for _, task := range rs.retryQueue {
		if task.NextAttempt.After(now) {
			stats.PendingRetries++
		} else {
			stats.OverdueRetries++
		}

		if task.RetryCount > stats.MaxRetryCount {
			stats.MaxRetryCount = task.RetryCount
		}

		stats.TotalRetryCount += task.RetryCount
	}

	if len(rs.retryQueue) > 0 {
		stats.AverageRetryCount = float64(stats.TotalRetryCount) / float64(len(rs.retryQueue))
	}

	return stats
}

// RetryStatistics provides statistics about retry operations
type RetryStatistics struct {
	TotalScheduled    int     `json:"total_scheduled"`
	PendingRetries    int     `json:"pending_retries"`
	OverdueRetries    int     `json:"overdue_retries"`
	MaxRetryCount     int     `json:"max_retry_count"`
	TotalRetryCount   int     `json:"total_retry_count"`
	AverageRetryCount float64 `json:"average_retry_count"`
}

// RetrySchedulerConfig provides configuration for the retry scheduler
type RetrySchedulerConfig struct {
	MaxConcurrentRetries int           `json:"max_concurrent_retries"`
	CleanupInterval      time.Duration `json:"cleanup_interval"`
	MaxTaskAge           time.Duration `json:"max_task_age"`
	EnableEscalation     bool          `json:"enable_escalation"`
}

// DefaultRetrySchedulerConfig returns default configuration for the retry scheduler
func DefaultRetrySchedulerConfig() *RetrySchedulerConfig {
	return &RetrySchedulerConfig{
		MaxConcurrentRetries: 5,
		CleanupInterval:      30 * time.Second,
		MaxTaskAge:           24 * time.Hour,
		EnableEscalation:     true,
	}
}
