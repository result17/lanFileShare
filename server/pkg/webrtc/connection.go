package webrtc

import (
	"context"
	"fmt"
	"log"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
)

// State defines the various states of the WebRTC connection.
type State int

const (
	MTU uint = 1400
)

// Connection wraps a single WebRTC peer connection and its state.
type Connection struct {
	peerConnection *webrtc.PeerConnection
	signaler       Signaler // Used to send signals to the remote peer
}

type WebRTCAPI struct {
	api *webrtc.API
}

// Config holds the configuration for creating a new Connection.
type Config struct {
	ICEServers []webrtc.ICEServer
	Signaler   Signaler // A Signaler implementation is required.
}

func NewWebRTCAPI() *WebRTCAPI {

	settings := webrtc.SettingEngine{}
	settings.SetICEMulticastDNSMode(ice.MulticastDNSModeQueryAndGather)
	settings.SetReceiveMTU(MTU)

	// Using NewAPI is crucial for managing multiple PeerConnections in one application.
	api := webrtc.NewAPI(webrtc.WithSettingEngine(settings))
	return &WebRTCAPI{
		api: api,
	}
}

// NewConnection creates and initializes a new WebRTC connection.
func (w *WebRTCAPI) NewConnection(config Config) (*Connection, error) {
	// Use a public STUN server if no ICE servers are provided.
	if len(config.ICEServers) == 0 {
		config.ICEServers = []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		}
	}
	pc, err := w.api.NewPeerConnection(webrtc.Configuration{
		ICEServers: config.ICEServers,
	})
	if err != nil {
		return nil, err
	}

	conn := &Connection{
		peerConnection: pc,
		signaler:       config.Signaler,
	}

	return conn, nil
}

func (c *Connection) Establish(ctx context.Context) error {
	if c.signaler == nil {
		err := fmt.Errorf("signaler is not configured")
		log.Printf("[Establish]: %w", err)
		return err
	}
	c.peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			c.signaler.SendICECandidate(candidate.ToJSON())
		}
	})
	offer, err := c.peerConnection.CreateOffer(nil)
	if err != nil {
		err := fmt.Errorf("fail to createOffer %w", err)
		log.Printf("[Establish] %w", err)
		return err
	}
	if err := c.peerConnection.SetLocalDescription(offer); err != nil {
		err := fmt.Errorf("fail to set local description %w", err)
		log.Printf("[Establish] %w", err)
		return err
	}

	if err := c.signaler.SendOffer(offer); err != nil {
		err := fmt.Errorf("fail to send offer %w", err)
		log.Printf("[Establish] %w", err)
	}
	return nil
}

// HandleOffer is called by the receiver to process an incoming offer.
func (c *Connection) HandleOfferAndCreateAnswer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	if err := c.peerConnection.SetRemoteDescription(offer); err != nil {
		err = fmt.Errorf("failed to set remote description: %w", err)
		log.Printf("[HandleOfferAndCreateAnswer] %w", err)
		return nil, err
	}

	answer, err := c.peerConnection.CreateAnswer(nil)
	if err != nil {
		err = fmt.Errorf("failed to create answer: %w", err)
		log.Printf("[HandleOfferAndCreateAnswer] %w", err)
		return nil, err
	}

	if err := c.peerConnection.SetLocalDescription(answer); err != nil {
		err = fmt.Errorf("failed to set local description for answer: %w", err)
		log.Printf("[HandleOfferAndCreateAnswer] %w", err)
		return nil, err
	}
	return &answer, nil
}

// AddICECandidate is called by both peers to add a candidate received from the other peer.
func (c *Connection) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	if err := c.peerConnection.AddICECandidate(candidate); err != nil {
		err := fmt.Errorf("failed to ice candidate")
		log.Printf("[AddICECandidate] %w", err)
		return err
	}
	return nil
}

func (c *Connection) OnDataChannel(f func(*webrtc.DataChannel)) {
	c.peerConnection.OnDataChannel(f)
}

func (c *Connection) CreateDataChannel(label string, options *webrtc.DataChannelInit) (*webrtc.DataChannel, error) {
	return c.peerConnection.CreateDataChannel(label, options)
}

// Close gracefully shuts down the WebRTC connection.
func (c *Connection) Close() error {
	if c.peerConnection != nil {
		log.Printf("Closing webrtc connection")
		return c.peerConnection.Close()
	}
	return nil
}
