package sender

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/rescp17/lanFileSharer/api"
	appevents "github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/internal/app_events/sender"
	"github.com/rescp17/lanFileSharer/pkg/concurrency"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/rescp17/lanFileSharer/pkg/transfer"
	webrtcPkg "github.com/rescp17/lanFileSharer/pkg/webrtc"
	"golang.org/x/sync/errgroup"
)

// App is the main application logic controller for the sender.
type App struct {
	serviceID       string
	guard           *concurrency.ConcurrencyGuard
	discoverer      discovery.Adapter
	apiClient       *api.Client
	uiMessages      chan tea.Msg            // App -> TUI
	appEvents       chan appevents.AppEvent // TUI -> App
	webrtcAPI       *webrtcPkg.WebrtcAPI
	transferTimeout time.Duration
	transferWG      sync.WaitGroup // Track active transfer goroutines

	// Note: Removed fileStructure field for stateless design
	// Each transfer will create its own FileStructureManager
}

// NewApp creates a new sender application instance.
func NewApp(adapter discovery.Adapter) *App {
	serviceID := uuid.New().String()
	webrtcAPI := webrtcPkg.NewWebrtcAPI()
	return &App{
		serviceID:       serviceID,
		guard:           concurrency.NewConcurrencyGuard(),
		discoverer:      adapter,
		apiClient:       api.NewClient(serviceID),
		uiMessages:      make(chan tea.Msg, 10),
		appEvents:       make(chan appevents.AppEvent),
		webrtcAPI:       webrtcAPI,
		transferTimeout: 2 * time.Minute,
	}
}

// UIMessages returns the channel for the UI to listen on for updates.
func (a *App) UIMessages() <-chan tea.Msg {
	return a.uiMessages
}

// AppEvents returns a write-only channel for the TUI to send events to the app.
func (a *App) AppEvents() chan<- appevents.AppEvent {
	return a.appEvents
}

// prepareFilesForTransfer creates a new FileStructureManager for a specific transfer
// This function is stateless and creates a fresh manager for each transfer
func (a *App) prepareFilesForTransfer(files []fileInfo.FileNode) (*transfer.FileStructureManager, error) {
	// Create a new FileStructureManager for this transfer
	fileStructure := transfer.NewFileStructureManager()

	// Add files to the new manager
	for _, file := range files {
		if err := fileStructure.AddFileNode(&file); err != nil {
			return nil, fmt.Errorf("failed to add file %s: %w", file.Path, err)
		}
	}

	slog.Info("Files prepared for sending",
		"fileCount", fileStructure.GetFileCount(),
		"dirCount", fileStructure.GetDirCount(),
		"totalSize", fileStructure.GetTotalSize())

	return fileStructure, nil
}

// Note: GetFileStructure method removed as part of stateless design
// Each transfer now creates its own FileStructureManager

// Run starts the application's main event loop.
func (a *App) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return a.runDiscovery(ctx)
	})

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				// Wait for any active transfers to complete gracefully
				a.transferWG.Wait()
				return nil
			case event, ok := <-a.appEvents:
				if !ok {
					slog.Info("App event channel closed")
					return nil
				}
				switch e := event.(type) {
				case sender.SendFilesMsg:
					// Show files to users and start the transfer process
					a.StartSendProcess(ctx, e.Receiver, e.Files)
				}
			}
		}
	})
	return g.Wait()
}

// runDiscovery begins the process of finding receivers on the network.
func (a *App) runDiscovery(ctx context.Context) error {
	// TODO: Use HTTPS for secure communication
	serviceChan := a.discoverer.Discover(ctx, fmt.Sprintf("%s.%s.", discovery.DefaultServerType, discovery.DefaultDomain))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-serviceChan:
			if result.Error != nil {
				a.sendAndLogError("Failed to discover service", result.Error)
				return result.Error
			}

			a.uiMessages <- sender.FoundServicesMsg{Services: result.Services}
		}
	}
}

// sendAndLogError is a helper function to both log an error and send it to the UI.
func (a *App) sendAndLogError(baseMessage string, err error) {
	slog.Error(baseMessage, "error", err)
	a.uiMessages <- appevents.Error{Err: fmt.Errorf("%s: %w", baseMessage, err)}
}

// StartSendProcess is the main entry point for starting a file transfer.
func (a *App) StartSendProcess(ctx context.Context, receiver discovery.ServiceInfo, files []fileInfo.FileNode) {
	task := func(taskCtx context.Context) error {
		// Create a new FileStructureManager for this transfer (stateless)
		fileStructure, err := a.prepareFilesForTransfer(files)
		if err != nil {
			return fmt.Errorf("failed to prepare files: %w", err)
		}
		// fileStructure will be garbage collected after this function returns

		// Use the shorter of the two timeouts: main context or transfer timeout
		transferCtx, cancel := context.WithTimeout(taskCtx, a.transferTimeout)
		defer cancel()

		a.uiMessages <- sender.TransferStartedMsg{}
		// TODO: Use HTTPS for secure communication
		receiverURL := fmt.Sprintf("http://%s", net.JoinHostPort(receiver.Addr.String(), fmt.Sprintf("%d", receiver.Port)))

		a.uiMessages <- sender.StatusUpdateMsg{Message: "Creating secure connection..."}

		config := webrtcPkg.Config{}
		webrtcConn, err := a.webrtcAPI.NewSenderConnection(transferCtx, config, a.apiClient, receiverURL)
		if err != nil {
			return fmt.Errorf("failed to create webrtc connection: %w", err)
		}
		defer func() {
			if err := webrtcConn.Close(); err != nil {
				slog.Error("Failed to close webrtc connection", "error", err)
			}
		}()

		a.uiMessages <- sender.StatusUpdateMsg{Message: "Establishing connection..."}
		if err := webrtcConn.Establish(transferCtx, fileStructure); err != nil {
			return fmt.Errorf("could not establish webrtc connection: %w", err)
		}

		a.uiMessages <- sender.StatusUpdateMsg{Message: "Connection established. Preparing to send files..."}

		transferFiles := fileStructure.GetAllFileEntities()

		if err := webrtcConn.SendFiles(transferCtx, transferFiles, a.serviceID); err != nil {
			return fmt.Errorf("failed to send files: %w", err)
		}

		return nil // Success
	}

	a.transferWG.Add(1)
	go func() {
		defer a.transferWG.Done()
		err := a.guard.ExecuteWithContext(ctx, task)
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
