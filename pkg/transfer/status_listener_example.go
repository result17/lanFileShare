package transfer

import (
	"fmt"
	"log"

	"github.com/google/uuid"
)

// ExampleStatusListener demonstrates how to implement the StatusListener interface
// with a unique UUID identifier
type ExampleStatusListener struct {
	id   string
	name string
}

// NewExampleStatusListener creates a new status listener with a unique UUID
func NewExampleStatusListener(name string) *ExampleStatusListener {
	return &ExampleStatusListener{
		id:   uuid.New().String(),
		name: name,
	}
}

// ID returns the unique identifier for this listener
func (esl *ExampleStatusListener) ID() string {
	return esl.id
}

// OnFileStatusChanged handles file status change events
func (esl *ExampleStatusListener) OnFileStatusChanged(filePath string, oldStatus, newStatus *TransferStatus) {
	var oldState, newState string
	
	if oldStatus != nil {
		oldState = oldStatus.State.String()
	} else {
		oldState = "nil"
	}
	
	if newStatus != nil {
		newState = newStatus.State.String()
	} else {
		newState = "nil"
	}
	
	log.Printf("[%s] File %s: %s -> %s", esl.name, filePath, oldState, newState)
	
	// Example: Handle specific state transitions
	if newStatus != nil {
		switch newStatus.State {
		case TransferStateActive:
			fmt.Printf("📤 Started transferring: %s\n", filePath)
		case TransferStateCompleted:
			fmt.Printf("✅ Completed transfer: %s (%.2f%% done)\n", 
				filePath, newStatus.GetProgressPercentage())
		case TransferStateFailed:
			fmt.Printf("❌ Failed transfer: %s", filePath)
			if newStatus.LastError != nil {
				fmt.Printf(" - Error: %v", newStatus.LastError)
			}
			fmt.Println()
		case TransferStatePaused:
			fmt.Printf("⏸️ Paused transfer: %s (%.2f%% done)\n", 
				filePath, newStatus.GetProgressPercentage())
		}
	}
}

// OnSessionStatusChanged handles session status change events
func (esl *ExampleStatusListener) OnSessionStatusChanged(oldStatus, newStatus *SessionTransferStatus) {
	var oldState, newState string
	
	if oldStatus != nil {
		oldState = oldStatus.State.String()
	} else {
		oldState = "nil"
	}
	
	if newStatus != nil {
		newState = newStatus.State.String()
	} else {
		newState = "nil"
	}
	
	log.Printf("[%s] Session: %s -> %s", esl.name, oldState, newState)
	
	// Example: Handle session progress updates
	if newStatus != nil {
		switch newStatus.State {
		case StatusSessionStateActive:
			fmt.Printf("🚀 Session active: %d/%d files, %.1f%% complete\n",
				newStatus.CompletedFiles, newStatus.TotalFiles, newStatus.OverallProgress)
		case StatusSessionStateCompleted:
			fmt.Printf("🎉 Session completed: %d files transferred successfully\n", 
				newStatus.CompletedFiles)
		case StatusSessionStateFailed:
			fmt.Printf("💥 Session failed: %d completed, %d failed\n",
				newStatus.CompletedFiles, newStatus.FailedFiles)
		}
	}
}

// ExampleUsage demonstrates how to use the StatusListener with UUID
func ExampleUsage() {
	// Create a transfer manager
	manager := NewUnifiedTransferManager("example-service")
	defer manager.Close()
	
	// Create multiple listeners with unique IDs
	progressListener := NewExampleStatusListener("ProgressTracker")
	logListener := NewExampleStatusListener("Logger")
	metricsListener := NewExampleStatusListener("MetricsCollector")
	
	// Add listeners to the manager
	manager.AddStatusListener(progressListener)
	manager.AddStatusListener(logListener)
	manager.AddStatusListener(metricsListener)
	
	// Each listener now has a unique UUID that can be used for:
	// - Identifying listeners in logs
	// - Removing specific listeners (if removal functionality is added)
	// - Tracking listener performance
	// - Debugging listener behavior
	
	fmt.Printf("Progress Listener ID: %s\n", progressListener.ID())
	fmt.Printf("Log Listener ID: %s\n", logListener.ID())
	fmt.Printf("Metrics Listener ID: %s\n", metricsListener.ID())
	
	// Now when you perform transfer operations, all listeners will receive
	// notifications with their unique IDs for identification
}

// LoggingStatusListener is a simple listener that just logs events with its UUID
type LoggingStatusListener struct {
	id string
}

// NewLoggingStatusListener creates a new logging listener
func NewLoggingStatusListener() *LoggingStatusListener {
	return &LoggingStatusListener{
		id: uuid.New().String(),
	}
}

// ID returns the unique identifier
func (lsl *LoggingStatusListener) ID() string {
	return lsl.id
}

// OnFileStatusChanged logs file status changes
func (lsl *LoggingStatusListener) OnFileStatusChanged(filePath string, oldStatus, newStatus *TransferStatus) {
	log.Printf("[Listener %s] File status changed: %s", lsl.id[:8], filePath)
}

// OnSessionStatusChanged logs session status changes
func (lsl *LoggingStatusListener) OnSessionStatusChanged(oldStatus, newStatus *SessionTransferStatus) {
	log.Printf("[Listener %s] Session status changed", lsl.id[:8])
}
