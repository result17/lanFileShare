package webrtc

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pion/webrtc/v4"
)

// State defines the various states of the WebRTC connection.
type State int

const (
	StateIdle State = iota
	StateConnecting
	StateConnected
	StateDisconnected
	StateFailed
)

// String provides a human-readable representation of the connection state.
func (s State) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateDisconnected:
		return "Disconnected"
	case StateFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// Signal is a generic struct for signaling messages exchanged between peers.
type Signal struct {
	Type    string `json:"type"` // "offer", "answer", or "candidate"
	Payload string `json:"payload"` // The SDP or ICE Candidate JSON
}

// Signaler is an interface that decouples the WebRTC logic from the signaling transport.
// The application layer must provide a concrete implementation (e.g., over HTTP, WebSocket).
type Signaler interface {
	Send(signal Signal) error
}

// Connection wraps a single WebRTC peer connection and its state.
type Connection struct {
	peerConnection *webrtc.PeerConnection
	dataChannel    *webrtc.DataChannel
	signaler       Signaler // Used to send signals to the remote peer

	state      State
	stateMutex sync.RWMutex // Protects concurrent access to the state field

	// Callbacks for the application layer to respond to events.
	OnStateChange        func(State)
	OnDataChannelOpen    func()
	OnDataChannelMessage func(msg webrtc.DataChannelMessage)
}

// Config holds the configuration for creating a new Connection.
type Config struct {
	ICEServers []webrtc.ICEServer
	Signaler   Signaler // A Signaler implementation is required.
}

// NewConnection creates and initializes a new WebRTC connection.
func NewConnection(config Config) (*Connection, error) {
	// Use a public STUN server if no ICE servers are provided.
	if len(config.ICEServers) == 0 {
		config.ICEServers = []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		}
	}

	// Using NewAPI is crucial for managing multiple PeerConnections in one application.
	api := webrtc.NewAPI()
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: config.ICEServers,
	})
	if err != nil {
		return nil, err
	}

	conn := &Connection{
		peerConnection: pc,
		signaler:       config.Signaler,
		state:          StateIdle,
	}

	// Register core event handlers.
	conn.registerEventHandlers()

	return conn, nil
}

// registerEventHandlers sets up the callbacks for the underlying peer connection.
func (c *Connection) registerEventHandlers() {
	// Handle ICE connection state changes to update the overall connection state.
	c.peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		var s State
		switch state {
		case webrtc.ICEConnectionStateChecking, webrtc.ICEConnectionStateNew:
			s = StateConnecting
		case webrtc.ICEConnectionStateConnected, webrtc.ICEConnectionStateCompleted:
			s = StateConnected
		case webrtc.ICEConnectionStateDisconnected:
			s = StateDisconnected
		case webrtc.ICEConnectionStateFailed:
			s = StateFailed
		default:
			s = StateIdle
		}
		c.setState(s)
	})

	// Handle newly gathered ICE candidates.
	c.peerConnection.OnICECandidate(func(cand *webrtc.ICECandidate) {
		if cand == nil {
			return
		}
		candidateJSON, err := json.Marshal(cand.ToJSON())
		if err != nil {
			// In a real app, log this error.
			return
		}
		signal := Signal{
			Type:    "candidate",
			Payload: string(candidateJSON),
		}
		if c.signaler != nil {
			// Send the candidate to the remote peer via the signaling channel.
			c.signaler.Send(signal)
		}
	})

	// Handle the event when the remote peer opens a data channel.
	c.peerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		c.dataChannel = dc
		c.setupDataChannelCallbacks()
	})
}

// setState is a thread-safe method to update the connection state and trigger the callback.
func (c *Connection) setState(s State) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()

	if c.state == s {
		return // No change in state.
	}
	c.state = s

	if c.OnStateChange != nil {
		// Run callback in a new goroutine to avoid blocking.
		go c.OnStateChange(s)
	}
}

// setupDataChannelCallbacks configures the OnOpen and OnMessage handlers for the data channel.
func (c *Connection) setupDataChannelCallbacks() {
	if c.dataChannel == nil {
		return
	}
	c.dataChannel.OnOpen(func() {
		if c.OnDataChannelOpen != nil {
			go c.OnDataChannelOpen()
		}
	})
	c.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		if c.OnDataChannelMessage != nil {
			c.OnDataChannelMessage(msg)
		}
	})
}

// CreateOffer is called by the initiator (Sender) to start the WebRTC handshake.
func (c *Connection) CreateOffer() error {
	// The initiator creates the data channel.
	dc, err := c.peerConnection.CreateDataChannel("lan-file-sharer-channel", nil)
	if err != nil {
		return fmt.Errorf("failed to create data channel: %w", err)
	}
	c.dataChannel = dc
	c.setupDataChannelCallbacks()

	offer, err := c.peerConnection.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	if err := c.peerConnection.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("failed to set local description: %w", err)
	}

	// Send the offer to the remote peer.
	return c.signaler.Send(Signal{Type: "offer", Payload: offer.SDP})
}

// HandleOffer is called by the receiver to process an incoming offer.
func (c *Connection) HandleOffer(offerSDP string) error {
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
	}

	if err := c.peerConnection.SetRemoteDescription(offer); err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	answer, err := c.peerConnection.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("failed to create answer: %w", err)
	}

	if err := c.peerConnection.SetLocalDescription(answer); err != nil {
		return fmt.Errorf("failed to set local description for answer: %w", err)
	}

	// Send the answer back to the initiator.
	return c.signaler.Send(Signal{Type: "answer", Payload: answer.SDP})
}

// HandleAnswer is called by the initiator (Sender) to process the receiver's answer.
func (c *Connection) HandleAnswer(answerSDP string) error {
	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answerSDP,
	}
	return c.peerConnection.SetRemoteDescription(answer)
}

// AddICECandidate is called by both peers to add a candidate received from the other peer.
func (c *Connection) AddICECandidate(candidateJSON string) error {
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(candidateJSON), &candidate); err != nil {
		return fmt.Errorf("failed to unmarshal ICE candidate: %w", err)
	}
	return c.peerConnection.AddICECandidate(candidate)
}

// SendData sends byte data over the established data channel.
func (c *Connection) SendData(data []byte) error {
	if c.dataChannel == nil {
		return fmt.Errorf("data channel is not initialized")
	}
	return c.dataChannel.Send(data)
}

// Close gracefully shuts down the WebRTC connection.
func (c *Connection) Close() error {
	if c.peerConnection != nil {
		return c.peerConnection.Close()
	}
	return nil
}