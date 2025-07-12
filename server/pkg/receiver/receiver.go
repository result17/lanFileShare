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
	uiMessages chan tea.Msg // Channel to send messages TO the UI
}

// NewApp creates a new receiver application instance.
func NewApp() *App {
	// The API layer will now need a way to send messages back to the app,
	// which will then be forwarded to the UI.
	// We will pass the uiMessages channel down to the api layer.
	uiMessages := make(chan tea.Msg, 5)
	apiHandler := api.NewAPI(uiMessages)
	dnssdlog.Info.SetOutput(io.Discard)
	dnssdlog.Debug.SetOutput(io.Discard)
	return &App{
		guard:      concurrency.NewConcurrencyGuard(),
		registrar:  &discovery.MDNSAdapter{},
		api:        apiHandler,
		uiMessages: uiMessages,
	}
}

// Run starts the application's main event loop and services.
func (a *App) Run(ctx context.Context, port int) {
	// // Start the mDNS registration service in the background.
	a.startRegistration(ctx, port)
	a.startServer(ctx, port)
	<-ctx.Done()
}

func (a *App) UIMessages() <-chan tea.Msg {
	return a.uiMessages
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
