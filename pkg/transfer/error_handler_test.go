package transfer

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultErrorHandler_CategorizeError(t *testing.T) {
	handler := NewDefaultErrorHandler(nil)

	tests := []struct {
		name     string
		err      error
		expected ErrorCategory
	}{
		// Recoverable errors
		{
			name:     "connection timeout",
			err:      errors.New("connection timeout"),
			expected: ErrorCategoryRecoverable,
		},
		{
			name:     "network unreachable",
			err:      errors.New("network unreachable"),
			expected: ErrorCategoryRecoverable,
		},
		{
			name:     "temporary failure",
			err:      errors.New("temporary failure"),
			expected: ErrorCategoryRecoverable,
		},
		{
			name:     "rate limit exceeded",
			err:      errors.New("rate limit exceeded"),
			expected: ErrorCategoryRecoverable,
		},

		// Non-recoverable errors
		{
			name:     "file not found",
			err:      errors.New("file not found"),
			expected: ErrorCategoryNonRecoverable,
		},
		{
			name:     "permission denied",
			err:      errors.New("permission denied"),
			expected: ErrorCategoryNonRecoverable,
		},
		{
			name:     "disk full",
			err:      errors.New("disk full"),
			expected: ErrorCategoryNonRecoverable,
		},
		{
			name:     "authentication failed",
			err:      errors.New("authentication failed"),
			expected: ErrorCategoryNonRecoverable,
		},

		// System errors
		{
			name:     "out of memory",
			err:      errors.New("out of memory"),
			expected: ErrorCategorySystem,
		},
		{
			name:     "database connection failed",
			err:      errors.New("database connection failed"),
			expected: ErrorCategorySystem,
		},
		{
			name:     "configuration error",
			err:      errors.New("configuration error"),
			expected: ErrorCategorySystem,
		},

		// Specific error types
		{
			name:     "transfer not found",
			err:      ErrTransferNotFound,
			expected: ErrorCategoryNonRecoverable,
		},
		{
			name:     "max transfers exceeded",
			err:      ErrMaxTransfersExceeded,
			expected: ErrorCategoryRecoverable,
		},
		{
			name:     "invalid configuration",
			err:      ErrInvalidConfiguration,
			expected: ErrorCategorySystem,
		},

		// Unknown errors (should default to recoverable)
		{
			name:     "unknown error",
			err:      errors.New("some unknown error"),
			expected: ErrorCategoryRecoverable,
		},

		// Nil error
		{
			name:     "nil error",
			err:      nil,
			expected: ErrorCategoryRecoverable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			category := handler.CategorizeError(test.err)
			assert.Equal(t, test.expected, category,
				"CategorizeError(%v) = %v, expected %v", test.err, category, test.expected)
		})
	}
}

func TestDefaultErrorHandler_HandleError(t *testing.T) {
	retryPolicy := &RetryPolicy{
		MaxRetries:    3,
		InitialDelay:  time.Second,
		BackoffFactor: 2.0,
		MaxDelay:      30 * time.Second,
	}
	handler := NewDefaultErrorHandler(retryPolicy)

	tests := []struct {
		name       string
		err        error
		retryCount int
		expected   ErrorAction
	}{
		// Recoverable errors within retry limit
		{
			name:       "recoverable error, first attempt",
			err:        errors.New("connection timeout"),
			retryCount: 0,
			expected:   ErrorActionRetry,
		},
		{
			name:       "recoverable error, second attempt",
			err:        errors.New("network unreachable"),
			retryCount: 1,
			expected:   ErrorActionRetry,
		},
		{
			name:       "recoverable error, at retry limit",
			err:        errors.New("temporary failure"),
			retryCount: 3,
			expected:   ErrorActionFail,
		},

		// Non-recoverable errors
		{
			name:       "non-recoverable error",
			err:        errors.New("file not found"),
			retryCount: 0,
			expected:   ErrorActionFail,
		},
		{
			name:       "permission denied",
			err:        errors.New("permission denied"),
			retryCount: 1,
			expected:   ErrorActionFail,
		},

		// System errors
		{
			name:       "system error, first attempt",
			err:        errors.New("out of memory"),
			retryCount: 0,
			expected:   ErrorActionRetry,
		},
		{
			name:       "system error, second attempt",
			err:        errors.New("database connection failed"),
			retryCount: 1,
			expected:   ErrorActionPause,
		},

		// Nil error
		{
			name:       "nil error",
			err:        nil,
			retryCount: 0,
			expected:   ErrorActionRetry,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := handler.HandleError("test.txt", test.err, test.retryCount)
			assert.Equal(t, test.expected, action,
				"HandleError(%v, %d) = %v, expected %v", test.err, test.retryCount, action, test.expected)
		})
	}
}

func TestDefaultErrorHandler_IsRetryable(t *testing.T) {
	handler := NewDefaultErrorHandler(nil)

	tests := []struct {
		name       string
		err        error
		retryCount int
		maxRetries int
		expected   bool
	}{
		{
			name:       "recoverable error within limit",
			err:        errors.New("connection timeout"),
			retryCount: 1,
			maxRetries: 3,
			expected:   true,
		},
		{
			name:       "recoverable error at limit",
			err:        errors.New("network unreachable"),
			retryCount: 3,
			maxRetries: 3,
			expected:   false,
		},
		{
			name:       "non-recoverable error",
			err:        errors.New("file not found"),
			retryCount: 0,
			maxRetries: 3,
			expected:   false,
		},
		{
			name:       "nil error",
			err:        nil,
			retryCount: 0,
			maxRetries: 3,
			expected:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := handler.IsRetryable(test.err, test.retryCount, test.maxRetries)
			assert.Equal(t, test.expected, result,
				"IsRetryable(%v, %d, %d) = %v, expected %v",
				test.err, test.retryCount, test.maxRetries, result, test.expected)
		})
	}
}

func TestDefaultErrorHandler_GetRetryDelay(t *testing.T) {
	retryPolicy := &RetryPolicy{
		InitialDelay:  time.Second,
		BackoffFactor: 2.0,
		MaxDelay:      10 * time.Second,
	}
	handler := NewDefaultErrorHandler(retryPolicy)

	tests := []struct {
		retryCount int
		expected   time.Duration
	}{
		{0, time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 10 * time.Second}, // Capped at MaxDelay
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("retry_count_%d", test.retryCount), func(t *testing.T) {
			delay := handler.GetRetryDelay(test.retryCount, retryPolicy)
			assert.Equal(t, test.expected, delay,
				"GetRetryDelay(%d) = %v, expected %v", test.retryCount, delay, test.expected)
		})
	}
}

func TestErrorContext(t *testing.T) {
	t.Run("add_error", func(t *testing.T) {
		ctx := &ErrorContext{}

		err1 := errors.New("first error")
		ctx.AddError(err1)

		assert.Len(t, ctx.ErrorHistory, 1)
		assert.Equal(t, err1, ctx.ErrorHistory[0])
		assert.False(t, ctx.LastAttempt.IsZero())
	})

	t.Run("error_history_limit", func(t *testing.T) {
		ctx := &ErrorContext{}

		// Add more than 10 errors
		for i := 0; i < 15; i++ {
			ctx.AddError(errors.New(fmt.Sprintf("error %d", i)))
		}

		// Should keep only the last 10
		assert.Len(t, ctx.ErrorHistory, 10)
		assert.Equal(t, "error 14", ctx.ErrorHistory[9].Error())
		assert.Equal(t, "error 5", ctx.ErrorHistory[0].Error())
	})

	t.Run("get_error_pattern", func(t *testing.T) {
		tests := []struct {
			name     string
			errors   []error
			expected string
		}{
			{
				name:     "no errors",
				errors:   []error{},
				expected: "no_errors",
			},
			{
				name:     "single error",
				errors:   []error{errors.New("test error")},
				expected: "single_error",
			},
			{
				name: "repeated errors",
				errors: []error{
					errors.New("same error"),
					errors.New("same error"),
					errors.New("same error"),
				},
				expected: "repeated_error",
			},
			{
				name: "mixed errors",
				errors: []error{
					errors.New("error 1"),
					errors.New("error 2"),
					errors.New("error 3"),
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
				name: "repeated errors should escalate",
				errors: []error{
					errors.New("same error"),
					errors.New("same error"),
					errors.New("same error"),
				},
				retryCount: 2,
				maxRetries: 5,
				expected:   true,
			},
			{
				name: "mixed errors at limit should escalate",
				errors: []error{
					errors.New("error 1"),
					errors.New("error 2"),
					errors.New("error 3"),
				},
				retryCount: 5,
				maxRetries: 5,
				expected:   true,
			},
			{
				name: "mixed errors below limit should not escalate",
				errors: []error{
					errors.New("error 1"),
					errors.New("error 2"),
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

func TestErrorAction_String(t *testing.T) {
	tests := []struct {
		action   ErrorAction
		expected string
	}{
		{ErrorActionRetry, "retry"},
		{ErrorActionFail, "fail"},
		{ErrorActionPause, "pause"},
		{ErrorActionCancel, "cancel"},
		{ErrorAction(999), "unknown"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			result := test.action.String()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestNewDefaultErrorHandler(t *testing.T) {
	t.Run("with_policy", func(t *testing.T) {
		policy := &RetryPolicy{MaxRetries: 5}
		handler := NewDefaultErrorHandler(policy)

		require.NotNil(t, handler)
		assert.Equal(t, policy, handler.retryPolicy)
	})

	t.Run("with_nil_policy", func(t *testing.T) {
		handler := NewDefaultErrorHandler(nil)

		require.NotNil(t, handler)
		require.NotNil(t, handler.retryPolicy)
		assert.Equal(t, 3, handler.retryPolicy.MaxRetries) // Default policy
	})
}
