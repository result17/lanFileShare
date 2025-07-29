package webrtc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/api"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

type CommonConnection interface {
	Peer() *webrtc.PeerConnection
	Close() error
}

type SenderConnection interface {
	CommonConnection
	Establish(ctx context.Context, fileNodes []fileInfo.FileNode) error
	CreateDataChannel(label string, options *webrtc.DataChannelInit) (*webrtc.DataChannel, error)
	SendFiles(ctx context.Context, files []fileInfo.FileNode) error 
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

// Peer returns the underlying webrtc.PeerConnection object.
func (c *Connection) Peer() *webrtc.PeerConnection {
	return c.peerConnection
}

func (c *Connection) Close() error {
	if c.peerConnection != nil {
		slog.Info("Closing WebRTC connection")
		return c.peerConnection.Close()
	}
	return nil
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

	api := webrtc.NewAPI(webrtc.WithSettingEngine(settings))
	return &WebrtcAPI{
		api: api,
	}
}

func (a *WebrtcAPI) createPeerConnection(config Config) (*webrtc.PeerConnection, error) {
	peerConnectionConfig := webrtc.Configuration{
		ICEServers: config.ICEServers,
	}
	if len(config.ICEServers) == 0 {
		peerConnectionConfig.ICEServers = []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		}
	}
	pc, err := a.api.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		// Just wrap and return. Let the caller log.
		return nil, fmt.Errorf("failed to create new peer connection: %w", err)
	}
	return pc, nil
}

func (a *WebrtcAPI) NewSenderConnection(ctx context.Context, config Config, apiClient *api.Client) (SenderConnection, error) {
	pc, err := a.createPeerConnection(config)
	if err != nil {
		return nil, err
	}
	conn := &SenderConn{
		Connection: &Connection{
			peerConnection: pc,
		},
	}

	signaler := api.NewAPISignaler(ctx, apiClient, conn.Peer().AddICECandidate)
	conn.signaler = signaler

	return conn, nil

}

func (a *WebrtcAPI) NewReceiverConnection(config Config) (*ReceiverConn, error) {
	pc, err := a.createPeerConnection(config)
	if err != nil {
		slog.Error("Failed to create peer connection for receiver", "error", err)
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
		err := errors.New("signaler is not set for SenderConn")
		slog.Error("Cannot establish connection", "error", err)
		return err
	}

	c.Peer().OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			c.signaler.SendICECandidate(candidate.ToJSON())
		}
	})

	offer, err := c.Peer().CreateOffer(nil)
	if err != nil {
		slog.Error("Failed to create offer", "error", err)
		return fmt.Errorf("failed to create offer: %w", err)
	}

	if err := c.Peer().SetLocalDescription(offer); err != nil {
		slog.Error("Failed to set local description", "error", err)
		return fmt.Errorf("failed to set local description: %w", err)
	}

	if err := c.signaler.SendOffer(offer, fileNodes); err != nil {
		slog.Error("Failed to send offer via signaler", "error", err)
		return fmt.Errorf("failed to send offer via signaler: %w", err)
	}

	answer, err := c.signaler.WaitForAnswer()
	if err != nil {
		slog.Error("Failed to wait for answer", "error", err)
		return fmt.Errorf("failed to wait for answer: %w", err)
	}

	if err := c.Peer().SetRemoteDescription(*answer); err != nil {
		slog.Error("Failed to set remote description for answer", "error", err)
		return fmt.Errorf("failed to set remote description for answer: %w", err)
	}

	return nil
}

func (c *ReceiverConn) HandleOfferAndCreateAnswer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	if err := c.Peer().SetRemoteDescription(offer); err != nil {
		slog.Error("Failed to set remote description", "error", err)
		return nil, err
	}

	answer, err := c.Peer().CreateAnswer(nil)
	if err != nil {
		slog.Error("Failed to create answer", "error", err)
		return nil, err
	}

	if err := c.Peer().SetLocalDescription(answer); err != nil {
		slog.Error("Failed to set local description for answer", "error", err)
		return nil, err
	}
	return &answer, nil
}

func (c *SenderConn) CreateDataChannel(label string, options *webrtc.DataChannelInit) (*webrtc.DataChannel, error) {
	return c.peerConnection.CreateDataChannel(label, options)
}

func (c *SenderConn) SendFiles(ctx context.Context, files []fileInfo.FileNode) error {
	// TODO
	return nil
}
