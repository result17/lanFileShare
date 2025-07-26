package receiver

import (
	"context"
	"fmt"
	"io"
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
	a.startRegistration(ctx, a.port, cancel)
	a.startServer(ctx, a.port, cancel)

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
			} else {
				 slog.Warn("Received an ICE candidate but there is no active connection.")
			}
			a.connMu.Unlock()
		case event := <-a.appEvents:
			switch event.(type) {
			case receiver.AcceptFileRequestEvent:
				go a.guard.Execute(func() error {
					return a.handleAcceptFileRequest(ctx)
				})
			case receiver.RejectFileRequestEvent:
				slog.Info("User rejected file transfer.")
				a.stateManager.SetDecision(app.Rejected)
			default:
				slog.Warn("Received unhandled app event", "event", event)
			}
		}
	}
}

// sendAndLogError is a helper function to both log an error and send it to the UI.
func (a *App) sendAndLogError(baseMessage string, err error) {
	slog.Error(baseMessage, "error", err)
	a.uiMessages <- app_events.ErrorMsg{Err: fmt.Errorf("%s: %w", baseMessage, err)}
}

// handleAcceptFileRequest contains the logic for setting up a WebRTC connection.
func (a *App) handleAcceptFileRequest(ctx context.Context) error {
	slog.Info("User accepted file transfer. Preparing to receive...")
	hctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	a.stateManager.SetDecision(app.Accepted)

	webRTCAPI := webrtcPkg.NewWebrtcAPI()
	receiverConn, err := webRTCAPI.NewReceiverConnection(webrtcPkg.Config{})
	a.setActiveConn(receiverConn)
	if err != nil {
		a.sendAndLogError("Failed to create receiver connection", err)
		return err
	}

	receiverConn.Peer().OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog.Info("Peer Connection State has changed", "state", state.String())
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateDisconnected {
			slog.Info("Closing active connection due to state change.")
			a.closeActiveConnection()
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

	offer, err := a.stateManager.GetOffer()
	if err != nil {
		a.sendAndLogError("Could not get offer from state", err)
		return err
	}

	answer, err := receiverConn.HandleOfferAndCreateAnswer(offer)
	if err != nil {
		a.sendAndLogError("Failed to create answer", err)
		return err
	}

	select {
	case <-hctx.Done():
		slog.Warn("Handshake cancelled or timed out before sending answer.", "error", hctx.Err())
		return hctx.Err()
	default:
		a.stateManager.SetAnswer(*answer)
		slog.Info("Answer created and sent to state manager.")
		return nil
	}

}

func (a *App) UIMessages() <-chan tea.Msg {
	return a.uiMessages
}

func (a *App) AppEvents() chan<- app_events.AppEvent {
	return a.appEvents
}

func (a *App) startRegistration(ctx context.Context, port int, cancel context.CancelFunc) {
	hostname, err := os.Hostname()
	if err != nil {
		a.sendAndLogError("Could not get hostname", err)
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
			a.sendAndLogError("Failed to start mDNS announcement", err)
			// exit the app if we can't announce
			cancel()
			return
		}
	}()
}

func (a *App) startServer(ctx context.Context, port int, cancel context.CancelFunc) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: a.api,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.sendAndLogError("HTTP server failed", err)
			cancel()
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		}
	}()
}

func (a *App) closeActiveConnection() {
	a.connMu.Lock()
	defer a.connMu.Unlock()

	if a.activeConn != nil {
		slog.Info("Closing active connection.")
		a.activeConn.Close()
		a.activeConn = nil
	}
}

func (a *App) setActiveConn(conn *webrtcPkg.ReceiverConn) {
	a.connMu.Lock()
	if a.activeConn != nil {
		slog.Warn("An active connection already exits. Closing it before creating a new one.")
		a.activeConn.Close()
	}
	a.activeConn = conn
	a.connMu.Unlock()

}
