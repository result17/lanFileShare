package receiver

import (
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// ErrorMsg is a message that carries an error.
type ErrorMsg struct {
	Err error
}

// FileNodeUpdateMsg is a message sent to the UI to update it with file info.
type FileNodeUpdateMsg struct {
	Nodes []fileInfo.FileNode
}
