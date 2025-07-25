package sender

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/rescp17/lanFileSharer/api"
	"github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/internal/app_events/sender"
	"github.com/rescp17/lanFileSharer/pkg/concurrency"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	webrtcPkg "github.com/rescp17/lanFileSharer/pkg/webrtc"
)

// App is the main application logic controller for the sender.
type App struct {
	serviceID     string
	guard         *concurrency.ConcurrencyGuard
	discoverer    discovery.Adapter
	apiClient     *api.Client
	uiMessages    chan tea.Msg
	appEvents     chan app_events.AppEvent
	selectedFiles []fileInfo.FileNode
	webrtcAPI     *webrtcPkg.WebrtcAPI
	transferTimeout time.Duration
}

// NewApp creates a new sender application instance.
func NewApp() *App {
	serviceID := uuid.New().String()
	webrtcAPI := webrtcPkg.NewWebrtcAPI()
	return &App{
		serviceID:  serviceID,
		guard:      concurrency.NewConcurrencyGuard(),
		discoverer: &discovery.MDNSAdapter{},
		apiClient:  api.NewClient(serviceID),
		uiMessages: make(chan tea.Msg, 10),
		appEvents:  make(chan app_events.AppEvent),
		webrtcAPI:  webrtcAPI,
		transferTimeout: 2 * time.Minute,
	}
}

// UIMessages returns the channel for the UI to listen on for updates.
func (a *App) UIMessages() <-chan tea.Msg {
	return a.uiMessages
}

// AppEvents returns a write-only channel for the TUI to send events to the app.
func (a *App) AppEvents() chan<- app_events.AppEvent {
	return a.appEvents
}

// Run starts the application's main event loop.
func (a *App) Run(ctx context.Context, cancel context.CancelFunc) {
	a.startDiscovery(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-a.appEvents:
			switch e := event.(type) {
			case sender.QuitAppMsg:
				cancel()
				return
			case sender.SendFilesMsg:
				a.SelectFiles(e.Files)
				a.StartSendProcess(ctx, e.Receiver)
			}
		}
	}
}

// startDiscovery begins the process of finding receivers on the network.
func (a *App) startDiscovery(ctx context.Context) {
	go func() {
		serviceChan, err := a.discoverer.Discover(ctx, fmt.Sprintf("%s.%s.", discovery.DefaultServerType, discovery.DefaultDomain))
		if err != nil {
			a.sendAndLogError("Failed to start discovery", err)
			return
		}

		var currentServices []discovery.ServiceInfo

		for {
			select {
			case <-ctx.Done():
				return
			case services, ok := <-serviceChan:
				if !ok {
					return
				}
				currentServices = services
				a.uiMessages <- sender.FoundServicesMsg{Services: currentServices}

			}
		}
	}()
}

// sendAndLogError is a helper function to both log an error and send it to the UI.
func (a *App) sendAndLogError(baseMessage string, err error) {
	slog.Error(baseMessage, "error", err)
	a.uiMessages <- sender.ErrorMsg{Err: fmt.Errorf("%s: %w", baseMessage, err)}
}

// SelectFiles sets the files that the user has chosen.
func (a *App) SelectFiles(files []fileInfo.FileNode) {
	a.selectedFiles = files
}

// StartSendProcess is the main entry point for starting a file transfer.
func (a *App) StartSendProcess(ctx context.Context, receiver discovery.ServiceInfo) {
	task := func() error {
		a.uiMessages <- sender.TransferStartedMsg{}

		tctx, cancel := context.WithTimeout(ctx, a.transferTimeout)
		defer cancel()

		receiverURL := fmt.Sprintf("http://%s:%d", receiver.Addr.String(), receiver.Port)
		a.apiClient.SetReceiverURL(receiverURL)

		a.uiMessages <- sender.StatusUpdateMsg{Message: "Creating secure connection..."}

		config := webrtcPkg.Config{}
		webrtcConn, err := a.webrtcAPI.NewSenderConnection(tctx, config, a.apiClient)
		if err != nil {
			return fmt.Errorf("failed to create webrtc connection: %w", err)
		}
		defer webrtcConn.Close()

		a.uiMessages <- sender.StatusUpdateMsg{Message: "Establishing connection..."}
		if err := webrtcConn.Establish(tctx, a.selectedFiles); err != nil {
			return fmt.Errorf("could not establish webrtc connection: %w", err)
		}

		a.uiMessages <- sender.StatusUpdateMsg{Message: "Connection established. Preparing to send files..."}
		time.Sleep(2 * time.Second) // Simulate file transfer preparation

		return nil // Success
	}

	go func() {
		err := a.guard.Execute(task)
		if err != nil {
			if err == concurrency.ErrBusy {
				a.sendAndLogError("A transfer is already in progress", err)
			} else {
				a.sendAndLogError("Transfer failed", err)
			}
		} else {
			a.uiMessages <- sender.TransferCompleteMsg{}
		}
	}()
}
