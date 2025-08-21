package transfer

import (
	"errors"
	"log/slog"
	"strings"
	"time"
)

// ErrorCategory represents the category of an error for handling purposes
type ErrorCategory int

const (
	// ErrorCategoryRecoverable indicates errors that can be retried
	ErrorCategoryRecoverable ErrorCategory = iota
	// ErrorCategoryNonRecoverable indicates errors that should not be retried
	ErrorCategoryNonRecoverable
	// ErrorCategorySystem indicates system-level errors that may require special handling
	ErrorCategorySystem
)

// ErrorAction represents the action to take when an error occurs
type ErrorAction int

const (
	// ErrorActionRetry indicates the operation should be retried
	ErrorActionRetry ErrorAction = iota
	// ErrorActionFail indicates the transfer should be marked as failed
	ErrorActionFail
	// ErrorActionPause indicates the transfer should be paused for manual intervention
	ErrorActionPause
	// ErrorActionCancel indicates the entire session should be cancelled
	ErrorActionCancel
)

// String returns a string representation of ErrorAction
func (ea ErrorAction) String() string {
	switch ea {
	case ErrorActionRetry:
		return "retry"
	case ErrorActionFail:
		return "fail"
	case ErrorActionPause:
		return "pause"
	case ErrorActionCancel:
		return "cancel"
	default:
		return "unknown"
	}
}

// ErrorHandler defines the interface for handling transfer errors
type ErrorHandler interface {
	// HandleError determines what action to take for a given error
	HandleError(filePath string, err error, retryCount int) ErrorAction

	// CategorizeError determines the category of an error
	CategorizeError(err error) ErrorCategory

	// IsRetryable checks if an error should trigger a retry
	IsRetryable(err error, retryCount int, maxRetries int) bool

	// GetRetryDelay calculates the delay before the next retry attempt
	GetRetryDelay(retryCount int, policy *RetryPolicy) time.Duration

	// LogError logs an error with appropriate context
	LogError(filePath string, err error, action ErrorAction, retryCount int)
}

// DefaultErrorHandler provides a default implementation of ErrorHandler
type DefaultErrorHandler struct {
	retryPolicy *RetryPolicy
}

// NewDefaultErrorHandler creates a new DefaultErrorHandler with the given retry policy
func NewDefaultErrorHandler(retryPolicy *RetryPolicy) *DefaultErrorHandler {
	if retryPolicy == nil {
		retryPolicy = DefaultRetryPolicy()
	}
	return &DefaultErrorHandler{
		retryPolicy: retryPolicy,
	}
}

// HandleError determines what action to take for a given error
func (h *DefaultErrorHandler) HandleError(filePath string, err error, retryCount int) ErrorAction {
	if err == nil {
		return ErrorActionRetry // Should not happen, but safe default
	}

	category := h.CategorizeError(err)

	switch category {
	case ErrorCategoryRecoverable:
		if h.IsRetryable(err, retryCount, h.retryPolicy.MaxRetries) {
			return ErrorActionRetry
		}
		return ErrorActionFail

	case ErrorCategoryNonRecoverable:
		return ErrorActionFail

	case ErrorCategorySystem:
		// System errors might require pausing to allow manual intervention
		if retryCount == 0 {
			return ErrorActionRetry // Try once
		}
		return ErrorActionPause

	default:
		return ErrorActionFail
	}
}

// CategorizeError determines the category of an error
func (h *DefaultErrorHandler) CategorizeError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryRecoverable
	}

	errMsg := strings.ToLower(err.Error())

	// Check for non-recoverable errors first
	nonRecoverablePatterns := []string{
		"file not found",
		"no such file",
		"permission denied",
		"access denied",
		"disk full",
		"no space left",
		"invalid file format",
		"authentication failed",
		"unauthorized",
		"forbidden",
		"file too large",
		"invalid checksum",
		"corrupted",
	}

	for _, pattern := range nonRecoverablePatterns {
		if strings.Contains(errMsg, pattern) {
			return ErrorCategoryNonRecoverable
		}
	}

	// Check for system errors
	systemErrorPatterns := []string{
		"out of memory",
		"database connection",
		"configuration error",
		"service unavailable",
		"internal server error",
		"panic",
		"deadlock",
	}

	for _, pattern := range systemErrorPatterns {
		if strings.Contains(errMsg, pattern) {
			return ErrorCategorySystem
		}
	}

	// Check for recoverable errors
	recoverablePatterns := []string{
		"connection timeout",
		"temporary failure",
		"network unreachable",
		"connection reset",
		"connection refused",
		"timeout",
		"rate limit",
		"throttled",
		"busy",
		"try again",
		"temporary",
	}

	for _, pattern := range recoverablePatterns {
		if strings.Contains(errMsg, pattern) {
			return ErrorCategoryRecoverable
		}
	}

	// Check for specific error types
	switch {
	case errors.Is(err, ErrTransferNotFound):
		return ErrorCategoryNonRecoverable
	case errors.Is(err, ErrInvalidStateTransition):
		return ErrorCategoryNonRecoverable
	case errors.Is(err, ErrTransferAlreadyExists):
		return ErrorCategoryNonRecoverable
	case errors.Is(err, ErrSessionNotFound):
		return ErrorCategoryNonRecoverable
	case errors.Is(err, ErrMaxTransfersExceeded):
		return ErrorCategoryRecoverable // Might be able to retry later
	case errors.Is(err, ErrInvalidConfiguration):
		return ErrorCategorySystem
	case errors.Is(err, ErrTransferCancelled):
		return ErrorCategoryNonRecoverable
	}

	// Default to recoverable for unknown errors (conservative approach)
	return ErrorCategoryRecoverable
}

// IsRetryable checks if an error should trigger a retry
func (h *DefaultErrorHandler) IsRetryable(err error, retryCount int, maxRetries int) bool {
	if err == nil {
		return false
	}

	if retryCount >= maxRetries {
		return false
	}

	category := h.CategorizeError(err)
	return category == ErrorCategoryRecoverable
}

// GetRetryDelay calculates the delay before the next retry attempt
func (h *DefaultErrorHandler) GetRetryDelay(retryCount int, policy *RetryPolicy) time.Duration {
	if policy == nil {
		policy = h.retryPolicy
	}
	return policy.GetRetryDelay(retryCount)
}

// LogError logs an error with appropriate context
func (h *DefaultErrorHandler) LogError(filePath string, err error, action ErrorAction, retryCount int) {
	category := h.CategorizeError(err)

	logFields := []interface{}{
		"file", filePath,
		"error", err,
		"action", action.String(),
		"retry_count", retryCount,
		"category", category,
	}

	switch action {
	case ErrorActionRetry:
		slog.Warn("Transfer error, will retry", logFields...)
	case ErrorActionFail:
		slog.Error("Transfer failed", logFields...)
	case ErrorActionPause:
		slog.Warn("Transfer paused due to error", logFields...)
	case ErrorActionCancel:
		slog.Error("Transfer cancelled due to error", logFields...)
	default:
		slog.Error("Transfer error with unknown action", logFields...)
	}
}

// ErrorContext provides additional context for error handling
type ErrorContext struct {
	FilePath     string
	SessionID    string
	RetryCount   int
	MaxRetries   int
	LastAttempt  time.Time
	TotalBytes   int64
	BytesSent    int64
	ErrorHistory []error
}

// AddError adds an error to the error history
func (ec *ErrorContext) AddError(err error) {
	ec.ErrorHistory = append(ec.ErrorHistory, err)
	ec.LastAttempt = time.Now()

	// Keep only the last 10 errors to prevent memory growth
	if len(ec.ErrorHistory) > 10 {
		ec.ErrorHistory = ec.ErrorHistory[len(ec.ErrorHistory)-10:]
	}
}

// GetErrorPattern analyzes the error history to detect patterns
func (ec *ErrorContext) GetErrorPattern() string {
	if len(ec.ErrorHistory) == 0 {
		return "no_errors"
	}

	if len(ec.ErrorHistory) == 1 {
		return "single_error"
	}

	// Check if all recent errors are the same
	lastError := ec.ErrorHistory[len(ec.ErrorHistory)-1].Error()
	sameErrorCount := 1

	for i := len(ec.ErrorHistory) - 2; i >= 0 && sameErrorCount < 3; i-- {
		if ec.ErrorHistory[i].Error() == lastError {
			sameErrorCount++
		} else {
			break
		}
	}

	if sameErrorCount >= 3 {
		return "repeated_error"
	}

	return "mixed_errors"
}

// ShouldEscalate determines if the error should be escalated based on patterns
func (ec *ErrorContext) ShouldEscalate() bool {
	pattern := ec.GetErrorPattern()

	switch pattern {
	case "repeated_error":
		return ec.RetryCount >= 2 // Escalate after 2 repeated errors
	case "mixed_errors":
		return ec.RetryCount >= ec.MaxRetries // Use normal retry limit
	default:
		return false
	}
}
