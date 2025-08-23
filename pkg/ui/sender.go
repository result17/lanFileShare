package ui

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	appevents "github.com/rescp17/lanFileSharer/internal/app_events"
	senderEvent "github.com/rescp17/lanFileSharer/internal/app_events/sender"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/multiFilePicker"
	"github.com/rescp17/lanFileSharer/pkg/ui/components"
)

// senderState defines the different states of the sender UI.
type senderState int

const (
	findingReceivers senderState = iota
	selectingReceiver
	selectingFiles
	waitingForReceiverConfirmation
	sendingFiles
	transferPaused
	transferComplete
	transferFailed
)

type senderModel struct {
	state           senderState
	spinner         spinner.Model
	table           table.Model
	fp              multiFilePicker.Model
	services        []discovery.ServiceInfo
	selectedService discovery.ServiceInfo

	// Enhanced UI components
	progressBar     *components.MultiFileProgress
	statusIndicator *components.StatusIndicator
	statsPanel      *components.TransferStatsPanel
	errorHandler    *components.ErrorHandler
	helpPanel       *components.HelpPanel
	quickTip        *components.QuickTip
	retryDialog     *components.RetryDialog

	// Transfer progress tracking (legacy - will be replaced)
	transferProgress *TransferProgress
}

// TransferProgress tracks the overall transfer progress
type TransferProgress struct {
	TotalFiles       int
	CompletedFiles   int
	TotalBytes       int64
	TransferredBytes int64
	CurrentFile      string
	TransferRate     float64 // bytes per second
	ETA              string  // estimated time remaining
	OverallProgress  float64 // percentage 0-100
}

var columns = []table.Column{
	{Title: "Index", Width: 10},
	{Title: "Name", Width: 20},
	{Title: "Address", Width: 20},
	{Title: "Port", Width: 10},
}

func initSenderModel() senderModel {
	s := style.NewSpinner()

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(0),
	)

	t.SetStyles(style.NewTableStyles())

	// Initialize enhanced UI components
	progressConfig := components.DefaultProgressConfig()
	progressBar := components.NewMultiFileProgress(progressConfig)
	statusIndicator := components.NewStatusIndicator(5, true) // Keep 5 messages, show timestamps
	statsPanel := components.NewTransferStatsPanel()
	errorHandler := components.NewErrorHandler(3, true, 5*time.Second) // Keep 3 errors, auto-retry with 5s delay
	helpPanel := components.NewHelpPanel()
	quickTip := components.NewQuickTip()
	retryDialog := components.NewRetryDialog(errorHandler)

	return senderModel{
		spinner:         s,
		fp:              multiFilePicker.InitialModel(),
		state:           findingReceivers,
		table:           t,
		progressBar:     progressBar,
		statusIndicator: statusIndicator,
		statsPanel:      statsPanel,
		errorHandler:    errorHandler,
		helpPanel:       helpPanel,
		quickTip:        quickTip,
		retryDialog:     retryDialog,
	}
}

// listenForAppMessages is a command that listens for messages from the app controller.
func (m *model) listenForAppMessages() tea.Cmd {
	return func() tea.Msg {
		return <-m.appController.UIMessages()
	}
}

func (m *model) initSender() tea.Cmd {
	return tea.Batch(m.sender.spinner.Tick, m.listenForAppMessages())
}

func (m *model) updateReceiverTable(services []discovery.ServiceInfo) {
	m.sender.services = services
	rows := []table.Row{}
	for index, svc := range services {
		rows = append(rows, table.Row{
			strconv.Itoa(index), svc.Name, svc.Addr.String(), strconv.Itoa(svc.Port),
		})
	}
	m.sender.table.SetRows(rows)
	m.sender.table.SetHeight(len(rows) + 1)
	m.sender.adjustTableCursor(len(rows))
}

func (m *model) updateSender(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cmd, processed := m.handleSenderAppEvent(msg); processed {
		return m, cmd
	}

	// Handle global keyboard shortcuts first
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Handle help toggle
		if keyMsg.String() == "?" {
			m.sender.helpPanel.Toggle()
			return m, nil
		}

		// Handle retry dialog if visible
		if m.sender.retryDialog.IsVisible() {
			switch keyMsg.String() {
			case "r", "R":
				if m.sender.errorHandler.CanRetry() {
					m.sender.errorHandler.IncrementRetry()
					m.sender.retryDialog.Hide()
					// Trigger retry logic here
					return m, m.retryLastOperation()
				}
			case "c", "C", "esc":
				m.sender.retryDialog.Hide()
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	// Handle UI events
	switch m.sender.state {
	case selectingReceiver:
		cmd = m.updateSelectingReceiverState(msg)
	case selectingFiles:
		cmd = m.updateSelectingFilesState(msg)
	case sendingFiles:
		cmd = m.updateSendingFilesState(msg)
	case transferPaused:
		cmd = m.updateTransferPausedState(msg)
	case transferComplete, transferFailed:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyEnter {
			m.sender.reset()
			return m, m.initSender()
		}
	}

	var spinCmd tea.Cmd
	m.sender.spinner, spinCmd = m.sender.spinner.Update(msg)

	return m, tea.Batch(cmd, spinCmd)
}

//nolint:gocyclo
func (m *model) handleSenderAppEvent(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case senderEvent.FoundServicesMsg:
		slog.Info("Discovery update", "service_count", len(msg.Services))
		for _, s := range msg.Services {
			slog.Debug("Found service", "name", s.Name, "addr", s.Addr, "port", s.Port)
		}

		if len(msg.Services) > 0 && m.sender.state == findingReceivers {
			m.sender.state = selectingReceiver
			m.sender.helpPanel.SetContext(components.HelpContextSenderSelection)
		}
		// If the list of services becomes empty, go back to the finding state.
		if len(msg.Services) == 0 && m.sender.state == selectingReceiver {
			m.sender.state = findingReceivers
			m.sender.helpPanel.SetContext(components.HelpContextSenderDiscovery)
		}

		m.updateReceiverTable(msg.Services)
		return m.listenForAppMessages(), true // Continue listening
	case senderEvent.TransferStartedMsg:
		m.sender.state = waitingForReceiverConfirmation
		m.sender.statusIndicator.AddMessage(components.StatusInfo, "Transfer request sent, waiting for confirmation...")
		return m.listenForAppMessages(), true
	case senderEvent.ReceiverAcceptedMsg:
		m.sender.state = sendingFiles
		m.sender.helpPanel.SetContext(components.HelpContextTransfer)
		m.sender.statusIndicator.AddMessage(components.StatusSuccess, "Transfer accepted! Starting file transfer...")
		return m.listenForAppMessages(), true
	case senderEvent.StatusUpdateMsg:
		// Update status indicator with the message
		m.sender.statusIndicator.AddMessage(components.StatusInfo, msg.Message)
		slog.Info("Status Update", "message", msg.Message)
		return m.listenForAppMessages(), true
	case senderEvent.ProgressUpdateMsg:
		// Update legacy transfer progress for backward compatibility
		m.sender.transferProgress = &TransferProgress{
			TotalFiles:       msg.TotalFiles,
			CompletedFiles:   msg.CompletedFiles,
			TotalBytes:       msg.TotalBytes,
			TransferredBytes: msg.TransferredBytes,
			CurrentFile:      msg.CurrentFile,
			TransferRate:     msg.TransferRate,
			ETA:              msg.ETA,
			OverallProgress:  msg.OverallProgress,
		}

		// Update enhanced UI components
		overallProgress := components.ProgressData{
			Current:     msg.TransferredBytes,
			Total:       msg.TotalBytes,
			Rate:        msg.TransferRate,
			ETA:         time.Duration(0), // Convert from string if needed
			Label:       "Overall Progress",
			Status:      "active",
			CurrentFile: msg.CurrentFile,
		}
		m.sender.progressBar.UpdateOverall(overallProgress)

		// Update statistics panel
		m.sender.statsPanel.Update(
			msg.TotalFiles, msg.CompletedFiles, 0, // failedFiles
			msg.TotalBytes, msg.TransferredBytes,
			msg.TransferRate, msg.TransferRate, msg.TransferRate, // current, average, peak rates
		)

		return m.listenForAppMessages(), true
	case senderEvent.TransferCompleteMsg:
		m.sender.state = transferComplete
		m.sender.statusIndicator.AddMessage(components.StatusSuccess, "Transfer completed successfully! 🎉")
		// Update progress bar to complete status
		if m.sender.transferProgress != nil {
			completeProgress := components.ProgressData{
				Current: m.sender.transferProgress.TotalBytes,
				Total:   m.sender.transferProgress.TotalBytes,
				Status:  "complete",
				Label:   "Transfer Complete",
			}
			m.sender.progressBar.UpdateOverall(completeProgress)
		}
		return m.listenForAppMessages(), true
	case senderEvent.TransferPausedMsg:
		m.sender.state = transferPaused
		m.sender.statusIndicator.AddMessage(components.StatusWarning, "Transfer paused")
		return m.listenForAppMessages(), true
	case senderEvent.TransferResumedMsg:
		m.sender.state = sendingFiles
		m.sender.statusIndicator.AddMessage(components.StatusInfo, "Transfer resumed")
		return m.listenForAppMessages(), true
	case senderEvent.TransferCancelledMsg:
		m.sender.state = transferFailed // Treat cancellation as failure for UI purposes
		m.sender.statusIndicator.AddMessage(components.StatusWarning, "Transfer cancelled by user")
		return m.listenForAppMessages(), true
	case appevents.Error:
		m.err = msg.Err
		m.sender.state = transferFailed
		m.sender.helpPanel.SetContext(components.HelpContextError)

		// Classify error type for better handling
		errorType := m.classifyError(msg.Err)
		m.sender.errorHandler.AddError(errorType, "Transfer failed", msg.Err.Error(), true)

		// Add to status indicator as well
		m.sender.statusIndicator.AddDetailedMessage(components.StatusError,
			"Transfer failed", msg.Err.Error(), "Press Enter to try again")

		// Show retry dialog if error is recoverable
		if m.sender.errorHandler.CanRetry() {
			m.sender.retryDialog.Show(m.sender.errorHandler.ShouldAutoRetry(), 5)
		}

		return m.listenForAppMessages(), true
	}
	return nil, false
}

// updateSelectingReceiverState handles UI events for the selectingReceiver state.
func (m *model) updateSelectingReceiverState(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	// ... logic for key presses and table updates
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if len(m.sender.services) > 0 {
				selectedIndex := m.sender.table.Cursor()
				if selectedIndex >= 0 && selectedIndex < len(m.sender.services) {
					m.err = nil // Reset any previous error
					m.sender.selectedService = m.sender.services[selectedIndex]
					m.sender.state = selectingFiles
				} else {
					// This case should ideally not be hit, but good to have for safety
					err := fmt.Errorf("internal error: cursor %d is out of sync with services list (len %d)", selectedIndex, len(m.sender.services))
					slog.Error("Cursor out of sync", "error", err)
					m.err = err
				}
				_, cmd := m.sender.table.Update(msg)
				return cmd
			}
		}
	}
	// Update the table on every message to handle navigation
	m.sender.table, cmd = m.sender.table.Update(msg)
	return cmd
}

func (m *model) updateSelectingFilesState(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case multiFilePicker.SelectedFileNodeMsg:
		// The app will now send messages about the transfer progress
		m.appController.AppEvents() <- senderEvent.SendFilesMsg{
			Receiver: m.sender.selectedService,
			Files:    msg.Files,
		}
	}
	newFpModel, cmd := m.sender.fp.Update(msg)
	m.sender.fp = newFpModel.(multiFilePicker.Model)
	return cmd
}

func (m *model) senderView() string {
	var result strings.Builder

	// Main content based on state
	switch m.sender.state {
	case findingReceivers:
		result.WriteString(fmt.Sprintf("\n%s Finding receivers...", m.sender.spinner.View()))
	case selectingReceiver:
		result.WriteString(fmt.Sprintf("\n✔  Found %d receiver(s)\n", len(m.sender.services)))
		result.WriteString(style.BaseStyle.Render(m.sender.table.View()) + "\n")
		result.WriteString("Use arrow keys to navigate, Enter to select.")
	case selectingFiles:
		result.WriteString(fmt.Sprintf("Receiver: %s\n%s\n",
			style.HighlightFontStyle.Render(m.sender.selectedService.Name),
			m.sender.fp.View()))
	case waitingForReceiverConfirmation:
		result.WriteString(fmt.Sprintf("\n%s Waiting for %s to confirm...",
			m.sender.spinner.View(),
			style.HighlightFontStyle.Render(m.sender.selectedService.Name)))
	case sendingFiles:
		result.WriteString(m.renderTransferProgress())
	case transferPaused:
		result.WriteString(m.renderTransferPaused())
	case transferComplete:
		result.WriteString(m.renderTransferComplete())
	case transferFailed:
		result.WriteString(m.renderTransferFailed())
	default:
		result.WriteString("Internal error: unknown sender state")
	}

	// Add enhanced UI components
	result.WriteString("\n")

	// Show retry dialog if visible
	if m.sender.retryDialog.IsVisible() {
		result.WriteString("\n")
		result.WriteString(m.sender.retryDialog.Render())
		result.WriteString("\n")
	}

	// Show quick tip if visible
	if m.sender.quickTip.IsVisible() {
		result.WriteString("\n")
		result.WriteString(m.sender.quickTip.Render())
		result.WriteString("\n")
	}

	// Show help panel
	result.WriteString("\n")
	result.WriteString(m.sender.helpPanel.Render())

	return result.String()
}

func (m *senderModel) reset() {
	*m = initSenderModel()
}

func (m *senderModel) adjustTableCursor(newRowCount int) {
	if newRowCount <= 0 {
		m.table.SetCursor(0)
		return
	}

	currentCursor := m.table.Cursor()
	if currentCursor >= newRowCount {
		newCursor := newRowCount - 1
		slog.Debug("Adjusting table cursor due to service list shrink",
			"old_cursor", currentCursor,
			"new_cursor", newCursor,
			"row_count", newRowCount)

		m.table.SetCursor(newCursor)
	}
}

// renderTransferProgress renders the enhanced transfer progress display
func (m *model) renderTransferProgress() string {
	var result strings.Builder

	// Header with receiver info
	result.WriteString(fmt.Sprintf("\n%s Sending files to %s\n\n",
		m.sender.spinner.View(),
		style.HighlightFontStyle.Render(m.sender.selectedService.Name)))

	// Enhanced progress display
	if m.sender.progressBar != nil {
		result.WriteString(m.sender.progressBar.Render())
		result.WriteString("\n")
	}

	// Transfer statistics panel (compact mode)
	if m.sender.statsPanel != nil {
		m.sender.statsPanel.SetCompact(true)
		result.WriteString(m.sender.statsPanel.Render())
		result.WriteString("\n\n")
	}

	// Status messages (show latest)
	if m.sender.statusIndicator != nil {
		m.sender.statusIndicator.SetCompact(true)
		statusMsg := m.sender.statusIndicator.Render()
		if statusMsg != "" {
			result.WriteString(statusMsg)
			result.WriteString("\n\n")
		}
	}

	// Control hints
	result.WriteString(style.FileStyle.Render("Controls: P=Pause | C=Cancel | Ctrl+C=Quit"))

	return result.String()
}

// renderTransferPaused renders the enhanced paused transfer display
func (m *model) renderTransferPaused() string {
	var result strings.Builder

	// Header with pause indicator
	result.WriteString(fmt.Sprintf("\n⏸️  Transfer paused to %s\n\n",
		style.HighlightFontStyle.Render(m.sender.selectedService.Name)))

	// Enhanced progress display with paused status
	if m.sender.progressBar != nil {
		// Update progress bar status to paused
		if m.sender.transferProgress != nil {
			pausedProgress := components.ProgressData{
				Current: m.sender.transferProgress.TransferredBytes,
				Total:   m.sender.transferProgress.TotalBytes,
				Status:  "paused",
				Label:   "Transfer Paused",
			}
			m.sender.progressBar.UpdateOverall(pausedProgress)
		}
		result.WriteString(m.sender.progressBar.Render())
		result.WriteString("\n")
	}

	// Transfer statistics (compact mode)
	if m.sender.statsPanel != nil {
		m.sender.statsPanel.SetCompact(true)
		result.WriteString(m.sender.statsPanel.Render())
		result.WriteString("\n\n")
	}

	// Status messages
	if m.sender.statusIndicator != nil {
		m.sender.statusIndicator.SetCompact(true)
		statusMsg := m.sender.statusIndicator.Render()
		if statusMsg != "" {
			result.WriteString(statusMsg)
			result.WriteString("\n\n")
		}
	}

	// Control hints for paused state
	result.WriteString(style.FileStyle.Render("Controls: R/Space=Resume | C=Cancel | Ctrl+C=Quit"))

	return result.String()
}

// updateSendingFilesState handles UI events during file transfer
func (m *model) updateSendingFilesState(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "p", "P":
			// Pause transfer
			return func() tea.Msg {
				return senderEvent.PauseTransferMsg{}
			}
		case "c", "C":
			// Cancel transfer
			return func() tea.Msg {
				return senderEvent.CancelTransferMsg{}
			}
		case "q", "ctrl+c":
			// Quit application
			return tea.Quit
		}
	}
	return nil
}

// updateTransferPausedState handles UI events when transfer is paused
func (m *model) updateTransferPausedState(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "r", "R", " ":
			// Resume transfer
			return func() tea.Msg {
				return senderEvent.ResumeTransferMsg{}
			}
		case "c", "C":
			// Cancel transfer
			return func() tea.Msg {
				return senderEvent.CancelTransferMsg{}
			}
		case "q", "ctrl+c":
			// Quit application
			return tea.Quit
		}
	}
	return nil
}

// renderTransferComplete renders the enhanced transfer completion display
func (m *model) renderTransferComplete() string {
	var result strings.Builder

	// Success header
	result.WriteString(fmt.Sprintf("\n✅ Transfer completed successfully to %s!\n\n",
		style.HighlightFontStyle.Render(m.sender.selectedService.Name)))

	// Final progress display (complete status)
	if m.sender.progressBar != nil {
		result.WriteString(m.sender.progressBar.Render())
		result.WriteString("\n")
	}

	// Final transfer statistics (full mode for completion summary)
	if m.sender.statsPanel != nil {
		m.sender.statsPanel.SetCompact(false)
		result.WriteString(m.sender.statsPanel.Render())
		result.WriteString("\n")
	}

	// Success status message
	if m.sender.statusIndicator != nil {
		m.sender.statusIndicator.SetCompact(false)
		statusMsg := m.sender.statusIndicator.Render()
		if statusMsg != "" {
			result.WriteString(statusMsg)
			result.WriteString("\n")
		}
	}

	// Control hints for completion
	result.WriteString(style.FileStyle.Render("Controls: Enter=Send More Files | Q=Quit"))

	return result.String()
}

// renderTransferFailed renders the enhanced transfer failure display
func (m *model) renderTransferFailed() string {
	var result strings.Builder

	// Error header
	result.WriteString("\n❌ Transfer Failed\n\n")

	// Show current progress if available
	if m.sender.progressBar != nil {
		// Update progress bar status to error
		if m.sender.transferProgress != nil {
			errorProgress := components.ProgressData{
				Current: m.sender.transferProgress.TransferredBytes,
				Total:   m.sender.transferProgress.TotalBytes,
				Status:  "error",
				Label:   "Transfer Failed",
			}
			m.sender.progressBar.UpdateOverall(errorProgress)
		}
		result.WriteString(m.sender.progressBar.Render())
		result.WriteString("\n")
	}

	// Error status messages (full mode to show details)
	if m.sender.statusIndicator != nil {
		m.sender.statusIndicator.SetCompact(false)
		statusMsg := m.sender.statusIndicator.Render()
		if statusMsg != "" {
			result.WriteString(statusMsg)
			result.WriteString("\n")
		}
	}

	// Fallback error message if no status indicator
	if m.err != nil && m.sender.statusIndicator == nil {
		result.WriteString(fmt.Sprintf("Error: %s\n\n", style.ErrorStyle.Render(m.err.Error())))
	}

	// Control hints for failure
	result.WriteString(style.FileStyle.Render("Controls: Enter=Try Again | Q=Quit"))

	return result.String()
}

// classifyError classifies an error into a specific error type for better handling
func (m *model) classifyError(err error) components.ErrorType {
	if err == nil {
		return components.ErrorTypeUnknown
	}

	errStr := strings.ToLower(err.Error())

	// Network-related errors
	if strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "dial") ||
		strings.Contains(errStr, "refused") ||
		strings.Contains(errStr, "unreachable") {
		return components.ErrorTypeNetwork
	}

	// Timeout errors
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline") ||
		strings.Contains(errStr, "context canceled") {
		return components.ErrorTypeTimeout
	}

	// File system errors
	if strings.Contains(errStr, "file") ||
		strings.Contains(errStr, "directory") ||
		strings.Contains(errStr, "no such") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "exists") {
		return components.ErrorTypeFileSystem
	}

	// Permission errors
	if strings.Contains(errStr, "permission") ||
		strings.Contains(errStr, "denied") ||
		strings.Contains(errStr, "access") ||
		strings.Contains(errStr, "forbidden") {
		return components.ErrorTypePermission
	}

	// User cancellation
	if strings.Contains(errStr, "cancel") ||
		strings.Contains(errStr, "abort") ||
		strings.Contains(errStr, "interrupt") {
		return components.ErrorTypeUserCancelled
	}

	return components.ErrorTypeUnknown
}

// retryLastOperation attempts to retry the last failed operation
func (m *model) retryLastOperation() tea.Cmd {
	// Clear previous errors
	m.sender.errorHandler.Clear()
	m.sender.statusIndicator.AddMessage(components.StatusInfo, "Retrying operation...")

	// Depending on the current state, retry the appropriate operation
	switch m.sender.state {
	case transferFailed:
		// Reset to file selection state to allow user to retry
		m.sender.state = selectingFiles
		m.sender.quickTip.Show("Select files again and press Tab to retry transfer", "info", 5)
		return nil
	case findingReceivers:
		// Retry discovery
		return m.initSender()
	}

	return nil
}
