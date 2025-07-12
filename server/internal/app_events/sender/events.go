package sender

import (
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// --- App Events (from TUI to App) ---

// AppEvent defines an event sent from the TUI to the App's logic controller.
type AppEvent interface {
	isAppEvent()
}

// QuitAppMsg is an event sent when the user wants to quit the application.
type QuitAppMsg struct{}

func (q QuitAppMsg) isAppEvent() {}

// SendFilesMsg is an event sent when the user selects a receiver to send files to.
type SendFilesMsg struct {
	Receiver discovery.ServiceInfo
	Files    []fileInfo.FileNode
}

func (s SendFilesMsg) isAppEvent() {}

// --- UI Messages (from App to TUI) ---

type FoundServicesMsg struct {
	Services []discovery.ServiceInfo
}

type StatusUpdateMsg struct {
	Message string
}

type TransferStartedMsg struct{}

type TransferCompleteMsg struct{}

type ErrorMsg struct {
	Err error
}
