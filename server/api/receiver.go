package api

import (
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/rescp17/lanFileSharer/pkg/concurrency"
)

// API is the main entry point for the entire receiver API.
// It is responsible for routing, middleware, and handling logic.
type API struct {
	server *ReceiverGuard
	mux    *http.ServeMux
}

// NewAPI creates and initializes a new API instance.
// It sets up all routes and middleware.
func NewAPI() *API {
	api := &API{
		server: NewReceiverGuard(),
		mux:    http.NewServeMux(),
	}
	api.registerRoutes()
	return api
}

// ServeHTTP allows the API struct to satisfy the http.Handler interface.
// This allows it to be used directly in an http.Server.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

// registerRoutes connects all handlers and middleware.
func (a *API) registerRoutes() {
	// Wrap the business handler with the concurrency control middleware.
	askHandlerWithMiddleware := a.server.ConcurrencyControlMiddleware(http.HandlerFunc(a.server.AskHandler))
	a.mux.Handle("POST /ask", askHandlerWithMiddleware)
}

// ReceiverGuard manages the server's state and core logic.
type ReceiverGuard struct {
	guard *concurrency.ConcurrencyGuard
}

// NewReceiverGuard creates a new ReceiverServer instance.
func NewReceiverGuard() *ReceiverGuard {
	return &ReceiverGuard{
		guard: concurrency.NewConcurrencyGuard(),
	}
}

// ConcurrencyControlMiddleware is a middleware that ensures only one request is processed at a time.
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

// AskHandler is the core business logic for handling /ask requests.
// It is now a method of ReceiverGuard.
func (s *ReceiverGuard) AskHandler(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WebSocket upgrade error:", err)
			http.Error(w, "WebSocket upgrade failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		// exchange sdp

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println("WebSocket read error:", err)
				break
			}
			log.Printf("Received WS message: %s", msg)
			// reply answer
		}
		return
	}

	// This part of the logic is now protected by middleware, so there are no concurrency issues.
	var req AskPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	slog.Info("Ask received", "request", req)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Accepted"))

	// TODO show file information
}
