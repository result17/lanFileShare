package ui

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/internal/app_events"
)

// AppController defines the contract between the UI and the backend application logic.
// Both Sender and Receiver apps should implement this interface.
type AppController interface {
	// Run starts the backend services and the event loop.
	Run(ctx context.Context, cancel context.CancelFunc)

	// UIMessages returns a read-only channel for receiving messages from the backend to the UI.
	UIMessages() <-chan tea.Msg

	// AppEvents returns a write-only channel for the UI to send events to the backend.
	AppEvents() chan<- app_events.AppEvent
}
