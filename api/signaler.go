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
	"strings"

	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// APISignaler is the client-side implementation of the Signaler interface.
// It communicates with the Receiver's API endpoint to exchange WebRTC signaling messages.
type APISignaler struct {
	apiClient           *Client
	ctx                 context.Context
	addIceCandidateFunc func(webrtc.ICECandidateInit) error // Callback to add candidates to the sender's connection
	answerChan          chan *webrtc.SessionDescription
	errChan             chan error
}

// 
// Sender
// NewAPISignaler creates a new signaler.
// It requires a callback function, which will be used to pass ICE candidates received
// from the receiver back to the sender's PeerConnection.
func NewAPISignaler(
	ctx context.Context,
	apiClient *Client,
	addIceCandidateFunc func(webrtc.ICECandidateInit) error,
) *APISignaler {
	return &APISignaler{
		apiClient:           apiClient,
		ctx:                 ctx,
		addIceCandidateFunc: addIceCandidateFunc,
		answerChan:          make(chan *webrtc.SessionDescription, 1),
		errChan:             make(chan error, 1),
	}
}

// SendOffer sends the offer to the receiver and starts listening for the SSE event stream.
// This is the main entry point that triggers the entire signaling process.
func (s *APISignaler) SendOffer(offer webrtc.SessionDescription, files []fileInfo.FileNode) error {
	// The /ask endpoint is the single point of contact.
	// It receives the offer and returns an SSE stream.
	url := s.apiClient.receiverURL + "/ask"

	payload := AskPayload{
		Files: files,
		Offer: offer,
		// The actual offer is now part of the AppController/StateManager logic,
		// but for the purpose of the request, we can imagine it being sent here
		// or handled implicitly by the session.
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal offer payload: %w", err)
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create /ask request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := s.apiClient.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to /ask endpoint: %w", err)
	}

	// Do not close the body here. Start a goroutine to process the streaming response.
	go s.listenToSSEResponse(resp)

	return nil
}

// listenToSSEResponse runs in a goroutine, processing events from the receiver.
func (s *APISignaler) listenToSSEResponse(resp *http.Response) {
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	var currentEvent string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			s.routeEvent(currentEvent, data)
		}
	}

	if err := scanner.Err(); err != nil {
		s.errChan <- fmt.Errorf("error reading SSE stream: %w", err)
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
		s.errChan <- errors.New("transfer rejected by the receiver")
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
	if err := json.Unmarshal([]byte(data), &respData); err != nil {
		s.errChan <- fmt.Errorf("failed to unmarshal answer event: %w", err)
		return
	}
	s.answerChan <- &respData.Answer
}

func (s *APISignaler) handleCandidateEvent(data string) {
	var respData struct {
		Candidate webrtc.ICECandidateInit `json:"candidate"`
	}
	if err := json.Unmarshal([]byte(data), &respData); err != nil {
		slog.Error("Failed to unmarshal candidate event", "error", err)
		return // Don't kill the whole connection for one bad candidate.
	}

	if err := s.addIceCandidateFunc(respData.Candidate); err != nil {
		slog.Warn("Failed to add ICE candidate", "error", err)
	}
}

// WaitForAnswer blocks until the answer is received from the SSE stream or the context is cancelled.
func (s *APISignaler) WaitForAnswer(ctx context.Context) (*webrtc.SessionDescription, error) {
	select {
	case answer := <-s.answerChan:
		return answer, nil
	case err := <-s.errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendICECandidate sends a candidate from the sender to the receiver.
// NOTE: The current receiver API in `api/receiver.go` does not have an endpoint
// to handle this. This will require a new endpoint on the receiver side.
func (s *APISignaler) SendICECandidate(candidate webrtc.ICECandidateInit) {
	slog.Info("Sending ICE candidate to receiver", "candidate", candidate)
	// This is a fire-and-forget action.
	go s.apiClient.SendICECandidateRequest(context.Background(), candidate)
}