package api

import (
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"github.com/rescp17/lanFileSharer/pkg/concurrency"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// fileNodeUpdateMsg is a message sent to the UI to update it with file info.
// Note: This is a simplified message. In a real app, you'd have separate
// messages for start, progress, and completion.
type fileNodeUpdateMsg struct {
	Node fileInfo.FileNode
}

// API is the main entry point for the entire receiver API.
type API struct {
	server *ReceiverGuard
	mux    *http.ServeMux
}

// NewAPI creates and initializes a new API instance.
func NewAPI(uiMessages chan<- tea.Msg) *API {
	api := &API{
		server: NewReceiverGuard(uiMessages),
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
	guard      *concurrency.ConcurrencyGuard
	uiMessages chan<- tea.Msg // Channel to send messages to the UI
}

// NewReceiverGuard creates a new ReceiverServer instance.
func NewReceiverGuard(uiMessages chan<- tea.Msg) *ReceiverGuard {
	return &ReceiverGuard{
		guard:      concurrency.NewConcurrencyGuard(),
		uiMessages: uiMessages,
	}
}

// ConcurrencyControlMiddleware ensures only one request is processed at a time.
func (s *ReceiverGuard) ConcurrencyControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		task := func() error {
			next.ServeHTTP(w, r)
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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// AskPayload is the structure of the request body for the /ask endpoint.
type AskPayload struct {
	Files []fileInfo.FileNode `json:"files"`
}

// AskHandler is the core business logic for handling /ask requests.
func (s *ReceiverGuard) AskHandler(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		// ... (WebSocket logic remains the same)
		return
	}

	var req AskPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	slog.Info("Ask received", "request", req)

	// For now, just take the first file to demonstrate communication.
	if len(req.Files) > 0 {
		firstFile := req.Files[0]
		// Send the file information to the UI via the channel.
		s.uiMessages <- fileNodeUpdateMsg{Node: firstFile}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Accepted"))

	// In a real implementation, you would now prepare for the actual file transfer.
	// After the transfer is complete, you would send a completion message.
	// e.g., s.uiMessages <- transferCompleteMsg{}
}

