package receiver

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	dnssdlog "github.com/brutella/dnssd/log"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/api"
	"github.com/rescp17/lanFileSharer/internal/app"
	"github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/pkg/concurrency"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	webrtcPkg "github.com/rescp17/lanFileSharer/pkg/webrtc"
)

// App is the main application logic controller for the receiver.
type App struct {
	guard                *concurrency.ConcurrencyGuard
	registrar            discovery.Adapter
	api                  *api.API
	port                 int
	uiMessages           chan tea.Msg
	appEvents            chan app_events.AppEvent
	stateManager         *app.StateManager
	inboundCandidateChan chan webrtc.ICECandidateInit
	activeConn           *webrtcPkg.ReceiverConn
	connMu               sync.Mutex
}

// NewApp creates a new receiver application instance.
func NewApp(port int) *App {
	uiMessages := make(chan tea.Msg, 10)
	stateManager := app.NewStateManager()

	// We will pass a reference to App itself later to solve the dependency cycle.
	apiHandler := api.NewAPI(uiMessages, stateManager)

	dnssdlog.Info.SetOutput(io.Discard)
	dnssdlog.Debug.SetOutput(io.Discard)

	return &App{
		guard:                concurrency.NewConcurrencyGuard(),
		registrar:            &discovery.MDNSAdapter{},
		api:                  apiHandler,
		port:                 port,
		uiMessages:           uiMessages,
		appEvents:            make(chan app_events.AppEvent),
		stateManager:         stateManager,
		inboundCandidateChan: make(chan webrtc.ICECandidateInit, 10),
	}
}

// InboundCandidateChan provides a channel for the API layer to send candidates to the app logic.
func (a *App) InboundCandidateChan() chan<- webrtc.ICECandidateInit {
	return a.inboundCandidateChan
}

// Run starts the application's main event loop and services.
func (a *App) Run(ctx context.Context, cancel context.CancelFunc) {
	a.startRegistration(ctx, a.port)
	a.startServer(ctx, a.port)

	for {
		select {
		case <-ctx.Done():
			return
		case candidate := <-a.inboundCandidateChan:
			a.connMu.Lock()
			if a.activeConn != nil {
				if err := a.activeConn.Peer().AddICECandidate(candidate); err != nil {
					slog.Warn("Failed to add inbound ICE candidate", "error", err)
				}
			}
			a.connMu.Unlock()
		case event := <-a.appEvents:
			switch event.(type) {
			case receiver.AcceptFileRequestEvent:
				go a.handleAcceptFileRequest()
			case receiver.RejectFileRequestEvent:
				slog.Info("User rejected file transfer.")
				a.stateManager.SetDecision(app.Rejected)
			default:
				log.Printf("Received unhandled app event: %v", event)
			}
		}
	}
}

// handleAcceptFileRequest contains the logic for setting up a WebRTC connection.
func (a *App) handleAcceptFileRequest() {
	slog.Info("[handleAcceptFileRequest] User accepted file transfer. Preparing to receive...")
	a.stateManager.SetDecision(app.Accepted)

	WebrtcAPI := webrtcPkg.NewWebrtcAPI()
	receiverConn, err := WebrtcAPI.NewReceiverConnection(webrtcPkg.Config{})
	if err != nil {
		a.uiMessages <- receiver.ErrorMsg{Err: fmt.Errorf("failed to create receiver connection: %w", err)}
		return
	}

	// Store the connection so it can be accessed by the candidate handler
	a.connMu.Lock()
	a.activeConn = receiverConn
	a.connMu.Unlock()

	// Ensure the connection is closed and cleaned up when this session is over.
	receiverConn.Peer().OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog.Info("Peer Connection state has changed", "state", state)
		if state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateDisconnected {
			slog.Info("Closing active connection due to state change.")
			a.connMu.Lock()
			if a.activeConn != nil {
				a.activeConn.Close()
				a.activeConn = nil
			}
			a.connMu.Unlock()
		}
	})

	receiverConn.Peer().OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			slog.Info("All local ICE candidates gathered.")
			a.stateManager.CloseCandidateChan()
			return
		}
		slog.Info("Got a local ICE candidate", "candidate", candidate.ToJSON().Candidate)
		a.stateManager.SetCandidate(candidate.ToJSON())
	})

	offer := a.stateManager.GetOffer()
	if offer.SDP == "" {
		err := fmt.Errorf("failed to get offer from state manager")
		log.Printf("[handleAcceptFileRequest] %v", err)
		a.uiMessages <- receiver.ErrorMsg{Err: err}
		return
	}

	answer, err := receiverConn.HandleOfferAndCreateAnswer(offer)
	if err != nil {
		err = fmt.Errorf("fail to create answer: %w", err)
		a.uiMessages <- receiver.ErrorMsg{Err: err}
		return
	}

	a.stateManager.SetAnswer(*answer)
	slog.Info("Answer created and sent to state manager.")
}

func (a *App) UIMessages() <-chan tea.Msg {
	return a.uiMessages
}

func (a *App) AppEvents() chan<- app_events.AppEvent {
	return a.appEvents
}

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
		Addr:   nil,
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

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()
}
