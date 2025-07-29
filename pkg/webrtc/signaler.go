package webrtc

import (
	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// Signaler is an interface that decouples the WebRTC logic from the signaling transport.
// The application layer must provide a concrete implementation (e.g., over HTTP, WebSocket).
type Signaler interface {
	SendOffer(offer webrtc.SessionDescription,  files []fileInfo.FileNode) error
	WaitForAnswer() (*webrtc.SessionDescription, error)
	SendICECandidate(candidate webrtc.ICECandidateInit)
}
