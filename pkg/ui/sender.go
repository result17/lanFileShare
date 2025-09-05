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
	appController    AppController
	state            senderState
	spinner          spinner.Model
	table            table.Model
	fp               multiFilePicker.Model
	services         []discovery.ServiceInfo
	selectedService  discovery.ServiceInfo
	statsCollector   *components.AdvancedStatsCollector
	realTimeStats    *components.RealTimeStatsPanel
	rateChart        *components.LineChart
	sparkLine        *components.SparkLine
	transferProgress *TransferProgress
	progressBar      *components.MultiFileProgress
	statsPanel       *components.TransferStatsPanel
	keyboardManager  *components.KeyboardManager
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

func initSenderModel(appController AppController) senderModel {
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
	statsPanel := components.NewTransferStatsPanel()

	// Initialize advanced statistics components
	statsCollector := components.NewAdvancedStatsCollector(100, time.Second) // Keep 100 points, update every second
	realTimeStats := components.NewRealTimeStatsPanel(statsCollector, time.Second)
	rateChart := components.NewLineChart("📈 Transfer Rate", 60, 10, 60) // 60 chars wide, 10 high, 60 points max
	rateChart.SetLabels("Time", "rate")
	sparkLine := components.NewSparkLine(40, 40) // 40 chars wide, 40 values max

	// Initialize keyboard manager for sender-specific actions
	keyboardManager := components.NewKeyboardManager()

	return senderModel{
		spinner:         s,
		fp:              multiFilePicker.InitialModel(),
		state:           findingReceivers,
		table:           t,
		progressBar:     progressBar,
		statsPanel:      statsPanel,
		statsCollector:  statsCollector,
		realTimeStats:   realTimeStats,
		rateChart:       rateChart,
		sparkLine:       sparkLine,
		keyboardManager: keyboardManager,
		appController:   appController,
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

func (m *senderModel) updateReceiverTable(services []discovery.ServiceInfo) {
	m.services = services

	rows := []table.Row{}
	for index, svc := range services {
		rows = append(rows, table.Row{
			strconv.Itoa(index), svc.Name, svc.Addr.String(), strconv.Itoa(svc.Port),
		})
	}
	m.table.SetRows(rows)
	m.table.SetHeight(len(rows) + 1)
	m.adjustTableCursor(len(rows))
}

func (m *model) updateSender(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.updateSenderByMsg(msg)

	var cmd tea.Cmd
	// Handle UI events first - this ensures multiFilePicker gets the message
	switch m.sender.state {
	case selectingFiles:
		cmd = m.sender.updateSelectingFilesState(msg)
	case selectingReceiver:
		cmd = m.sender.updateSelectingReceiverState(msg)
	case sendingFiles:
		cmd = m.sender.updateSendingFilesState(msg)
	case transferPaused:
		cmd = m.sender.updateTransferPausedState(msg)
	case transferComplete, transferFailed:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyEnter {
			m.sender.reset()
			return m, m.initSender()
		}
	}

	// Handle keyboard input through the keyboard manager for non-file-selection states
	if keyMsg, ok := msg.(tea.KeyMsg); ok && m.sender.state != selectingFiles {
		action := m.keyboardManager.ProcessKey(keyMsg)

		// Handle state-specific actions
		return m, m.handleStateSpecificAction(action)
	}

	if m.sender.state == findingReceivers {
		var spinCmd tea.Cmd
		m.sender.spinner, spinCmd = m.sender.spinner.Update(msg)

		return m, tea.Batch(cmd, spinCmd)
	}

	return m, cmd
}

func (m *model) updateSenderByMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	// spinner cmd
	if tickMsg, ok := msg.(spinner.TickMsg); ok {
		var spinCmd tea.Cmd
		m.sender.spinner, spinCmd = m.sender.spinner.Update(tickMsg)
		return m, spinCmd
	}

	// Handle window size changes for responsive layout
	if windowMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.responsiveLayout.Update(windowMsg)
		// Update status bar width
		m.statusBar.SetWidth(windowMsg.Width)
		return m, nil
	}

	// send file msg to senderApp
	if fileMsg, ok := msg.(multiFilePicker.SelectedFileNodeMsg); ok {
		m.appController.AppEvents() <- senderEvent.SendFilesMsg{
			Files: fileMsg.Files,
		}
		return m, nil
	}

	if ae, ok := msg.(appevents.AppEvent); ok {
		if cmd, processed := m.handleSenderAppEvent(ae); processed {
			return m, cmd
		}
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// user type file path here
		if m.sender.state == selectingFiles && !m.sender.fp.IsBrowseMode() {
			return m.sender.fp.Update(keyMsg)
		}

		action := m.keyboardManager.ProcessKey(keyMsg)

		
		// Handle statistics display mode switching
		switch action {
		case components.KeyActionStatsOverview:
			m.sender.realTimeStats.SetDisplayMode("overview")
			return m, nil

		case components.KeyActionStatsDetailed:
			m.sender.realTimeStats.SetDisplayMode("detailed")
			return m, nil
		case components.KeyActionStatsFiles:
			m.sender.realTimeStats.SetDisplayMode("files")
			return m, nil
		case components.KeyActionStatsNetwork:
			m.sender.realTimeStats.SetDisplayMode("network")
			return m, nil
		case components.KeyActionStatsEfficiency:
			m.sender.realTimeStats.SetDisplayMode("efficiency")
			return m, nil
		}


	}

	return m, nil
}

//nolint:gocyclo
func (m *model) handleSenderAppEvent(msg appevents.AppEvent) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case senderEvent.FoundServicesMsg:
		slog.Info("Discovery update", "service_count", len(msg.Services))
		for _, s := range msg.Services {
			slog.Debug("Found service", "name", s.Name, "addr", s.Addr, "port", s.Port)
		}

		if len(msg.Services) > 0 && m.sender.state == findingReceivers {
			m.sender.state = selectingReceiver
			m.helpPanel.SetContext(components.HelpContextSenderSelection)
			m.keyboardManager.SetContext("selection")
			m.breadcrumb.AddItem("Select Receiver", "selection", "📡", false)
		}
		// If the list of services becomes empty, go back to the finding state.
		if len(msg.Services) == 0 && m.sender.state == selectingReceiver {
			m.sender.state = findingReceivers
			m.helpPanel.SetContext(components.HelpContextSenderDiscovery)
			m.keyboardManager.SetContext("discovery")
			m.breadcrumb.PopItem()
		}

		m.sender.updateReceiverTable(msg.Services)
		return m.listenForAppMessages(), true // Continue listening
	case senderEvent.TransferStartedMsg:
		m.sender.state = waitingForReceiverConfirmation
		// 发送状态更新事件而不是直接更新状态指示器
		m.appController.AppEvents() <- senderEvent.StatusUpdateMsg{
			Message: "Transfer request sent, waiting for confirmation...",
		}
		return m.listenForAppMessages(), true
	case senderEvent.ReceiverAcceptedMsg:
		m.sender.state = sendingFiles
		m.helpPanel.SetContext(components.HelpContextTransfer)
		m.keyboardManager.SetContext("transfer")
		m.breadcrumb.AddItem("Transferring", "transfer", "🚀", false)
		m.statusIndicator.AddMessage(components.StatusSuccess, "Transfer accepted! Starting file transfer...")
		return m.listenForAppMessages(), true
	case senderEvent.StatusUpdateMsg:
		// Update status indicator with the message
		// Just log the message, status updates will be handled by main model
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

		// Update advanced statistics collector
		m.sender.statsCollector.UpdateTransferMetrics(msg.TotalBytes, msg.TransferredBytes, msg.TransferRate)

		// Update current file metrics if available
		if msg.CurrentFile != "" {
			// Estimate current file size and progress (this would ideally come from the transfer system)
			m.sender.statsCollector.UpdateFileMetrics(msg.CurrentFile, 0, 0, "active")
		}

		// Update rate chart
		m.sender.rateChart.AddPoint(float64(time.Now().Unix()), msg.TransferRate, "")

		// Update sparkline
		m.sender.sparkLine.AddValue(msg.TransferRate)

		return m.listenForAppMessages(), true
	case senderEvent.TransferCompleteMsg:
		m.sender.state = transferComplete
		m.statusIndicator.AddMessage(components.StatusSuccess, "Transfer completed successfully! 🎉")
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
		m.keyboardManager.SetContext("paused")
		m.statusIndicator.AddMessage(components.StatusWarning, "Transfer paused")
		return m.listenForAppMessages(), true
	case senderEvent.TransferResumedMsg:
		m.sender.state = sendingFiles
		m.keyboardManager.SetContext("transfer")
		m.statusIndicator.AddMessage(components.StatusInfo, "Transfer resumed")
		return m.listenForAppMessages(), true
	case senderEvent.TransferCancelledMsg:
		m.sender.state = transferFailed // Treat cancellation as failure for UI purposes
		m.statusIndicator.AddMessage(components.StatusWarning, "Transfer cancelled by user")
		return m.listenForAppMessages(), true
	case appevents.Error:
		m.err = msg.Err
		m.sender.state = transferFailed
		m.helpPanel.SetContext(components.HelpContextError)
		m.keyboardManager.SetContext("error")
		m.breadcrumb.AddItem("Error", "error", "❌", false)

		// Classify error type for better handling
		errorType := m.classifyError(msg.Err)
		m.errorHandler.AddError(errorType, "Transfer failed", msg.Err.Error(), true)

		// Add to status indicator as well
		m.statusIndicator.AddDetailedMessage(components.StatusError,
			"Transfer failed", msg.Err.Error(), "Press Enter to try again")

		// Error handling will be managed by the main model		return m.listenForAppMessages(), true
	}
	return nil, false
}

// updateSelectingReceiverState handles UI events for the selectingReceiver state.
func (m *senderModel) updateSelectingReceiverState(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		action := m.keyboardManager.ProcessKey(msg)
		switch action {
		case components.KeyActionSelect:
			if len(m.services) > 0 {
				selectedIndex := m.table.Cursor()
				if selectedIndex >= 0 && selectedIndex < len(m.services) {
					m.selectedService = m.services[selectedIndex]
					m.state = selectingFiles
				} else {
					// This case should ideally not be hit, but good to have for safety
					slog.Error("Cursor out of sync", "cursor", selectedIndex, "services_len", len(m.services))
				}
				_, cmd := m.table.Update(msg)
				return cmd
			}
		case components.KeyActionNavigateUp, components.KeyActionNavigateDown:
			_, cmd := m.table.Update(msg)
			return cmd
		}
	}
	return cmd
}

func (m *senderModel) updateSelectingFilesState(msg tea.Msg) tea.Cmd {
	// Update file picker
	newFpModel, fpCmd := m.fp.Update(msg)
	m.fp = newFpModel.(multiFilePicker.Model)

	switch msg := msg.(type) {
	case multiFilePicker.SelectedFileNodeMsg:
		// We can't handle this here - need to return it to main model
		// to access appController properly
		return tea.Cmd(func() tea.Msg {
			return msg // Pass the message back to main model
		})
	}

	// Return UI commands normally
	return fpCmd
}

func (m *model) senderView() string {
	var result strings.Builder
	// Breadcrumb navigation (if not empty and layout allows)
	if len(m.breadcrumb.GetItems()) > 0 && m.responsiveLayout.GetConfig().ShowBreadcrumb {
		result.WriteString(m.breadcrumb.Render())
		result.WriteString("\n\n")
	}

	// Main content based on state (wrapped in responsive container)
	var mainContent string
	switch m.sender.state {
	case findingReceivers:
		mainContent = fmt.Sprintf("\n%s Finding receivers...", m.sender.spinner.View())
	case selectingReceiver:
		mainContent = fmt.Sprintf("\n✔  Found %d receiver(s)\n", len(m.sender.services))
		mainContent += style.BaseStyle.Render(m.sender.table.View()) + "\n"
		if !m.responsiveLayout.IsCompactMode() {
			mainContent += "Use arrow keys to navigate, Enter to select."
		}
	case selectingFiles:
		receiverInfo := fmt.Sprintf("Receiver: %s", style.HighlightFontStyle.Render(m.sender.selectedService.Name))
		if m.responsiveLayout.IsCompactMode() {
			receiverInfo = m.responsiveLayout.TruncateText(receiverInfo)
		}
		mainContent = receiverInfo + "\n" + m.sender.fp.View() + "\n"
	case waitingForReceiverConfirmation:
		receiverName := m.sender.selectedService.Name
		if m.responsiveLayout.IsCompactMode() {
			receiverName = m.responsiveLayout.TruncateText(receiverName)
		}
		mainContent = fmt.Sprintf("\n%s Waiting for %s to confirm...",
			m.sender.spinner.View(),
			style.HighlightFontStyle.Render(receiverName))
	case sendingFiles:
		mainContent = m.renderTransferProgress()
	case transferPaused:
		mainContent = m.renderTransferPaused()
	case transferComplete:
		mainContent = m.renderTransferComplete()
	case transferFailed:
		mainContent = m.renderTransferFailed()
	default:
		mainContent = "Internal error: unknown sender state"
	}

	// Wrap main content in adaptive container
	result.WriteString(m.responsiveLayout.AdaptiveContainer(mainContent, ""))

	// Add enhanced UI components
	result.WriteString("\n")

	// Show context menu if visible
	if m.contextMenu != nil && m.contextMenu.IsVisible() {
		result.WriteString("\n")
		result.WriteString(m.contextMenu.Render())
		result.WriteString("\n")
	}

	// Show retry dialog if visible
	if m.retryDialog.IsVisible() {
		result.WriteString("\n")
		result.WriteString(m.retryDialog.Render())
		result.WriteString("\n")
	}

	// Show quick tip if visible
	if m.quickTip.IsVisible() {
		result.WriteString("\n")
		result.WriteString(m.quickTip.Render())
		result.WriteString("\n")
	}

	// Show help panel (if layout allows)
	if !m.responsiveLayout.IsCompactMode() {
		result.WriteString("\n")
		result.WriteString(m.helpPanel.Render())
	}

	// Status bar at the bottom (if layout allows)
	if m.responsiveLayout.GetConfig().ShowStatusBar {
		result.WriteString("\n")
		if m.statusBar != nil {
			result.WriteString(m.statusBar.Render())
		}
	}

	// Keyboard hints at the very bottom (always show but adapt to layout)
	result.WriteString("\n")
	hints := m.keyboardManager.RenderHints()
	if m.responsiveLayout.IsCompactMode() {
		// Truncate hints for compact mode
		maxWidth := m.responsiveLayout.GetContentWidth()
		hints = m.responsiveLayout.FormatText(hints, maxWidth)
	}
	result.WriteString(hints)

	// Add theme switch and performance hints
	if !m.responsiveLayout.IsCompactMode() {
		result.WriteString(" | T=Theme")
		if m.sender.state != sendingFiles && m.sender.state != transferPaused {
			result.WriteString(" | P=Performance")
		}
	}

	return result.String()
}

func (m *senderModel) reset() {
	*m = initSenderModel(m.appController)
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

	// Header with receiver info (adapt to layout)
	receiverName := m.sender.selectedService.Name
	if m.responsiveLayout.IsCompactMode() {
		receiverName = m.responsiveLayout.TruncateText(receiverName)
	}

	if m.responsiveLayout.ShouldShowIcons() {
		result.WriteString(fmt.Sprintf("\n%s Sending files to %s\n\n",
			m.sender.spinner.View(),
			style.HighlightFontStyle.Render(receiverName)))
	} else {
		result.WriteString(fmt.Sprintf("Sending to %s\n\n", receiverName))
	}

	// Enhanced progress display
	if m.sender.progressBar != nil {
		result.WriteString(m.sender.progressBar.Render())
		result.WriteString("\n")
	}

	// Real-time statistics panel (if layout allows details)
	if m.sender.realTimeStats != nil && m.responsiveLayout.ShouldShowDetails() {
		result.WriteString(m.sender.realTimeStats.Render())
		result.WriteString("\n")
	}

	// Transfer rate chart (compact sparkline)
	if m.sender.sparkLine != nil {
		if m.responsiveLayout.ShouldShowIcons() {
			result.WriteString("📈 Rate: ")
		} else {
			result.WriteString("Rate: ")
		}

		// Adjust sparkline width based on layout
		sparklineWidth := 40
		if m.responsiveLayout.IsCompactMode() {
			sparklineWidth = m.responsiveLayout.GetContentWidth() - 10
			if sparklineWidth < 10 {
				sparklineWidth = 10
			}
		}

		result.WriteString(m.sender.sparkLine.Render())
		if m.sender.transferProgress != nil {
			result.WriteString(fmt.Sprintf(" %s", formatRate(m.sender.transferProgress.TransferRate)))
		}
		result.WriteString("\n\n")
	}

	// Status messages will be handled by the main model

	// Control hints
	result.WriteString(style.FileStyle.Render("Controls: P=Pause | C=Cancel | 1-5=Stats Views | ?=Help"))

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
	if m.statusIndicator != nil {
		m.statusIndicator.SetCompact(true)
		statusMsg := m.statusIndicator.Render()
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
func (m *senderModel) updateSendingFilesState(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		action := m.keyboardManager.ProcessKey(keyMsg)
		switch action {
		case components.KeyActionPause:
			// Pause transfer
			return func() tea.Msg {
				return senderEvent.PauseTransferMsg{}
			}
		case components.KeyActionCancel:
			// Cancel transfer
			return func() tea.Msg {
				return senderEvent.CancelTransferMsg{}
			}
		case components.KeyActionQuit:
			// Quit application
			return tea.Quit
		}
	}
	return nil
}

// updateTransferPausedState handles UI events when transfer is paused
func (m *senderModel) updateTransferPausedState(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		action := m.keyboardManager.ProcessKey(keyMsg)
		switch action {
		case components.KeyActionResume:
			// Resume transfer
			return func() tea.Msg {
				return senderEvent.ResumeTransferMsg{}
			}
		case components.KeyActionCancel:
			// Cancel transfer
			return func() tea.Msg {
				return senderEvent.CancelTransferMsg{}
			}
		case components.KeyActionQuit:
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
	if m.statusIndicator != nil {
		m.statusIndicator.SetCompact(false)
		statusMsg := m.statusIndicator.Render()
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
	if m.statusIndicator != nil {
		m.statusIndicator.SetCompact(false)
		statusMsg := m.statusIndicator.Render()
		if statusMsg != "" {
			result.WriteString(statusMsg)
			result.WriteString("\n")
		}
	}

	// Fallback error message if no status indicator
	if m.err != nil && m.statusIndicator == nil {
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
	m.errorHandler.Clear()
	m.statusIndicator.AddMessage(components.StatusInfo, "Retrying operation...")

	// Depending on the current state, retry the appropriate operation
	switch m.sender.state {
	case transferFailed:
		// Reset to file selection state to allow user to retry
		m.sender.state = selectingFiles
		m.quickTip.Show("Select files again and press Tab to retry transfer", "info", 5)
		return nil
	case findingReceivers:
		// Retry discovery
		return m.initSender()
	}

	return nil
}

// formatRate formats transfer rate in a human-readable format
func formatRate(rate float64) string {
	if rate > 1024*1024*1024 {
		return fmt.Sprintf("%.1f GB/s", rate/(1024*1024*1024))
	} else if rate > 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", rate/(1024*1024))
	} else if rate > 1024 {
		return fmt.Sprintf("%.1f KB/s", rate/1024)
	}
	return fmt.Sprintf("%.0f B/s", rate)
}

// handleMenuAction handles context menu actions
func (m *model) handleMenuAction(action components.KeyAction) tea.Cmd {
	switch action {
	case components.KeyActionPause:
		return m.handlePauseResume()
	case components.KeyActionCancel:
		return m.handleCancel()
	case components.KeyActionRetry:
		return m.retryLastOperation()
	default:
		return nil
	}
}

// handleStateSpecificAction handles state-specific keyboard actions
func (m *model) handleStateSpecificAction(action components.KeyAction) tea.Cmd {
	switch m.sender.state {
	case findingReceivers:
		return m.handleDiscoveryAction(action)
	case selectingReceiver:
		return m.handleSelectionAction(action)
	case selectingFiles:
		return m.handleFileSelectionAction(action)
	case sendingFiles, transferPaused:
		return m.handleTransferAction(action)
	case transferFailed:
		return m.handleErrorAction(action)
	case transferComplete:
		return m.handleCompleteAction(action)
	default:
		return nil
	}
}

// handleDiscoveryAction handles actions during discovery phase
func (m *model) handleDiscoveryAction(action components.KeyAction) tea.Cmd {
	switch action {
	case components.KeyActionRefresh:
		return m.initSender()
	case components.KeyActionBack:
		// Go back to main menu (if implemented)
		return nil
	default:
		return nil
	}
}

// handleSelectionAction handles actions during receiver selection
func (m *model) handleSelectionAction(action components.KeyAction) tea.Cmd {
	switch action {
	case components.KeyActionNavigateUp, components.KeyActionNavigateDown:
		keyMsg := m.sender.keyboardManager.ProcessSpecAction(action)
		var cmd tea.Cmd
		m.sender.table, cmd = m.sender.table.Update(keyMsg)
		return cmd
	case components.KeyActionSelect:
		keyMsg := m.sender.keyboardManager.ProcessSpecAction(action)
		m.sender.updateSelectingReceiverState(keyMsg)
		m.appController.AppEvents() <- senderEvent.ReceiverSelectedMsg{}
		return nil
	case components.KeyActionBack:
		return m.initSender()
	default:
		return nil
	}
}

// handleFileSelectionAction handles actions during file selection
func (m *model) handleFileSelectionAction(action components.KeyAction) tea.Cmd {
	switch action {
	case components.KeyActionConfirm:
		// Confirm file selection and start transfer
		return nil
	case components.KeyActionBack:
		m.sender.state = selectingReceiver
		m.sender.keyboardManager.SetContext("selection")
		return nil
	default:
		return nil
	}
}

// handleTransferAction handles actions during transfer
func (m *model) handleTransferAction(action components.KeyAction) tea.Cmd {
	switch action {
	case components.KeyActionPause:
		return m.handlePauseResume()
	case components.KeyActionCancel:
		return m.handleCancel()
	default:
		return nil
	}
}

// handleErrorAction handles actions during error state
func (m *model) handleErrorAction(action components.KeyAction) tea.Cmd {
	switch action {
	case components.KeyActionRetry:
		return m.retryLastOperation()
	case components.KeyActionCancel:
		m.sender.state = selectingFiles
		m.sender.keyboardManager.SetContext("file_selection")
		return nil
	default:
		return nil
	}
}

// handleCompleteAction handles actions after transfer completion
func (m *model) handleCompleteAction(action components.KeyAction) tea.Cmd {
	switch action {
	case components.KeyActionConfirm:
		// Start new transfer
		m.sender.state = selectingFiles
		m.sender.keyboardManager.SetContext("file_selection")
		return nil
	case components.KeyActionBack:
		// Go back to main menu
		return nil
	default:
		return nil
	}
}

// handlePauseResume handles pause/resume actions
func (m *model) handlePauseResume() tea.Cmd {
	if m.sender.state == sendingFiles {
		// Pause transfer
		m.sender.state = transferPaused
		m.sender.keyboardManager.SetContext("paused")
		return func() tea.Msg {
			return senderEvent.PauseTransferMsg{}
		}
	} else if m.sender.state == transferPaused {
		// Resume transfer
		m.sender.state = sendingFiles
		m.sender.keyboardManager.SetContext("transfer")
		return func() tea.Msg {
			return senderEvent.ResumeTransferMsg{}
		}
	}
	return nil
}

// updateStatusBar updates the status bar with current information
func (s *senderModel) updateStatusBar() {
	// Clear previous items
	// This method is now a no-op. StatusBar is managed globally.
}

// handleCancel handles cancel actions
func (m *model) handleCancel() tea.Cmd {
	if m.sender.state == sendingFiles || m.sender.state == transferPaused {
		// Cancel transfer
		m.sender.state = transferFailed
		m.sender.keyboardManager.SetContext("error")
		return func() tea.Msg {
			return senderEvent.CancelTransferMsg{}
		}
	}
	return nil
}
