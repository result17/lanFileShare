package sender

import (
	"context"
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/rescp17/lanFileSharer/api"
	"github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/internal/app_events/sender"
	"github.com/rescp17/lanFileSharer/pkg/concurrency"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/rescp17/lanFileSharer/pkg/webrtc"
)

// App is the main application logic controller for the sender.
// It manages state, coordinates services, and communicates with the UI.
type App struct {
	serviceID     string
	guard         *concurrency.ConcurrencyGuard
	discoverer    discovery.Adapter
	apiClient     *api.Client
	uiMessages    chan tea.Msg             // Channel to send messages to the UI
	appEvents     chan app_events.AppEvent // Channel to receive events from the UI
	selectedFiles []fileInfo.FileNode
	webrtcAPI     *webrtc.WebRTCAPI
	webrtcConn    *webrtc.Connection
}

// NewApp creates a new sender application instance.
func NewApp() *App {
	serviceID := uuid.New().String()
	webrtcAPI := webrtc.NewWebRTCAPI()
	return &App{
		serviceID:  serviceID,
		guard:      concurrency.NewConcurrencyGuard(),
		discoverer: &discovery.MDNSAdapter{},
		apiClient:  api.NewClient(serviceID), // Pass the serviceID to the client
		uiMessages: make(chan tea.Msg, 5),
		appEvents:  make(chan app_events.AppEvent),
		webrtcAPI:  webrtcAPI,
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
// It listens for events from the TUI and manages the application's lifecycle.
func (a *App) Run(ctx context.Context, cancel context.CancelFunc) {
	a.startDiscovery(ctx)

	for {
		select {
		case <-ctx.Done():
			// Context was cancelled, so we are shutting down.
			return
		case event := <-a.appEvents:
			// Process events from the TUI
			switch e := event.(type) {
			case sender.QuitAppMsg:
				// TUI requested to quit, cancel the context to trigger shutdown.
				cancel()
				return
			case sender.SendFilesMsg:
				// TUI requested to send files to a specific receiver.
				a.SelectFiles(e.Files)
				a.StartSendProcess(e.Receiver)
			}
		}
	}
}

// startDiscovery begins the process of finding receivers on the network.
func (a *App) startDiscovery(ctx context.Context) {
	go func() {
		serviceChan, err := a.discoverer.Discover(ctx, fmt.Sprintf("%s.%s.", discovery.DefaultServerType, discovery.DefaultDomain))
		if err != nil {
			err := fmt.Errorf("failed to start discovery: %w", err)
			log.Printf("[Discover discover]: %v", err)
			a.uiMessages <- sender.ErrorMsg{Err: err}
			return
		}

		// Ticker to periodically send updates to the UI, even if no new services are found.
		// This helps in observing when services disappear.
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		var currentServices []discovery.ServiceInfo

		for {
			select {
			case <-ctx.Done():
				return
			case services, ok := <-serviceChan:
				if !ok {
					// Channel closed, discovery stopped.
					return
				}
				currentServices = services
				a.uiMessages <- sender.FoundServicesMsg{Services: currentServices}
			case <-ticker.C:
				// Periodically send the current list to the UI.
				// This is crucial for noticing when a service disappears from the list.
				if currentServices != nil {
					a.uiMessages <- sender.FoundServicesMsg{Services: currentServices}
				}
			}
		}
	}()
}

// SelectFiles sets the files that the user has chosen.
func (a *App) SelectFiles(files []fileInfo.FileNode) {
	a.selectedFiles = files
}

// StartSendProcess is the main entry point for starting a file transfer.
// It is protected by a concurrency guard to prevent multiple simultaneous sends.
func (a *App) StartSendProcess(receiver discovery.ServiceInfo) {
	task := func() error {
		a.uiMessages <- sender.TransferStartedMsg{}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		receiverURL := fmt.Sprintf("http://%s:%d", receiver.Addr.String(), receiver.Port)
		a.apiClient.SetReceiverURL(receiverURL)

		a.uiMessages <- sender.StatusUpdateMsg{Message: "Waiting for receiver's confirmation..."}
		err := a.apiClient.SendAskRequest(ctx, a.selectedFiles)
		if err != nil {
			return fmt.Errorf("receiver did not accept transfer: %w", err)
		}

		signaler := api.NewAPISignaler(ctx, a.apiClient)
		config := webrtc.Config{
			Signaler: signaler,
		}
		webrtcConn, err := a.webrtcAPI.NewConnection(config)
		if err != nil {
			err := fmt.Errorf("failed to create webrtc connection: %w", err)
			log.Printf("[StartSendProcess] %w", err)
			return err
		}
		a.webrtcConn = webrtcConn
		defer a.webrtcConn.Close()

		a.uiMessages <- sender.StatusUpdateMsg{Message: "Sending files..."}
		if err := a.webrtcConn.Establish(ctx); err != nil {
			err := fmt.Errorf("could not establish webrtc connection: ")
			log.Printf("[StartSendProcess] %w", err)
			return err
		}
		a.uiMessages <- sender.StatusUpdateMsg{Message: "Connection established. Sending files..."}
		time.Sleep(2 * time.Second) // Simulate file transfer

		return nil // Success
	}

	go func() {
		err := a.guard.Execute(task)
		if err != nil {
			if err == concurrency.ErrBusy {
				a.uiMessages <- sender.ErrorMsg{Err: fmt.Errorf("a transfer is already in progress")}
			} else {
				a.uiMessages <- sender.ErrorMsg{Err: err}
			}
		} else {
			a.uiMessages <- sender.TransferCompleteMsg{}
		}
	}()
}
