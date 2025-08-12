package receiver

import (
	appevents "github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// --- UI to App Events ---

// FileRequestAccepted is sent when the user agrees to receive the files.
type FileRequestAccepted struct {
	appevents.Event
}

// FileRequestRejected is sent when the user rejects the file transfer.
type FileRequestRejected struct {
	appevents.Event
}

// --- App to UI Messages ---

// FileNodeUpdateMsg is a message sent to the UI to update it with file info.
type FileNodeUpdateMsg struct {
	appevents.AppUIMessage
	Nodes []fileInfo.FileNode
}

// TransferFinishedMsg signals the end of a file transfer, with status.
type TransferFinishedMsg struct {
	appevents.AppUIMessage
	Err error // nil if transfer was successful
}

// StatusUpdateMsg provides status updates during file transfer
type StatusUpdateMsg struct {
	appevents.AppUIMessage
	Message string
}
