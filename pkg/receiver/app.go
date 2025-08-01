package receiver

import (
	"context"
	"errors"
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
	appevents "github.com/rescp17/lanFileSharer/internal/app_events"
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
	appEvents            chan appevents.AppEvent
	stateManager         *app.SingleRequestManager
	inboundCandidateChan chan webrtc.ICECandidateInit
	activeConn           webrtcPkg.ReceiverConnection
	connMu               sync.Mutex
	errChan              chan error
}

// NewApp creates a new receiver application instance.
func NewApp(port int) *App {
	uiMessages := make(chan tea.Msg, 10)
	stateManager := app.NewSingleRequestManager()
	apiHandler := api.NewAPI(uiMessages, stateManager)

	dnssdlog.Info.SetOutput(io.Discard)
	dnssdlog.Debug.SetOutput(io.Discard)

	return &App{
		guard:                concurrency.NewConcurrencyGuard(),
		registrar:            &discovery.MDNSAdapter{},
		api:                  apiHandler,
		port:                 port,
		uiMessages:           uiMessages,
		appEvents:            make(chan appevents.AppEvent),
		stateManager:         stateManager,
		inboundCandidateChan: make(chan webrtc.ICECandidateInit, 10),
		errChan:              make(chan error, 1),
	}
}

// InboundCandidateChan provides a channel for the API layer to send candidates to the app logic.
func (a *App) InboundCandidateChan() chan<- webrtc.ICECandidateInit {
	return a.inboundCandidateChan
}

func (a *App) handleInboundCandidate(candidate webrtc.ICECandidateInit) error {
	a.connMu.Lock()
	defer a.connMu.Unlock()

	if a.activeConn != nil {
		if err := a.activeConn.Peer().AddICECandidate(candidate); err != nil {
			slog.Warn("Failed to add inbound ICE candidate", "error", err)
		}
	} else {
		/**
		 * f an ICE candidate is received after a connection has been closed or has failed
		 * (i.e., a.activeConn == nil), the application terminates. This could happen due to network latency where candidates from a stale session arrive late. This should be a non-fatal event.
		 */
		slog.Warn("Received an ICE candidate but there is no active connection.")
		return errors.New("received an ICE candidate but there is no active connection")
	}
	return nil
}

// Run starts the application's main event loop and services.
func (a *App) Run(ctx context.Context) error {
	tctx, cancel := context.WithCancel(ctx)
	defer cancel()
	a.startRegistration(tctx, a.port, cancel)
	a.startServer(tctx, a.port)

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-a.errChan:
			return fmt.Errorf("application service failed: %w", err)
		case candidate := <-a.inboundCandidateChan:
			if err := a.handleInboundCandidate(candidate); err != nil {
				slog.Warn("Failed to handle inbound ICE candidate", "error", err)
			}
		case event := <-a.appEvents:
			switch event.(type) {
			case receiver.FileRequestAccepted:
				go func() {
					if err := a.guard.Execute(func() error {
						return a.handleAcceptFileRequest(ctx)
					}); err != nil {
						slog.Error("File acceptance handler failed", "error", err)
						// DO NOT send the error to a.errChan, as this is a recoverable error.
						// select {
						// case a.errChan <- err:
						// default:
						//     slog.Error("Error channel full, dropping error", "error", err)
						// }
					}
				}()
			case receiver.FileRequestRejected:
				slog.Info("User rejected file transfer.")
				err := a.stateManager.SetDecision(app.Rejected)
				return fmt.Errorf("file request rejected: %w", err)
			default:
				slog.Warn("Received unhandled app event", "event", event)
			}
		}
	}
}

// sendAndLogError is a helper function to both log an error and send it to the UI.
func (a *App) sendAndLogError(baseMessage string, err error) {
	slog.Error(baseMessage, "error", err)
	a.uiMessages <- appevents.Error{Err: fmt.Errorf("%s: %w", baseMessage, err)}
}

// handleAcceptFileRequest contains the logic for setting up a WebRTC connection.
func (a *App) handleAcceptFileRequest(ctx context.Context) error {
	slog.Info("User accepted file transfer. Preparing to receive...")
	hctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := a.stateManager.SetDecision(app.Accepted); err != nil {
		a.sendAndLogError("Failed to set decision", err)
		return err
	}

	webrtcAPI := webrtcPkg.NewWebrtcAPI()

	offer, err := a.stateManager.GetOffer()
	if err != nil {
		a.sendAndLogError("Could not get offer from state", err)
		return err
	}

	receiverConn, err := webrtcAPI.NewReceiverConnection(webrtcPkg.Config{})
	if err != nil {
		a.sendAndLogError("Failed to create receiver connection", err)
		return err
	}

	receiverConn.Peer().OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog.Info("Peer Connection State has changed", "state", state.String())
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateDisconnected {
			slog.Info("Closing active connection due to state change.")

			// Only close and nil out the connection if it's the one this callback is for.
			a.closeActiveConnectionIfSameConn(receiverConn)
		}
	})

	receiverConn.Peer().OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			slog.Info("All local ICE candidates gathered.")
			a.stateManager.CloseCandidateChan()
			return
		}
		slog.Info("Got a local ICE candidate", "candidate", candidate.ToJSON().Candidate)
		if err := a.stateManager.SetCandidate(candidate.ToJSON()); err != nil {
			slog.Error("Failed to send ICE candidate", "error", err)
		}
	})

	answer, err := receiverConn.HandleOfferAndCreateAnswer(offer)
	if err != nil {
		a.sendAndLogError("Failed to create answer", err)
		return err
	}

	var success bool
	defer func() {
		if !success {
			slog.Warn("Closing receiver connection due to setup failure.")
			if err := receiverConn.Close(); err != nil {
				slog.Error("Failed to close receiver connection", "error", err)
			}
		}
	}()

	if err := hctx.Err(); err != nil {
		slog.Warn("Handshake cancelled or timed out before sending answer.", "error", err)
		return err
	}
	a.setActiveConn(receiverConn)
	if err := a.stateManager.SetAnswer(*answer); err != nil {
		a.sendAndLogError("Failed to send answer", err)
		return err
	}
	slog.Info("Answer created and sent to state manager.")
	success = true
	return nil
}

func (a *App) UIMessages() <-chan tea.Msg {
	return a.uiMessages
}

func (a *App) AppEvents() chan<- appevents.AppEvent {
	return a.appEvents
}

func (a *App) startRegistration(ctx context.Context, port int, cancel context.CancelFunc) {
	hostname, err := os.Hostname()
	if err != nil {
		a.sendAndLogError("Could not get hostname", err)
		cancel()
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
			a.errChan <- err
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
			a.sendAndLogError("HTTP server failed", err)
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

func (a *App) closeActiveConnectionIfSameConn(receiverConn webrtcPkg.ReceiverConnection) {
	a.connMu.Lock()
	defer a.connMu.Unlock()

	if a.activeConn != nil && a.activeConn == receiverConn {
		slog.Info("Closing active connection.")
		if err := a.activeConn.Close(); err != nil {
			slog.Error("Failed to close active connection", "error", err)
		}
		a.activeConn = nil
	}
}

func (a *App) setActiveConn(conn webrtcPkg.ReceiverConnection) {
	a.connMu.Lock()
	oldConn := a.activeConn
	a.activeConn = conn
	a.connMu.Unlock()
	if oldConn != nil {
		slog.Warn("An active connection already exits. Closing it before creating a new one.")
		if err := oldConn.Close(); err != nil {
			slog.Error("Failed to close old connection", "error", err)
		}
	}
}
