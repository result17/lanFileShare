package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
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
	a.mux.Handle("POST /ask", askHandlerWithMiddleware)
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
	Offer string              `json:"offer"`
}

// AskHandler is the core business logic for handling /ask requests.
func (s *ReceiverGuard) AskHandler(w http.ResponseWriter, r *http.Request) {
	var req AskPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	slog.Info("Ask received", "request", req)

	decisionChan, err := s.stateManager.CreateRequest(req.Offer)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Send file info to the UI
	s.uiMessages <- receiver.FileNodeUpdateMsg{Nodes: req.Files}

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Wait for the user's decision from the UI
	decision := <-decisionChan

	if decision == app.Rejected {
		slog.Info("Request rejected by user")
		fmt.Fprintf(w, "data: %s\n\n", `{"status": "rejected"}`)
		flusher.Flush()
		return
	}

	slog.Info("Request accepted by user")
	// The AppController is now responsible for creating the peer connection
	// and generating the answer. We just need to wait for the answer.
	answerChan := s.stateManager.GetAnswerChan()
	answer := <-answerChan

	slog.Info("Sending answer to sender", "answer", answer)
	response := map[string]string{
		"status": "accepted",
		"answer": answer,
	}
	jsonResponse, _ := json.Marshal(response)
	fmt.Fprintf(w, "data: %s\n\n", jsonResponse)
	flusher.Flush()
}

