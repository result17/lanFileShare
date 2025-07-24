package webrtc

import (
	"context"
	"fmt"

	"log"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

type CommonConnection interface {
	OnDataChannel(f func(*webrtc.DataChannel))
	OnICECandidate(f func(*webrtc.ICECandidate))
	AddICECandidate(candidate webrtc.ICECandidateInit) error
	Close() error
}

type SenderConnection interface {
	CommonConnection
	Establish(ctx context.Context) error
	CreateDataChannel(label string, options *webrtc.DataChannelInit) (*webrtc.DataChannel, error)
}

type ReceiverConnection interface {
	CommonConnection
	HandleOfferAndCreateAnswer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error)
}

const (
	MTU uint = 1400
)

// Connection wraps a single WebRTC peer connection and its state.
type Connection struct {
	peerConnection *webrtc.PeerConnection
}

type SenderConn struct {
	*Connection
	signaler Signaler // Used to send signals to the remote peer
}

type ReceiverConn struct {
	*Connection
}

type WebrtcAPI struct {
	api *webrtc.API
}

// Config holds the configuration for creating a new Connection.
type Config struct {
	ICEServers []webrtc.ICEServer
}

func NewWebrtcAPI() *WebrtcAPI {

	settings := webrtc.SettingEngine{}
	settings.SetICEMulticastDNSMode(ice.MulticastDNSModeQueryAndGather)
	settings.SetReceiveMTU(MTU)

	// Using NewAPI is crucial for managing multiple PeerConnections in one application.
	api := webrtc.NewAPI(webrtc.WithSettingEngine(settings))
	return &WebrtcAPI{
		api: api,
	}
}

func (a *WebrtcAPI) createPeerconnection(config Config) (*webrtc.PeerConnection, error) {
	if len(config.ICEServers) == 0 {
		config.ICEServers = append(config.ICEServers, webrtc.ICEServer{
			URLs: []string{"stun:stun.l.google.com:19302"},
		})
	}
	return a.api.NewPeerConnection(webrtc.Configuration{
		ICEServers: config.ICEServers,
	})
}

func (a *WebrtcAPI) NewSenderConnection(config Config) (*SenderConn, error) {
	pc, err := a.createPeerconnection(config)
	if err != nil {
		log.Printf("[NewSenderConnection] %v", err)
		return nil, err
	}

	return &SenderConn{
		Connection: &Connection{
			peerConnection: pc,
		},
		// signaler is left nil initially
	}, nil
}

func (c *SenderConn) SetSignaler(signaler Signaler) {
	c.signaler = signaler
}

func (a *WebrtcAPI) NewReceiverConnection(config Config) (*ReceiverConn, error) {
	pc, err := a.createPeerconnection(config)
	if err != nil {
		log.Printf("[NewSenderConnection] %v", err)
		return nil, err
	}

	return &ReceiverConn{
		Connection: &Connection{
			peerConnection: pc,
		},
	}, nil
}

func (c *SenderConn) Establish(ctx context.Context, fileNodes []fileInfo.FileNode) error {
	if c.signaler == nil {
		err := fmt.Errorf("signaler is not set for SenderConn")
		log.Printf("[Establish] %v", err)
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
		log.Printf("[Establish] %v", err)
		return err
	}
	if err := c.peerConnection.SetLocalDescription(offer); err != nil {
		err := fmt.Errorf("fail to set local description %w", err)
		log.Printf("[Establish] %v", err)
		return err
	}

	if err := c.signaler.SendOffer(offer, fileNodes); err != nil {
		err := fmt.Errorf("fail to send offer %w", err)
		log.Printf("[Establish] %v", err)
		return err // Return error if sending offer fails
	}

	// Wait for the answer to be received from the remote peer
	answer, err := c.signaler.WaitForAnswer(ctx)
	if err != nil {
		err := fmt.Errorf("failed to wait for answer: %w", err)
		log.Printf("[Establish] %v", err)
		return err
	}

	// Set the remote description with the received answer
	if err := c.peerConnection.SetRemoteDescription(*answer); err != nil {
		err := fmt.Errorf("failed to set remote description for answer: %w", err)
		log.Printf("[Establish] %v", err)
		return err
	}

	return nil
}

// HandleOffer is called by the receiver to process an incoming offer.
func (c *ReceiverConn) HandleOfferAndCreateAnswer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	if err := c.peerConnection.SetRemoteDescription(offer); err != nil {
		err = fmt.Errorf("failed to set remote description: %w", err)
		log.Printf("[HandleOfferAndCreateAnswer] %v", err)
		return nil, err
	}

	answer, err := c.peerConnection.CreateAnswer(nil)
	if err != nil {
		err = fmt.Errorf("failed to create answer: %w", err)
		log.Printf("[HandleOfferAndCreateAnswer] %v", err)
		return nil, err
	}

	if err := c.peerConnection.SetLocalDescription(answer); err != nil {
		err = fmt.Errorf("failed to set local description for answer: %w", err)
		log.Printf("[HandleOfferAndCreateAnswer] %v", err)
		return nil, err
	}
	return &answer, nil
}

// AddICECandidate is called by both peers to add a candidate received from the other peer.
func (c *Connection) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	if err := c.peerConnection.AddICECandidate(candidate); err != nil {
		err := fmt.Errorf("failed to add ice candidate, %w", err)
		log.Printf("[AddICECandidate] %v", err)
		return err
	}
	return nil
}

func (c *Connection) OnICECandidate(f func(*webrtc.ICECandidate)) {
	c.peerConnection.OnICECandidate(f)
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
