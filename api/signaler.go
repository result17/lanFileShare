package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/pkg/crypto"
)

var ErrTransferRejected = errors.New("transfer rejected by the receiver")

// APISignaler is the client-side implementation of the Signaler interface.
// It communicates with the receiver's API endpoint to exchange WebRTC signaling messages.
type APISignaler struct {
	apiClient           *Client
	receiverURL         string                              // URL of the receiver's API endpoint
	addIceCandidateFunc func(webrtc.ICECandidateInit) error // Callback to add candidates to the sender's connection
	answerChan          chan *webrtc.SessionDescription
	errChan             chan error
}

// NewAPISignaler creates a new signaler for the sender side.
// It requires the receiver's URL and a callback function, which will be used to pass
// ICE candidates received from the receiver back to the sender's PeerConnection.
func NewAPISignaler(
	apiClient *Client,
	receiverURL string,
	addIceCandidateFunc func(webrtc.ICECandidateInit) error,
) *APISignaler {
	return &APISignaler{
		apiClient:           apiClient,
		receiverURL:         receiverURL,
		addIceCandidateFunc: addIceCandidateFunc,
		answerChan:          make(chan *webrtc.SessionDescription, 1),
		errChan:             make(chan error, 1),
	}
}

// SendOffer sends the offer to the receiver and starts listening for the SSE event stream.
// This is the main entry point that triggers the entire signaling process.
func (s *APISignaler) SendOffer(ctx context.Context, offer webrtc.SessionDescription, signedFiles *crypto.SignedFileStructure) error {
	// The /ask endpoint is the single point of contact.
	// It receives the offer and returns an SSE stream.

	endpoint, err := url.JoinPath(s.receiverURL, "ask")

	if err != nil {
		return fmt.Errorf("failed to create ask url: %w", err)
	}

	payload := AskPayload{
		SignedFiles: signedFiles,
		Offer:       offer,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal offer payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create /ask request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err = s.apiClient.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to /ask endpoint: %w", err)
	}

	// Start a goroutine to process the streaming response.
	// The response body will be closed in the goroutine via defer.
	go s.listenToSSEResponse(resp) //nolint:bodyclose // Body is closed in goroutine

	return nil
}

// listenToSSEResponse runs in a goroutine, processing events from the receiver.
func (s *APISignaler) listenToSSEResponse(resp *http.Response) {
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("Failed to close response body", "error", err)
		}
	}()
	scanner := bufio.NewScanner(resp.Body)
	var currentEvent string
	var dataBuffer = &bytes.Buffer{}
	var answerReceived, rejectionReceived bool

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" { // Event boundary
			if dataBuffer.Len() > 0 {
				// Dispatch the buffered data
				s.routeEvent(currentEvent, strings.TrimSuffix(dataBuffer.String(), "\n"))
				dataBuffer.Reset()
			}
			continue
		}

		if eventValue, found := strings.CutPrefix(line, "event:"); found {
			currentEvent = strings.TrimSpace(eventValue)
		} else if dataValue, found := strings.CutPrefix(line, "data:"); found {
			dataBuffer.WriteString(strings.TrimSpace(dataValue))
			dataBuffer.WriteString("\n")

			switch currentEvent {
			case "answer":
				answerReceived = true
			case "rejection":
				rejectionReceived = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		s.sendError(fmt.Errorf("error reading SSE stream: %w", err))
	} else if !answerReceived && !rejectionReceived {
		s.sendError(fmt.Errorf("no answer or rejection received"))
	}
}

// routeEvent dispatches SSE events to the appropriate handler.
func (s *APISignaler) routeEvent(event, data string) {
	switch event {
	case "answer":
		s.handleAnswerEvent(data)
	case "candidate":
		s.handleCandidateEvent(data)
	case "rejection":
		s.sendError(ErrTransferRejected)
	case "candidates_done":
		slog.Info("Receiver has finished sending candidates.")
	default:
		slog.Warn("Received unknown SSE event", "event", event)
	}
}

func (s *APISignaler) handleAnswerEvent(data string) {
	var respData struct {
		Answer webrtc.SessionDescription `json:"answer"`
	}
	// Answer is an important part of WebRTC connection establishment
	if err := json.Unmarshal([]byte(data), &respData); err != nil {
		s.sendError(fmt.Errorf("failed to unmarshal answer event: %w", err))
		return
	}
	s.answerChan <- &respData.Answer
}

func (s *APISignaler) handleCandidateEvent(data string) {
	var respData struct {
		Candidate webrtc.ICECandidateInit `json:"candidate"`
	}
	if err := json.Unmarshal([]byte(data), &respData); err != nil {
		slog.Warn("Failed to unmarshal candidate event", "error", err)
		return // Don't kill the whole connection for one bad candidate
	}

	if err := s.addIceCandidateFunc(respData.Candidate); err != nil {
		slog.Warn("Failed to add ICE candidate", "error", err)
	}
}

// WaitForAnswer blocks until the answer is received from the SSE stream or the context is canceled.
func (s *APISignaler) WaitForAnswer(ctx context.Context) (*webrtc.SessionDescription, error) {
	select {
	case answer := <-s.answerChan:
		if answer == nil {
			return nil, fmt.Errorf("answer is nil") // This should never happen, but just in case
		}
		return answer, nil
	case err := <-s.errChan:
		// TODO: Handle receiver rejection options
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("signaler context canceled: %w", ctx.Err())
	}
}

func (s *APISignaler) SendICECandidate(ctx context.Context, candidate webrtc.ICECandidateInit) error {
	sendCandErr := make(chan error, 1)
	go func() {
		if err := s.apiClient.SendICECandidateRequest(ctx, s.receiverURL, candidate); err != nil {
			slog.Warn("Failed to send ICE candidate to receiver", "error", err, "candidate", candidate.Candidate)
			sendCandErr <- err
		} else {
			slog.Debug("ICE candidate sent successfully", "candidate", candidate.Candidate)
			sendCandErr <- nil
		}
	}()
	select {
	case err := <-sendCandErr:
		return err
	case <-ctx.Done():
		return fmt.Errorf("context canceled while sending ICE candidate: %w", ctx.Err())
	}
}

func (s *APISignaler) sendError(err error) {
	select {
	case s.errChan <- err:
	default:
		slog.Warn("Could not send error on channel (already full)", "error", err)
	}
}
