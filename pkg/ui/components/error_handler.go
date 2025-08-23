package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/rescp17/lanFileSharer/internal/style"
)

// ErrorType represents different types of errors
type ErrorType int

const (
	ErrorTypeNetwork ErrorType = iota
	ErrorTypeFileSystem
	ErrorTypePermission
	ErrorTypeTimeout
	ErrorTypeUserCancelled
	ErrorTypeUnknown
)

// ErrorInfo contains detailed information about an error
type ErrorInfo struct {
	Type        ErrorType
	Message     string
	Details     string
	Timestamp   time.Time
	Recoverable bool
	RetryCount  int
	MaxRetries  int
	Suggestions []string
}

// ErrorHandler manages error display and recovery suggestions
type ErrorHandler struct {
	errors      []ErrorInfo
	maxErrors   int
	autoRetry   bool
	retryDelay  time.Duration
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(maxErrors int, autoRetry bool, retryDelay time.Duration) *ErrorHandler {
	return &ErrorHandler{
		errors:     make([]ErrorInfo, 0),
		maxErrors:  maxErrors,
		autoRetry:  autoRetry,
		retryDelay: retryDelay,
	}
}

// AddError adds a new error to the handler
func (eh *ErrorHandler) AddError(errorType ErrorType, message, details string, recoverable bool) {
	errorInfo := ErrorInfo{
		Type:        errorType,
		Message:     message,
		Details:     details,
		Timestamp:   time.Now(),
		Recoverable: recoverable,
		RetryCount:  0,
		MaxRetries:  eh.getMaxRetriesForType(errorType),
		Suggestions: eh.getSuggestionsForType(errorType),
	}

	eh.errors = append(eh.errors, errorInfo)

	// Keep only the most recent errors
	if len(eh.errors) > eh.maxErrors {
		eh.errors = eh.errors[len(eh.errors)-eh.maxErrors:]
	}
}

// IncrementRetry increments the retry count for the latest error
func (eh *ErrorHandler) IncrementRetry() bool {
	if len(eh.errors) == 0 {
		return false
	}

	latest := &eh.errors[len(eh.errors)-1]
	if latest.RetryCount < latest.MaxRetries {
		latest.RetryCount++
		return true
	}
	return false
}

// GetLatestError returns the most recent error
func (eh *ErrorHandler) GetLatestError() *ErrorInfo {
	if len(eh.errors) == 0 {
		return nil
	}
	return &eh.errors[len(eh.errors)-1]
}

// CanRetry checks if the latest error can be retried
func (eh *ErrorHandler) CanRetry() bool {
	latest := eh.GetLatestError()
	if latest == nil {
		return false
	}
	return latest.Recoverable && latest.RetryCount < latest.MaxRetries
}

// ShouldAutoRetry checks if auto-retry should be attempted
func (eh *ErrorHandler) ShouldAutoRetry() bool {
	return eh.autoRetry && eh.CanRetry()
}

// Clear clears all errors
func (eh *ErrorHandler) Clear() {
	eh.errors = eh.errors[:0]
}

// Render renders the error display
func (eh *ErrorHandler) Render() string {
	if len(eh.errors) == 0 {
		return ""
	}

	var result strings.Builder

	// Show the latest error prominently
	latest := eh.errors[len(eh.errors)-1]
	result.WriteString(eh.renderError(latest, true))

	// Show previous errors in compact form if there are multiple
	if len(eh.errors) > 1 {
		result.WriteString("\n\n")
		result.WriteString(style.FileStyle.Render("Previous errors:"))
		result.WriteString("\n")

		for i := len(eh.errors) - 2; i >= 0 && i >= len(eh.errors)-4; i-- {
			result.WriteString(eh.renderError(eh.errors[i], false))
			result.WriteString("\n")
		}
	}

	return result.String()
}

// renderError renders a single error
func (eh *ErrorHandler) renderError(err ErrorInfo, detailed bool) string {
	var result strings.Builder

	// Error header with icon and type
	icon := eh.getErrorIcon(err.Type)
	typeStr := eh.getErrorTypeString(err.Type)
	headerStyle := eh.getErrorStyle(err.Type)

	if detailed {
		result.WriteString(fmt.Sprintf("%s %s: %s\n",
			icon,
			headerStyle.Render(typeStr),
			headerStyle.Render(err.Message)))

		// Add details if available
		if err.Details != "" {
			result.WriteString(fmt.Sprintf("   %s\n", style.FileStyle.Render(err.Details)))
		}

		// Add timestamp
		timeStr := err.Timestamp.Format("15:04:05")
		result.WriteString(fmt.Sprintf("   %s\n", style.FileStyle.Render(fmt.Sprintf("Time: %s", timeStr))))

		// Add retry information
		if err.Recoverable {
			if err.RetryCount > 0 {
				result.WriteString(fmt.Sprintf("   %s\n",
					style.FileStyle.Render(fmt.Sprintf("Retries: %d/%d", err.RetryCount, err.MaxRetries))))
			}

			if err.RetryCount < err.MaxRetries {
				result.WriteString(fmt.Sprintf("   %s\n",
					lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("This error can be retried")))
			} else {
				result.WriteString(fmt.Sprintf("   %s\n",
					style.ErrorStyle.Render("Maximum retries exceeded")))
			}
		} else {
			result.WriteString(fmt.Sprintf("   %s\n",
				style.ErrorStyle.Render("This error cannot be automatically recovered")))
		}

		// Add suggestions
		if len(err.Suggestions) > 0 {
			result.WriteString("\n   💡 Suggestions:\n")
			for _, suggestion := range err.Suggestions {
				result.WriteString(fmt.Sprintf("   • %s\n",
					lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(suggestion)))
			}
		}
	} else {
		// Compact format
		result.WriteString(fmt.Sprintf("%s %s: %s (%s)",
			icon,
			typeStr,
			err.Message,
			err.Timestamp.Format("15:04")))
	}

	return result.String()
}

// getErrorIcon returns the appropriate icon for the error type
func (eh *ErrorHandler) getErrorIcon(errorType ErrorType) string {
	switch errorType {
	case ErrorTypeNetwork:
		return "🌐"
	case ErrorTypeFileSystem:
		return "📁"
	case ErrorTypePermission:
		return "🔒"
	case ErrorTypeTimeout:
		return "⏰"
	case ErrorTypeUserCancelled:
		return "🚫"
	default:
		return "❌"
	}
}

// getErrorTypeString returns a human-readable string for the error type
func (eh *ErrorHandler) getErrorTypeString(errorType ErrorType) string {
	switch errorType {
	case ErrorTypeNetwork:
		return "Network Error"
	case ErrorTypeFileSystem:
		return "File System Error"
	case ErrorTypePermission:
		return "Permission Error"
	case ErrorTypeTimeout:
		return "Timeout Error"
	case ErrorTypeUserCancelled:
		return "User Cancelled"
	default:
		return "Unknown Error"
	}
}

// getErrorStyle returns the appropriate style for the error type
func (eh *ErrorHandler) getErrorStyle(errorType ErrorType) lipgloss.Style {
	switch errorType {
	case ErrorTypeNetwork:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // Blue
	case ErrorTypeFileSystem:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	case ErrorTypePermission:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	case ErrorTypeTimeout:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow
	case ErrorTypeUserCancelled:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("244")) // Gray
	default:
		return style.ErrorStyle
	}
}

// getMaxRetriesForType returns the maximum number of retries for an error type
func (eh *ErrorHandler) getMaxRetriesForType(errorType ErrorType) int {
	switch errorType {
	case ErrorTypeNetwork:
		return 3
	case ErrorTypeFileSystem:
		return 2
	case ErrorTypePermission:
		return 1
	case ErrorTypeTimeout:
		return 2
	case ErrorTypeUserCancelled:
		return 0
	default:
		return 1
	}
}

// getSuggestionsForType returns recovery suggestions for an error type
func (eh *ErrorHandler) getSuggestionsForType(errorType ErrorType) []string {
	switch errorType {
	case ErrorTypeNetwork:
		return []string{
			"Check your network connection",
			"Verify both devices are on the same network",
			"Try restarting the application",
			"Check firewall settings",
		}
	case ErrorTypeFileSystem:
		return []string{
			"Check if the file still exists",
			"Verify you have read permissions",
			"Ensure sufficient disk space",
			"Try closing other applications using the file",
		}
	case ErrorTypePermission:
		return []string{
			"Run the application as administrator",
			"Check file and folder permissions",
			"Verify the destination folder is writable",
		}
	case ErrorTypeTimeout:
		return []string{
			"Check network stability",
			"Try with smaller files first",
			"Increase timeout settings if possible",
			"Move closer to the receiver",
		}
	case ErrorTypeUserCancelled:
		return []string{
			"Press Enter to try again",
			"Select different files if needed",
		}
	default:
		return []string{
			"Try restarting the application",
			"Check the application logs for more details",
		}
	}
}

// RetryDialog represents a retry confirmation dialog
type RetryDialog struct {
	errorHandler *ErrorHandler
	visible      bool
	countdown    int
	autoRetry    bool
}

// NewRetryDialog creates a new retry dialog
func NewRetryDialog(errorHandler *ErrorHandler) *RetryDialog {
	return &RetryDialog{
		errorHandler: errorHandler,
		visible:      false,
		countdown:    5,
		autoRetry:    false,
	}
}

// Show shows the retry dialog
func (rd *RetryDialog) Show(autoRetry bool, countdown int) {
	rd.visible = true
	rd.autoRetry = autoRetry
	rd.countdown = countdown
}

// Hide hides the retry dialog
func (rd *RetryDialog) Hide() {
	rd.visible = false
}

// IsVisible returns whether the dialog is visible
func (rd *RetryDialog) IsVisible() bool {
	return rd.visible
}

// UpdateCountdown decrements the countdown
func (rd *RetryDialog) UpdateCountdown() bool {
	if rd.countdown > 0 {
		rd.countdown--
		return true
	}
	return false
}

// ShouldAutoRetry returns whether auto-retry should proceed
func (rd *RetryDialog) ShouldAutoRetry() bool {
	return rd.autoRetry && rd.countdown <= 0
}

// Render renders the retry dialog
func (rd *RetryDialog) Render() string {
	if !rd.visible {
		return ""
	}

	var result strings.Builder

	result.WriteString("┌─────────────────────────────────────────────────────────────────────────────────┐\n")
	result.WriteString("│ 🔄 Retry Transfer?\n")
	result.WriteString("├─────────────────────────────────────────────────────────────────────────────────┤\n")

	latest := rd.errorHandler.GetLatestError()
	if latest != nil {
		result.WriteString(fmt.Sprintf("│ Last error: %s\n", latest.Message))
		result.WriteString(fmt.Sprintf("│ Retry count: %d/%d\n", latest.RetryCount, latest.MaxRetries))
	}

	result.WriteString("├─────────────────────────────────────────────────────────────────────────────────┤\n")

	if rd.autoRetry {
		if rd.countdown > 0 {
			result.WriteString(fmt.Sprintf("│ Auto-retry in %d seconds... (Press any key to cancel)\n", rd.countdown))
		} else {
			result.WriteString("│ Retrying now...\n")
		}
	} else {
		result.WriteString("│ Press 'R' to retry, 'C' to cancel, or 'Q' to quit\n")
	}

	result.WriteString("└─────────────────────────────────────────────────────────────────────────────────┘")

	return result.String()
}
