package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/internal/app"
	"github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/pkg/concurrency"
	"github.com/rescp17/lanFileSharer/pkg/crypto"
)

// API is the main entry point for the entire receiver API.
type API struct {
	server *ReceiverService
	mux    *http.ServeMux
}

// AskPayload is the structure of the request body for the /ask endpoint.
type AskPayload struct {
	SignedFiles *crypto.SignedFileStructure `json:"signed_files"`
	Offer       webrtc.SessionDescription   `json:"offer"`
}

// NewAPI creates and initializes a new API instance.
func NewAPI(uiMessages chan<- tea.Msg, stateManager *app.SingleRequestManager) *API {
	api := &API{
		server: NewReceiverService(uiMessages, stateManager),
		mux:    http.NewServeMux(),
	}
	api.registerRoutes()
	return api
}

// ServeHTTP allows the API struct to satisfy the http.Handler interface.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

// registerRoutes connects all handlers and middleware.
func (a *API) registerRoutes() {
	askHandlerWithMiddleware := a.server.ConcurrencyControlMiddleware(http.HandlerFunc(a.server.AskHandler))
	a.mux.HandleFunc("POST /ask", askHandlerWithMiddleware.ServeHTTP)
	a.mux.HandleFunc("POST /candidate", a.server.CandidateHandler)
}

// ReceiverService manages the server's state and core logic.
type ReceiverService struct {
	guard        *concurrency.ConcurrencyGuard
	uiMessages   chan<- tea.Msg // Channel to send messages to the UI
	stateManager *app.SingleRequestManager
}

// NewReceiverService creates a new ReceiverServer instance.
func NewReceiverService(uiMessages chan<- tea.Msg, stateManager *app.SingleRequestManager) *ReceiverService {
	return &ReceiverService{
		guard:        concurrency.NewConcurrencyGuard(),
		uiMessages:   uiMessages,
		stateManager: stateManager,
	}
}

// ConcurrencyControlMiddleware ensures only one request is processed at a time.
func (s *ReceiverService) ConcurrencyControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		task := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("recovered from panic in HTTP handler", "panic", r)
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			next.ServeHTTP(w, r)
			return
		}

		err := s.guard.Execute(task)
		if errors.Is(err, concurrency.ErrBusy) {
			slog.Info("Request rejected, server is busy!")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			err := json.NewEncoder(w).Encode(map[string]string{
				"error": concurrency.ErrBusy.Error(),
			})
			if err != nil {
				slog.Error("Failed to encode busy response", "error", err)
			}
		} else if err != nil {
			slog.Error("Unexpected error in concurrency control middleware", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})
}

// AskHandler is the core business logic for handling /ask requests.
func (s *ReceiverService) AskHandler(w http.ResponseWriter, r *http.Request) {
	var req AskPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode request", "error", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	slog.Info("Ask received", "offer_type", req.Offer.Type)
	if err := crypto.VerifyFileStructure(req.SignedFiles); err != nil {
		slog.Error("failed to verify file structure", "error", err)
		http.Error(w, "Invalid file structure", http.StatusBadRequest)
		return
	}
	slog.Info("success to verify file structure")

	decisionChan, err := s.stateManager.CreateRequest(req.Offer)
	if err != nil {
		slog.Error("failed to create request", "error", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	defer s.stateManager.CloseRequest()

	s.uiMessages <- receiver.FileNodeUpdateMsg{Nodes: req.SignedFiles.Files}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		slog.Error("failed to support streaming")
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	select {
	case <-r.Context().Done():
		slog.Warn("Request context cancelled before processing")
		return
	case decision, ok := <-decisionChan:
		if !ok {
			slog.Warn("Decision channel closed unexpectedly")
			if err := s.sendRejection(w, flusher); err != nil {
				slog.Error("Failed to send rejection", "error", err)
			}
			return
		}
		if decision == app.Rejected {
			slog.Info("Request rejected by user")
			if err := s.sendRejection(w, flusher); err != nil {
				slog.Error("Failed to send rejection", "error", err)
			}
			return
		}
	}

	slog.Info("Request accepted by user")
	if err := s.sendAnswer(w, flusher, r.Context()); err != nil {
		slog.Error("Failed to send answer", "error", err)
		sendErrorEvent(w, flusher, err)
		return
	}

	if err := s.streamCandidates(w, flusher, r.Context()); err != nil {
		slog.Error("Failed to stream candidates", "error", err)
		sendErrorEvent(w, flusher, err)
	}
}

// sendRejection sends a rejection message to the sender.
func (s *ReceiverService) sendRejection(w http.ResponseWriter, flusher http.Flusher) error {
	response := map[string]string{"status": "rejected"}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		slog.Error("Failed to marshal rejection response", "error", err)
		return fmt.Errorf("failed to marshal rejection response: %w", err)
	}
	if _, err := fmt.Fprintf(w, "event: rejection\ndata: %s\n\n", jsonResponse); err != nil {
		return fmt.Errorf("failed to write rejection event: %w", err)
	}

	flusher.Flush()
	return nil
}

// sendAnswer waits for the WebRTC answer and sends it as an SSE event.
func (s *ReceiverService) sendAnswer(w http.ResponseWriter, flusher http.Flusher, ctx context.Context) error {
	answerChan := s.stateManager.GetAnswerChan()

	var answer webrtc.SessionDescription
	var ok bool

	select {
	case answer, ok = <-answerChan:
		if !ok {
			return fmt.Errorf("answer channel was closed unexpectedly")
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	slog.Info("Sending answer to sender", "answer_type", answer.Type)

	response := map[string]webrtc.SessionDescription{"answer": answer}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal answer: %w", err)
	}

	if _, err := fmt.Fprintf(w, "event: answer\ndata: %s\n\n", jsonResponse); err != nil {
		return fmt.Errorf("failed to write answer to response: %w", err)
	}
	flusher.Flush()
	return nil
}

// streamCandidates waits for ICE candidates and streams them as SSE events.
func (s *ReceiverService) streamCandidates(w http.ResponseWriter, flusher http.Flusher, ctx context.Context) error {
	slog.Info("Now streaming ICE candidates to sender...")
	candidateChan := s.stateManager.GetCandidateChan()

	// Ensure that the implementation guarantees that will always be closed after the peer connection is established or has failed
	for {
		select {
		case <-ctx.Done():
			slog.Warn("Client disconnected, stopping candidate streaming.")
			return ctx.Err()
		case candidate, ok := <-candidateChan:
			if !ok {
				// Channel closed, successfully finished streaming.
				slog.Info("Finished streaming candidates.")
				_, err := fmt.Fprintf(w, "event: candidates_done\ndata: {}\n\n")
				if err != nil {
					return fmt.Errorf("failed to write candidates_done event: %w", err)
				}
				flusher.Flush()
				return nil
			}

			slog.Info("Sending candidate to sender", "candidate", candidate.Candidate)
			response := map[string]webrtc.ICECandidateInit{"candidate": candidate}
			jsonResponse, err := json.Marshal(response)
			if err != nil {
				slog.Error("Failed to marshal candidate, skipping", "error", err)
				continue
			}

			if _, err := fmt.Fprintf(w, "event: candidate\ndata: %s\n\n", jsonResponse); err != nil {
				return fmt.Errorf("failed to write candidate to response: %w", err)
			}
			flusher.Flush()
		}
	}
}

func (s *ReceiverService) CandidateHandler(w http.ResponseWriter, r *http.Request) {
	var req webrtc.ICECandidateInit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid candidate payload", http.StatusBadRequest)
		return
	}
	slog.Info("Candidate received", "request", req)

	if err := s.stateManager.SetCandidate(req); err != nil {
		slog.Error("Failed to add ICE candidate", "error", err)
		// Consider what status code is appropriate here, e.g., 500
		http.Error(w, "Failed to process ICE candidate", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Candidate received successfully",
	}); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

func sendErrorEvent(w http.ResponseWriter, flusher http.Flusher, err error) {
	response := map[string]string{"error": err.Error()}
	jsonResponse, marshalErr := json.Marshal(response) // Marshalling a simple map shouldn't fail
	if marshalErr != nil {
		slog.Error("failed to marshal error response", "error", marshalErr)
		return
	}
	if _, writeErr := fmt.Fprintf(w, "event: error\ndata: %s\n\n", jsonResponse); writeErr != nil {
		slog.Warn("failed to write error event to client", "error", writeErr)
	}
	flusher.Flush()
}
