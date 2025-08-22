package sender

import (
	appevents "github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// --- App Events (from TUI to App) ---

// SendFilesMsg is an event sent when the user selects a receiver to send files to.
type SendFilesMsg struct {
	appevents.Event
	Receiver discovery.ServiceInfo
	Files    []fileInfo.FileNode
}

var (
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
