package receiver

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	dnssdlog "github.com/brutella/dnssd/log"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/rescp17/lanFileSharer/api"
	"github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/pkg/concurrency"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
)

// App is the main application logic controller for the receiver.
// It manages state, coordinates services, and communicates with the UI.
type App struct {
	guard      *concurrency.ConcurrencyGuard
	registrar  discovery.Adapter
	api        *api.API
	uiMessages chan tea.Msg             // Channel to send messages TO the UI
	appEvents  chan app_events.AppEvent // Channel to receive events FROM the UI
}

// NewApp creates a new receiver application instance.
func NewApp() *App {
	uiMessages := make(chan tea.Msg, 5)
	apiHandler := api.NewAPI(uiMessages)
	dnssdlog.Info.SetOutput(io.Discard)
	dnssdlog.Debug.SetOutput(io.Discard)
	return &App{
		guard:      concurrency.NewConcurrencyGuard(),
		registrar:  &discovery.MDNSAdapter{},
		api:        apiHandler,
		uiMessages: uiMessages,
		appEvents:  make(chan app_events.AppEvent),
	}
}

// Run starts the application's main event loop and services.
func (a *App) Run(ctx context.Context, cancel context.CancelFunc) {
	// Start the mDNS registration service in the background.
	a.startRegistration(ctx, 8080) // Assuming a default port for now
	a.startServer(ctx, 8080)

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-a.appEvents:
			// Handle events from the TUI (e.g., accept/reject transfer)
			// Placeholder for now
			log.Printf("Received app event: %#v", event)
		}
	}
}

func (a *App) UIMessages() <-chan tea.Msg {
	return a.uiMessages
}

// AppEvents returns a write-only channel for the TUI to send events to the app.
func (a *App) AppEvents() chan<- app_events.AppEvent {
	return a.appEvents
}

// startRegistration announces the receiver's presence on the network.
func (a *App) startRegistration(ctx context.Context, port int) {
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Could not get hostname %v", err)
		a.uiMessages <- receiver.ErrorMsg{Err: err}
		return
	}
	serviceUUID := uuid.New().String()

	serviceInfo := discovery.ServiceInfo{
		Name:   fmt.Sprintf("%s-%s", hostname, serviceUUID[:8]),
		Type:   discovery.DefaultServerType,
		Domain: discovery.DefaultDomain,
		Addr:   nil, // This will be set by the discovery package.
		Port:   port,
	}

	go func() {
		err := a.registrar.Announce(ctx, serviceInfo)
		if err != nil {
			err := fmt.Errorf("failed to start announce: %w", err)
			log.Printf("[Discover announce]: %v", err)
			a.uiMessages <- receiver.ErrorMsg{Err: err}
		}
	}()
}

// startServer starts the HTTP server in a goroutine.
func (a *App) startServer(ctx context.Context, port int) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: a.api,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
			a.uiMessages <- receiver.ErrorMsg{ Err: fmt.Errorf("failed to create http server") }
		}
	}()

	// Listen for context cancellation to gracefully shut down the server.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()
}
