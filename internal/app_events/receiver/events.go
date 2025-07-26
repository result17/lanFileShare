package receiver

import (
	"github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// --- UI to App Events ---

// AcceptFileRequestEvent is sent when the user agrees to receive the files.
type AcceptFileRequestEvent struct {
	appevents.Event
}

// RejectFileRequestEvent is sent when the user rejects the file transfer.
type RejectFileRequestEvent struct {
	appevents.Event
}

// --- App to UI Messages ---

// FileNodeUpdateMsg is a message sent to the UI to update it with file info.
type FileNodeUpdateMsg struct {
	appevents.AppUIMessage
	Nodes []fileInfo.FileNode
}

type TransferCompleteMsg struct {
	appevents.AppUIMessage
}
