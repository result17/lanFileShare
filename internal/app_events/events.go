package app_events

// AppEvent is a marker interface for events sent from the TUI to the App's logic controller.
// It uses an unexported method to ensure that only types from this package (by embedding Event)
// can satisfy the interface, providing compile-time safety.
type AppEvent interface {
	isAppEvent()
}

// Event is a struct that can be embedded in other event types to satisfy the AppEvent interface.
type Event struct{}

// isAppEvent is the marker method that makes a struct an AppEvent.
func (Event) isAppEvent() {}

// --- App Events (from TUI to App) ---

// QuitAppMsg is an event sent when the user wants to quit the application.
type QuitAppMsg struct{}

func (q QuitAppMsg) isAppEvent() {}

type ErrorMsg struct {
	Err error
}
