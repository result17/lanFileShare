package sender

import (
	"github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// --- App Events (from TUI to App) ---

// QuitAppMsg is an event sent when the user wants to quit the application.
type QuitAppMsg struct {
	app_events.Event
}

// SendFilesMsg is an event sent when the user selects a receiver to send files to.
type SendFilesMsg struct {
	app_events.Event
	Receiver discovery.ServiceInfo
	Files    []fileInfo.FileNode
}

var (
	// These static checks ensure that our event types correctly implement the AppEvent interface.
	// The code will not compile if they don't.
	_ app_events.AppEvent = (*SendFilesMsg)(nil)
)

// --- UI Messages (from App to TUI) ---

type FoundServicesMsg struct {
	Services []discovery.ServiceInfo
}

type StatusUpdateMsg struct {
	Message string
}

type TransferStartedMsg struct{}

type ReceiverAcceptedMsg struct{}

type TransferCompleteMsg struct{}

type ErrorMsg struct {
	Err error
}
