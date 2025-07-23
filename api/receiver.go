package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/internal/app"
	"github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/pkg/concurrency"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// API is the main entry point for the entire receiver API.
type API struct {
	server *ReceiverGuard
	mux    *http.ServeMux
}

// NewAPI creates and initializes a new API instance.
func NewAPI(uiMessages chan<- tea.Msg, stateManager *app.StateManager) *API {
	api := &API{
		server: NewReceiverGuard(uiMessages, stateManager),
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
	candidateHandlerWithMiddleware := a.server.ConcurrencyControlMiddleware(http.HandlerFunc(a.server.CandidateHandler))

	a.mux.Handle("POST /ask", askHandlerWithMiddleware)
	a.mux.Handle("POST /candidate", candidateHandlerWithMiddleware)
}

// ReceiverGuard manages the server's state and core logic.
type ReceiverGuard struct {
	guard        *concurrency.ConcurrencyGuard
	uiMessages   chan<- tea.Msg // Channel to send messages to the UI
	stateManager *app.StateManager
}

// NewReceiverGuard creates a new ReceiverServer instance.
func NewReceiverGuard(uiMessages chan<- tea.Msg, stateManager *app.StateManager) *ReceiverGuard {
	return &ReceiverGuard{
		guard:        concurrency.NewConcurrencyGuard(),
		uiMessages:   uiMessages,
		stateManager: stateManager,
	}
}

// ConcurrencyControlMiddleware ensures only one request is processed at a time.
func (s *ReceiverGuard) ConcurrencyControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		task := func() error {
			defer s.stateManager.CloseRequest() // Ensure state is cleaned up
			next.ServeHTTP(w, r)
			// Block until the transfer is fully complete
			<-s.stateManager.WaitForTransferDone()
			return nil
		}

		err := s.guard.Execute(task)
		if errors.Is(err, concurrency.ErrBusy) {
			log.Println("Request rejected, server is busy!")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"error": concurrency.ErrBusy.Error(),
			})
		}
	})
}

// AskPayload is the structure of the request body for the /ask endpoint.
type AskPayload struct {
	Files []fileInfo.FileNode `json:"files"`
	Offer webrtc.SessionDescription `json:"offer"`
}

// AskHandler is the core business logic for handling /ask requests.
// It now orchestrates calls to more specific helper methods.
func (s *ReceiverGuard) AskHandler(w http.ResponseWriter, r *http.Request) {
	var req AskPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	slog.Info("Ask received", "request", req)
	
	s.stateManager.SetOffer(req.Offer)
	decisionChan, err := s.stateManager.CreateRequest()
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	s.uiMessages <- receiver.FileNodeUpdateMsg{Nodes: req.Files}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	decision := <-decisionChan
	if decision == app.Rejected {
		slog.Info("Request rejected by user")
		s.sendRejection(w, flusher)
		return
	}

	slog.Info("Request accepted by user")
	if err := s.sendAnswer(w, flusher); err != nil {
		slog.Error("Failed to send answer", "error", err)
		// Don't write an http.Error here as the response has started
		return
	}

	if err := s.streamCandidates(w, flusher); err != nil {
		slog.Error("Failed to stream candidates", "error", err)
	}
}

// sendRejection sends a rejection message to the sender.
func (s *ReceiverGuard) sendRejection(w http.ResponseWriter, flusher http.Flusher) {
	response := map[string]string{"status": "rejected"}
	jsonResponse, _ := json.Marshal(response)
	fmt.Fprintf(w, "event: rejection\ndata: %s\n\n", jsonResponse)
	flusher.Flush()
}

// sendAnswer waits for the WebRTC answer and sends it as an SSE event.
func (s *ReceiverGuard) sendAnswer(w http.ResponseWriter, flusher http.Flusher) error {
	answerChan := s.stateManager.GetAnswerChan()
	answer, ok := <-answerChan
	if !ok {
		return fmt.Errorf("answer channel was closed unexpectedly")
	}

	slog.Info("Sending answer to sender", "answer", answer)
	jsonResponse, err := json.Marshal(answer)
	if err != nil {
		return fmt.Errorf("failed to marshal answer: %w", err)
	}

	fmt.Fprintf(w, "event: answer\ndata: %s\n\n", jsonResponse)
	flusher.Flush()
	return nil
}

// streamCandidates waits for ICE candidates and streams them as SSE events.
func (s *ReceiverGuard) streamCandidates(w http.ResponseWriter, flusher http.Flusher) error {
	slog.Info("Now streaming ICE candidates to sender...")
	candidateChan := s.stateManager.GetCandidateChan()

	for candidate := range candidateChan {
		slog.Info("Sending candidate to sender", "candidate", candidate)
		response := map[string]webrtc.ICECandidateInit{"candidate": candidate}
		jsonResponse, err := json.Marshal(response)
		if err != nil {
			slog.Error("Failed to marshal candidate, skipping", "error", err)
			continue // Don't let one bad candidate stop the stream
		}
		fmt.Fprintf(w, "event: candidate\ndata: %s\n\n", jsonResponse)
		flusher.Flush()
	}

	slog.Info("Finished streaming candidates.")
	// Send a final event to signal the end of the stream
	fmt.Fprintf(w, "event: candidates_done\ndata: {}\n\n")
	flusher.Flush()
	return nil
}

func (s *ReceiverGuard) CandidateHandler(w http.ResponseWriter, r *http.Request) {
	var req webrtc.ICECandidateInit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid candidate payload", http.StatusBadRequest)
		return
	}
	slog.Info("Candidate received", "request", req)

	s.stateManager.SetCandidate(req)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Candidate received successfully")
}
