package receiver

import (
	"github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// --- UI to App Events ---

// AcceptFileRequestEvent is sent when the user agrees to receive the files.
type AcceptFileRequestEvent struct {
	app_events.Event
}

// RejectFileRequestEvent is sent when the user rejects the file transfer.
type RejectFileRequestEvent struct {
	app_events.Event
}

// --- App to UI Messages ---

// FileNodeUpdateMsg is a message sent to the UI to update it with file info.
type FileNodeUpdateMsg struct {
	Nodes []fileInfo.FileNode
}
