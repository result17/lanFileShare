package sender

import (
	appevents "github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// --- App Events (from TUI to App) ---

// ReceiverSelectedMsg is an event sent when the user selects a receiver from the list.
type ReceiverSelectedMsg struct {
	appevents.Event
	Receiver discovery.ServiceInfo
}

// SendFilesMsg is an event sent when the user confirms which files to send.
// The receiver is already known to the app via a prior ReceiverSelectedMsg.
type SendFilesMsg struct {
	appevents.Event
	Files []fileInfo.FileNode
}

var (
	_ appevents.AppEvent = (*ReceiverSelectedMsg)(nil)
	_ appevents.AppEvent = (*SendFilesMsg)(nil)
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

type ProgressUpdateMsg struct {
	TotalFiles       int
	CompletedFiles   int
	TotalBytes       int64
	TransferredBytes int64
	CurrentFile      string
	TransferRate     float64 // bytes per second
	ETA              string  // estimated time remaining
	OverallProgress  float64 // percentage 0-100
}

type TransferCompleteMsg struct{}

// Transfer control events
type PauseTransferMsg struct {
	appevents.Event
}

type ResumeTransferMsg struct {
	appevents.Event
}

type CancelTransferMsg struct {
	appevents.Event
}

// Transfer control response events
type TransferPausedMsg struct{}
type TransferResumedMsg struct{}
type TransferCancelledMsg struct{}
