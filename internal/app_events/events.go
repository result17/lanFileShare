package appevents

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

// AppUIMessage is a marker interface for messages sent from the App's logic controller to the TUI.
 type AppUIMessage interface {
	isUIMessage()
 }
// UIMessage is a base struct that can be embedded in other types to implement the AppUIMessage interface.
type UIMessage struct{}

func (UIMessage) isUIMessage() {}

// --- App Events (from TUI to App) ---

// For events from the TUI to the App
type UIErrorEvent struct {
	Event
	Err error
}

// For messages from the App to the TUI
type AppErrorMsg struct {
	UIMessage
	Err error
}
