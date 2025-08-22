package webrtc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

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
	Establish(ctx context.Context, fileNodes *transfer.FileStructureManager) error
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
	signaler         Signaler // Used to send signals to the remote peer
	serializer       transfer.MessageSerializer
	progressSignaler ProgressSignaler // Optional progress signaler
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
	return a.NewSenderConnectionWithProgress(transferCtx, config, apiClient, receiverURL, nil)
}

func (a *WebrtcAPI) NewSenderConnectionWithProgress(transferCtx context.Context, config Config, apiClient *api.Client, receiverURL string, progressSignaler ProgressSignaler) (SenderConnection, error) {
	pc, err := a.createPeerConnection(config)
	if err != nil {
		return nil, err
	}
	conn := &SenderConn{
		Connection: &Connection{
			peerConnection: pc,
		},
		serializer:       transfer.NewJSONSerializer(),
		progressSignaler: progressSignaler,
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

func (c *SenderConn) Establish(ctx context.Context, fsm *transfer.FileStructureManager) error {
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

	signed, err := fileStructureSigner.SignFileStructureManager(fsm)
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

	// Add progress listener if progress signaler is available
	if c.progressSignaler != nil {
		progressListener := NewProgressListener(c.progressSignaler)
		utm.AddStatusListener(progressListener)
	}

	// Add files to the transfer manager
	for _, file := range files {
		if err := utm.AddFile(&file); err != nil {
			return fmt.Errorf("failed to add file: %w", err)
		}
	}

	var channelReadyOnce sync.Once
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
		channelReadyOnce.Do(func() { close(channelReady) })
	})

	dataChannel.OnError(func(err error) {
		select {
		case channelError <- err:
		default:
		}
		channelReadyOnce.Do(func() { close(channelReady) })
	})

	// Ensure data channel is closed when function exits
	defer func() {
		if err := dataChannel.Close(); err != nil {
			slog.Error("Failed to close data channel", "error", err)
		}
		// Clean up channels if they weren't closed
		channelReadyOnce.Do(func() { close(channelReady) })
	}()

	select {
	case <-channelReady:
		slog.Info("Data channel ready, starting file transfer", "serviceID", serviceID)
		return c.performFileTransfer(ctx, dataChannel, utm, serviceID)
	case err := <-channelError:
		return fmt.Errorf("data channel error: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("context canceled while waiting for data channel: %w", ctx.Err())
	}
}

func (c *SenderConn) performFileTransfer(ctx context.Context, dataChannel *webrtc.DataChannel, utm *transfer.UnifiedTransferManager, serviceID string) error {
	slog.Info("Starting file transfer process")

	// Helper closure to handle transfer failures gracefully
	// This provides resilient error handling that continues processing other files
	handleTransferFailure := func(filePath string, transferErr error, context string) {
		slog.Error("Transfer failure", "file", filePath, "context", context, "error", transferErr)

		// Attempt to mark file as failed, but don't abort the entire batch if this fails
		if failErr := utm.FailTransfer(filePath, transferErr); failErr != nil {
			// Log the secondary failure but continue processing other files
			slog.Warn("Failed to mark file as failed (secondary failure)",
				"file", filePath,
				"original_error", transferErr,
				"fail_error", failErr)
			// Note: We don't return here - we continue with the next file
		}
	}

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
			handleTransferFailure(fileNode.Path, err, "start transfer")
			continue
		}

		// Get chunker for this file
		chunker, exists := utm.GetChunker(fileNode.Path)
		if !exists {
			err := fmt.Errorf("chunker not found for file: %s", fileNode.Path)
			handleTransferFailure(fileNode.Path, err, "get chunker")
			continue
		}

		// Transfer file chunks
		if err := c.transferFileChunks(ctx, dataChannel, utm, fileNode, chunker, serviceID); err != nil {
			handleTransferFailure(fileNode.Path, err, "transfer chunks")
			continue
		}

		// Mark file as completed
		if err := utm.CompleteTransfer(fileNode.Path); err != nil {
			slog.Error("Failed to mark file as completed", "file", fileNode.Path, "error", err)
			// Note: We don't use handleTransferFailure here because the file was actually transferred successfully
			// The failure is only in marking it as completed, which is less critical
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
				Type:         transfer.ChunkData,                      // Use ChunkData message type
				Session:      *transfer.NewTransferSession(serviceID), // Create session with serviceID
				FileID:       fileNode.Path,                           // Use path as file ID
				FileName:     fileNode.Name,
				SequenceNo:   chunk.SequenceNo,
				Offset:       chunk.Offset, // Add offset to support out-of-order writes
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

// ProgressSignaler interface for sending progress updates
type ProgressSignaler interface {
	SendProgressUpdate(totalFiles, completedFiles int, totalBytes, transferredBytes int64,
		currentFile string, transferRate float64, eta string, overallProgress float64)
}

// ProgressListener implements transfer.StatusListener to send progress updates
type ProgressListener struct {
	signaler       ProgressSignaler
	lastUpdate     time.Time
	updateInterval time.Duration
}

// NewProgressListener creates a new progress listener
func NewProgressListener(signaler ProgressSignaler) *ProgressListener {
	return &ProgressListener{
		signaler:       signaler,
		updateInterval: 500 * time.Millisecond, // Update every 500ms
	}
}

// ID returns the listener ID
func (pl *ProgressListener) ID() string {
	return "webrtc-progress-listener"
}

// OnFileStatusChanged handles file status changes
func (pl *ProgressListener) OnFileStatusChanged(filePath string, oldStatus, newStatus *transfer.TransferStatus) {
	// File-level changes don't need immediate UI updates
	// We'll rely on session-level updates for better performance
}

// OnSessionStatusChanged handles session status changes and sends progress updates
func (pl *ProgressListener) OnSessionStatusChanged(oldStatus, newStatus *transfer.SessionTransferStatus) {
	// Throttle updates to avoid overwhelming the UI
	now := time.Now()
	if now.Sub(pl.lastUpdate) < pl.updateInterval {
		return
	}
	pl.lastUpdate = now

	// Calculate transfer rate and ETA
	var transferRate float64
	var eta string

	if newStatus.CurrentFile != nil {
		transferRate = newStatus.CurrentFile.TransferRate

		// Calculate ETA based on remaining bytes and current rate
		if transferRate > 0 {
			remainingBytes := newStatus.TotalBytes - newStatus.BytesCompleted
			etaSeconds := float64(remainingBytes) / transferRate
			if etaSeconds > 0 && etaSeconds < 3600 { // Only show ETA if less than 1 hour
				eta = pl.formatDuration(time.Duration(etaSeconds * float64(time.Second)))
			}
		}
	}

	// Get current file name
	currentFile := ""
	if newStatus.CurrentFile != nil {
		currentFile = pl.extractFileName(newStatus.CurrentFile.FilePath)
	}

	// Send progress update
	pl.signaler.SendProgressUpdate(
		newStatus.TotalFiles,
		newStatus.CompletedFiles,
		newStatus.TotalBytes,
		newStatus.BytesCompleted,
		currentFile,
		transferRate,
		eta,
		newStatus.OverallProgress,
	)
}

// formatDuration formats a duration into a human-readable string
func (pl *ProgressListener) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return "∞"
}

// extractFileName extracts the file name from a file path
func (pl *ProgressListener) extractFileName(filePath string) string {
	// Simple implementation - in production, use filepath.Base()
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '/' || filePath[i] == '\\' {
			return filePath[i+1:]
		}
	}
	return filePath
}
