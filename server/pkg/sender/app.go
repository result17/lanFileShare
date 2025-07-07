package sender

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/rescp17/lanFileSharer/api"
	"github.com/rescp17/lanFileSharer/pkg/concurrency"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// App is the main application logic controller for the sender.
// It manages state, coordinates services, and communicates with the UI.
type App struct {
	serviceID     string
	guard         *concurrency.ConcurrencyGuard
	discoverer    discovery.Adapter
	apiClient     *api.Client
	uiMessages    chan tea.Msg // Channel to send messages to the UI
	selectedFiles []*fileInfo.FileNode
}

// NewApp creates a new sender application instance.
func NewApp() *App {
	serviceID := uuid.New().String()
	return &App{
		serviceID:  serviceID,
		guard:      concurrency.NewConcurrencyGuard(),
		discoverer: &discovery.MDNSAdapter{},
		apiClient:  api.NewClient(serviceID), // Pass the serviceID to the client
		uiMessages: make(chan tea.Msg),
	}
}

// UIMessages returns the channel for the UI to listen on for updates.
func (a *App) UIMessages() <-chan tea.Msg {
	return a.uiMessages
}

// StartDiscovery begins the process of finding receivers on the network.
func (a *App) StartDiscovery() {
	go func() {
		serviceChan, err := a.discoverer.Discover(context.Background(), fmt.Sprintf("%s.%s.", discovery.DefaultServerType, discovery.DefaultDomain))
		if err != nil {
			a.uiMessages <- ErrorMsg{Err: fmt.Errorf("failed to start discovery: %w", err)}
			return
		}

		for services := range serviceChan {
			a.uiMessages <- FoundServicesMsg{Services: services}
		}
	}()
}

// SelectFiles sets the files that the user has chosen.
func (a *App) SelectFiles(files []*fileInfo.FileNode) {
	a.selectedFiles = files
}

// StartSendProcess is the main entry point for starting a file transfer.
// It is protected by a concurrency guard to prevent multiple simultaneous sends.
func (a *App) StartSendProcess(receiver discovery.ServiceInfo) {
	task := func() error {
		a.uiMessages <- TransferStartedMsg{}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		receiverURL := fmt.Sprintf("http://%s:%d", receiver.Addr.String(), receiver.Port)

		// 1. Send /ask request and wait for confirmation
		a.uiMessages <- StatusUpdateMsg{Message: "Waiting for receiver's confirmation..."}
		err := a.apiClient.SendAskRequest(ctx, receiverURL, a.selectedFiles)
		if err != nil {
			return fmt.Errorf("receiver did not accept transfer: %w", err)
		}

		// 2. Send the files (placeholder for now)
		a.uiMessages <- StatusUpdateMsg{Message: "Sending files..."}
		time.Sleep(2 * time.Second) // Simulate file transfer

		return nil // Success
	}

	go func() {
		err := a.guard.Execute(task)
		if err != nil {
			if err == concurrency.ErrBusy {
				a.uiMessages <- ErrorMsg{Err: fmt.Errorf("a transfer is already in progress")}
			} else {
				a.uiMessages <- ErrorMsg{Err: err}
			}
		} else {
			a.uiMessages <- TransferCompleteMsg{}
		}
	}()
}

// --- Custom tea.Msg types for UI communication ---

type FoundServicesMsg struct {
	Services []discovery.ServiceInfo
}

type StatusUpdateMsg struct {
	Message string
}

type TransferStartedMsg struct{}

type TransferCompleteMsg struct{}

type ErrorMsg struct {
	Err error
}
