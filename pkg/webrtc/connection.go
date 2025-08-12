package webrtc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/api"
	"github.com/rescp17/lanFileSharer/pkg/crypto"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/rescp17/lanFileSharer/pkg/transfer"
)

type CommonConnection interface {
	Peer() *webrtc.PeerConnection
	Close() error
}

type SenderConnection interface {
	CommonConnection
	Establish(ctx context.Context, fileNodes []fileInfo.FileNode) error
	CreateDataChannel(label string, options *webrtc.DataChannelInit) (*webrtc.DataChannel, error)
	SendFiles(ctx context.Context, files []fileInfo.FileNode, serviceID string) error
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
	serializer transfer.MessageSerializer
}

// SetSignaler allows setting a custom signaler (mainly for testing)
func (s *SenderConn) SetSignaler(signaler Signaler) {
	s.signaler = signaler
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

func (c *Connection) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	return c.peerConnection.AddICECandidate(candidate)
}

func (a *WebrtcAPI) NewSenderConnection(transferCtx context.Context, config Config, apiClient *api.Client, receiverURL string) (SenderConnection, error) {
	pc, err := a.createPeerConnection(config)
	if err != nil {
		return nil, err
	}
	conn := &SenderConn{
		Connection: &Connection{
			peerConnection: pc,
		},
		serializer: transfer.NewJSONSerializer(),
	}

	signaler := api.NewAPISignaler(apiClient, receiverURL, conn.AddICECandidate)
	conn.signaler = signaler

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			err := signaler.SendICECandidate(transferCtx, candidate.ToJSON())
			if err != nil {
				slog.Error("Failed to send ICE candidate", "error", err)
			}
		}
	})

	return conn, nil
}

func (a *WebrtcAPI) NewReceiverConnection(config Config) (ReceiverConnection, error) {
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
		return err
	}

	offer, err := c.Peer().CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	if err := c.Peer().SetLocalDescription(offer); err != nil {
		return fmt.Errorf("failed to set local description: %w", err)
	}

	fileStructureSigner, err := crypto.NewFileStructureSigner()
	if err != nil {
		return fmt.Errorf("failed to create file structure signer: %w", err)
	}

	signed, err := fileStructureSigner.SignFileNodes(fileNodes)
	if err != nil {
		return fmt.Errorf("failed to sign file structure: %w", err)
	}

	if err := c.signaler.SendOffer(ctx, offer, signed); err != nil {
		return fmt.Errorf("failed to send offer via signaler: %w", err)
	}

	answer, err := c.signaler.WaitForAnswer(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for answer: %w", err)
	}

	if err := c.Peer().SetRemoteDescription(*answer); err != nil {
		return fmt.Errorf("failed to set remote description for answer: %w", err)
	}

	return nil
}

func (c *ReceiverConn) HandleOfferAndCreateAnswer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	if err := c.Peer().SetRemoteDescription(offer); err != nil {
		return nil, fmt.Errorf("failed to set remote description: %w", err)
	}

	answer, err := c.Peer().CreateAnswer(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create answer: %w", err)
	}

	if err := c.Peer().SetLocalDescription(answer); err != nil {
		return nil, fmt.Errorf("failed to set local description for answer: %w", err)
	}
	return &answer, nil
}

func (c *SenderConn) CreateDataChannel(label string, options *webrtc.DataChannelInit) (*webrtc.DataChannel, error) {
	return c.peerConnection.CreateDataChannel(label, options)
}

func (c *SenderConn) SendFiles(ctx context.Context, files []fileInfo.FileNode, serviceID string) error {
	// Create unified transfer manager
	utm := transfer.NewUnifiedTransferManager(serviceID)
	defer func() {
		if err := utm.Close(); err != nil {
			slog.Error("Failed to close unified transfer manager", "error", err)
		}
	}()

	// Add files to the transfer manager
	for _, file := range files {
		if err := utm.AddFile(&file); err != nil {
			return fmt.Errorf("failed to add file: %w", err)
		}
	}

	var isChannelReadyClose = false
	channelReady := make(chan struct{})
	channelError := make(chan error, 1)

	dataChannel, err := c.CreateDataChannel("file-transfer", &webrtc.DataChannelInit{
		Ordered: &[]bool{true}[0],
	})
	if err != nil {
		return fmt.Errorf("failed to create data channel: %w", err)
	}

	dataChannel.OnOpen(func() {
		slog.Info("Data channel opened for file transfer")
		close(channelReady)
		isChannelReadyClose = true
	})

	dataChannel.OnError(func(err error) {
		channelError <- err
	})

	// Ensure data channel is closed when function exits
	defer func() {
		if err := dataChannel.Close(); err != nil {
			slog.Error("Failed to close data channel", "error", err)
		}
		// Clean up channels if they weren't closed
		if !isChannelReadyClose {
			close(channelReady)
		}
	}()

	select {
	case <-channelReady:
		slog.Info("Data channel ready, starting file transfer", "serviceID", serviceID)
		return c.performFileTransfer(ctx, dataChannel, utm, serviceID)
	case err := <-channelError:
		return fmt.Errorf("data channel error: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for data channel: %w", ctx.Err())
	}
}

func (c *SenderConn) performFileTransfer(ctx context.Context, dataChannel *webrtc.DataChannel, utm *transfer.UnifiedTransferManager, serviceID string) error {
	slog.Info("Starting file transfer process")
	
	// Process files one by one
	for {
		// Get next pending file
		fileNode, hasMore := utm.GetNextPendingFile()
		if !hasMore {
			slog.Info("All files have been processed")
			break
		}
		
		slog.Info("Starting transfer for file", "path", fileNode.Path, "size", fileNode.Size)
		
		// Start transfer for this file
		if err := utm.StartTransfer(fileNode.Path); err != nil {
			slog.Error("Failed to start transfer", "file", fileNode.Path, "error", err)
			if err := utm.FailTransfer(fileNode.Path, err); err != nil {
				slog.Error("Failed to mark file as failed", "file", fileNode.Path, "error", err)
			}
			continue
		}
		
		// Get chunker for this file
		chunker, exists := utm.GetChunker(fileNode.Path)
		if !exists {
			err := fmt.Errorf("chunker not found for file: %s", fileNode.Path)
			slog.Error("Chunker not found", "file", fileNode.Path)
			if err := utm.FailTransfer(fileNode.Path, err); err != nil {
				slog.Error("Failed to mark file as failed", "file", fileNode.Path, "error", err)
			}
			continue
		}
		
		// Transfer file chunks
		if err := c.transferFileChunks(ctx, dataChannel, utm, fileNode, chunker, serviceID); err != nil {
			slog.Error("Failed to transfer file chunks", "file", fileNode.Path, "error", err)
			if err := utm.FailTransfer(fileNode.Path, err); err != nil {
				slog.Error("Failed to mark file as failed", "file", fileNode.Path, "error", err)
			}
			continue
		}
		
		// Mark file as completed
		if err := utm.CompleteTransfer(fileNode.Path); err != nil {
			slog.Error("Failed to mark file as completed", "file", fileNode.Path, "error", err)
			continue
		}
		
		slog.Info("File transfer completed successfully", "file", fileNode.Path)
	}
	
	slog.Info("File transfer process completed")
	return nil
}

func (c *SenderConn) transferFileChunks(ctx context.Context, dataChannel *webrtc.DataChannel, utm *transfer.UnifiedTransferManager, fileNode *fileInfo.FileNode, chunker *transfer.Chunker, serviceID string) error {
	var totalBytesSent int64 = 0
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Get next chunk
			chunk, err := chunker.Next()
			if err != nil {
				if err == io.EOF {
					// File transfer completed
					return nil
				}
				return fmt.Errorf("failed to get next chunk: %w", err)
			}
			
			// Create chunk message using the correct ChunkMessage structure
			chunkMsg := &transfer.ChunkMessage{
				Type:         transfer.ChunkData,        // Use ChunkData message type
				Session:      *transfer.NewTransferSession(serviceID), // Create session with serviceID
				FileID:       fileNode.Path,             // Use path as file ID
				FileName:     fileNode.Name,
				SequenceNo:   chunk.SequenceNo,
				Data:         chunk.Data,
				ChunkHash:    chunk.Hash,
				TotalSize:    fileNode.Size,
				ExpectedHash: fileNode.Checksum,
			}
			
			// Send chunk
			if err := c.sendMessage(dataChannel, chunkMsg); err != nil {
				return fmt.Errorf("failed to send chunk %d: %w", chunk.SequenceNo, err)
			}
			
			// Update progress
			totalBytesSent += int64(len(chunk.Data))
			if err := utm.UpdateProgress(fileNode.Path, totalBytesSent); err != nil {
				slog.Warn("Failed to update progress", "file", fileNode.Path, "error", err)
			}
			
			// If this was the last chunk, we're done
			if chunk.IsLast {
				return nil
			}
		}
	}
}

func (c *SenderConn) sendMessage(dataChannel *webrtc.DataChannel, msg *transfer.ChunkMessage) error {
	if dataChannel == nil {
		return errors.New("data channel is nil")
	}

	data, err := c.serializer.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return dataChannel.Send(data)
}