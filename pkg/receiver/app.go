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
	"github.com/pion/webrtc/v4"
	"github.com/google/uuid"
	"github.com/rescp17/lanFileSharer/api"
	"github.com/rescp17/lanFileSharer/internal/app"
	"github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/pkg/concurrency"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	webrtcPkg "github.com/rescp17/lanFileSharer/pkg/webrtc"
)

// App is the main application logic controller for the receiver.
// It manages state, coordinates services, and communicates with the UI.
type App struct {
	guard        *concurrency.ConcurrencyGuard
	registrar    discovery.Adapter
	api          *api.API
	port         int
	uiMessages   chan tea.Msg             // Channel to send messages TO the UI
	appEvents    chan app_events.AppEvent // Channel to receive events FROM the UI
	stateManager *app.StateManager
}

// NewApp creates a new receiver application instance.
func NewApp(port int) *App {
	uiMessages := make(chan tea.Msg, 5)
	// The stateManager needs to be shared between the API and the App logic.
	stateManager := app.NewStateManager()
	apiHandler := api.NewAPI(uiMessages, stateManager)
	dnssdlog.Info.SetOutput(io.Discard)
	dnssdlog.Debug.SetOutput(io.Discard)

	return &App{
		guard:        concurrency.NewConcurrencyGuard(),
		registrar:    &discovery.MDNSAdapter{},
		api:          apiHandler,
		port:         port,
		uiMessages:   uiMessages,
		appEvents:    make(chan app_events.AppEvent),
		stateManager: stateManager,
	}
}

// Run starts the application's main event loop and services.
func (a *App) Run(ctx context.Context, cancel context.CancelFunc) {
	// Start the mDNS registration service in the background.
	a.startRegistration(ctx, a.port)
	a.startServer(ctx, a.port)

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-a.appEvents:
			// Handle events from the TUI
			switch event.(type) {
			case receiver.AcceptFileRequestEvent:
				log.Println("User accepted file transfer. Preparing to receive...")
			
				a.stateManager.SetDecision(app.Accepted)
				webrtcAPI := webrtcPkg.NewWebRTCAPI()
				receiverConn, err := webrtcAPI.NewReceiverConnection(webrtcPkg.Config{})
				if err != nil {
					err := fmt.Errorf("failed to create receiver connection: %w", err)
					a.uiMessages <- receiver.ErrorMsg{Err: err}
					log.Printf("[receiver run] %v", err)
					continue
				}
				receiverConn.OnICECandidate(func(candidate *webrtc.ICECandidate) {
					if candidate == nil {
						log.Println("Receiver: All ICE candidates sent")
						a.stateManager.CloseCandidateChan()
						return
					}
					candidateJSON := candidate.ToJSON()
					a.stateManager.SetCandidate(candidateJSON)
				})

					offer := a.stateManager.GetOffer()
				if offer.SDP == "" {
					err := fmt.Errorf("error: No offer found in state manager")
					log.Printf("[receiver run] %v", err)
					a.uiMessages <- receiver.ErrorMsg{Err: err}
					continue
				}
				
				answer, err := receiverConn.HandleOfferAndCreateAnswer(offer)

				if err != nil {
					err := fmt.Errorf("fail to create answer %w", err)
					log.Fatalf("[receiver run] %v", err)
					return
				}
				log.Printf("Generated placeholder answer: %v", answer)
				a.stateManager.SetAnswer(*answer)

			case receiver.RejectFileRequestEvent:
				log.Println("User rejected file transfer.")
				a.stateManager.SetDecision(app.Rejected)

			default:
				log.Printf("Received unhandled app event: %v", event)
			}
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
			log.Printf("HTTP server ListenAndServe: %v", err)
			a.uiMessages <- receiver.ErrorMsg{Err: fmt.Errorf("failed to create http server %v", err)}
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
