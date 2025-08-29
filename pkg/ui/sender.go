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
	"github.com/charmbracelet/lipgloss"
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

	// Advanced statistics components
	statsCollector *components.AdvancedStatsCollector
	realTimeStats  *components.RealTimeStatsPanel
	rateChart      *components.LineChart
	sparkLine      *components.SparkLine

	// Navigation and keyboard components
	keyboardManager *components.KeyboardManager
	breadcrumb      *components.Breadcrumb
	statusBar       *components.StatusBar
	contextMenu     *components.ContextualMenu

	// Theme and layout components
	themeManager     *components.ThemeManager
	responsiveLayout *components.ResponsiveLayout
	themeSelector    *components.ThemeSelector

	// Performance optimization components
	performanceOptimizer *components.PerformanceOptimizer
	performancePanel     *components.PerformancePanel

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

	// Initialize advanced statistics components
	statsCollector := components.NewAdvancedStatsCollector(100, time.Second) // Keep 100 points, update every second
	realTimeStats := components.NewRealTimeStatsPanel(statsCollector, time.Second)
	rateChart := components.NewLineChart("📈 Transfer Rate", 60, 10, 60) // 60 chars wide, 10 high, 60 points max
	rateChart.SetLabels("Time", "rate")
	sparkLine := components.NewSparkLine(40, 40) // 40 chars wide, 40 values max

	// Initialize navigation and keyboard components
	keyboardManager := components.NewKeyboardManager()
	keyboardManager.SetContext("discovery")   // Start with discovery context
	breadcrumb := components.NewBreadcrumb(5) // Keep up to 5 breadcrumb items
	statusBar := components.NewStatusBar(80)  // 80 characters wide
	contextMenu := components.NewContextualMenu("Actions")

	// Initialize theme and layout components
	themeManager := components.NewThemeManager("") // No config dir for now
	responsiveLayout := components.NewResponsiveLayout(themeManager)
	themeSelector := components.NewThemeSelector(themeManager)

	// Initialize performance optimization components
	performanceOptimizer := components.NewPerformanceOptimizer()
	performancePanel := components.NewPerformancePanel(performanceOptimizer)

	// Start performance monitoring in background
	go func() {
		ticker := time.NewTicker(5 * time.Second) // Collect metrics every 5 seconds
		defer ticker.Stop()

		for range ticker.C {
			performanceOptimizer.CollectMetrics()
		}
	}()

	return senderModel{
		spinner:              s,
		fp:                   multiFilePicker.InitialModel(),
		state:                findingReceivers,
		table:                t,
		progressBar:          progressBar,
		statusIndicator:      statusIndicator,
		statsPanel:           statsPanel,
		errorHandler:         errorHandler,
		helpPanel:            helpPanel,
		quickTip:             quickTip,
		retryDialog:          retryDialog,
		statsCollector:       statsCollector,
	realTimeStats:        realTimeStats,
	rateChart:            rateChart,
	sparkLine:            sparkLine,
		keyboardManager:      keyboardManager,
		breadcrumb:           breadcrumb,
		statusBar:            statusBar,
		contextMenu:          contextMenu,
		themeManager:         themeManager,
		responsiveLayout:     responsiveLayout,
		themeSelector:        themeSelector,
		performanceOptimizer: performanceOptimizer,
		performancePanel:     performancePanel,
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
	// Handle window size changes for responsive layout
	if windowMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.sender.responsiveLayout.Update(windowMsg)
		// Update status bar width
		m.sender.statusBar.SetWidth(windowMsg.Width)
		return m, nil
	}

	if cmd, processed := m.handleSenderAppEvent(msg); processed {
		return m, cmd
	}

	// Handle keyboard input through the keyboard manager
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		action := m.sender.keyboardManager.ProcessKey(keyMsg)

		// Handle global actions first
		switch action {
		case components.KeyActionQuit:
			return m, tea.Quit
		case components.KeyActionHelp:
			m.sender.helpPanel.Toggle()
			return m, nil
		case components.KeyActionRefresh:
			return m, m.handleRefresh()
		}

		// Handle theme selector if visible
		if m.sender.themeSelector.IsVisible() {
			if m.sender.themeSelector.Navigate(action) {
				return m, nil
			}
			return m, nil
		}

		// Handle performance panel if visible
		if m.sender.performancePanel.IsVisible() {
			if m.sender.performancePanel.Navigate(action) {
				return m, nil
			}
			return m, nil
		}

		// Handle context menu if visible
		if m.sender.contextMenu.IsVisible() {
			if m.sender.contextMenu.Navigate(action) {
				selectedItem := m.sender.contextMenu.GetSelectedItem()
				if selectedItem != nil {
					return m, m.handleMenuAction(selectedItem.Action)
				}
			}
			return m, nil
		}

		// Handle retry dialog if visible
		if m.sender.retryDialog.IsVisible() {
			switch action {
			case components.KeyActionRetry:
				if m.sender.errorHandler.CanRetry() {
					m.sender.errorHandler.IncrementRetry()
					m.sender.retryDialog.Hide()
					return m, m.retryLastOperation()
				}
			case components.KeyActionCancel:
				m.sender.retryDialog.Hide()
				return m, nil
			}
			return m, nil
		}

		// Handle theme switching (T key)
		if keyMsg.String() == "t" || keyMsg.String() == "T" {
			m.sender.themeSelector.Show()
			return m, nil
		}

		// Handle performance panel (P key when not in transfer)
		if (keyMsg.String() == "p" || keyMsg.String() == "P") &&
			m.sender.state != sendingFiles && m.sender.state != transferPaused {
			m.sender.performancePanel.Show()
			return m, nil
		}

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

		// Handle state-specific actions
		return m, m.handleStateSpecificAction(action)
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

	if m.sender.state == findingReceivers {
		var spinCmd tea.Cmd
		m.sender.spinner, spinCmd = m.sender.spinner.Update(msg)

		return m, tea.Batch(cmd, spinCmd)
	}

	return m, cmd
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
			m.sender.keyboardManager.SetContext("selection")
			m.sender.breadcrumb.AddItem("Select Receiver", "selection", "📡", false)
		}
		// If the list of services becomes empty, go back to the finding state.
		if len(msg.Services) == 0 && m.sender.state == selectingReceiver {
			m.sender.state = findingReceivers
			m.sender.helpPanel.SetContext(components.HelpContextSenderDiscovery)
			m.sender.keyboardManager.SetContext("discovery")
			m.sender.breadcrumb.PopItem()
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
		m.sender.keyboardManager.SetContext("transfer")
		m.sender.breadcrumb.AddItem("Transferring", "transfer", "🚀", false)
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
		m.sender.keyboardManager.SetContext("paused")
		m.sender.statusIndicator.AddMessage(components.StatusWarning, "Transfer paused")
		return m.listenForAppMessages(), true
	case senderEvent.TransferResumedMsg:
		m.sender.state = sendingFiles
		m.sender.keyboardManager.SetContext("transfer")
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
		m.sender.keyboardManager.SetContext("error")
		m.sender.breadcrumb.AddItem("Error", "error", "❌", false)

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
	// m.sender.table, cmd = m.sender.table.Update(msg)
	return cmd
}

func (m *model) updateSelectingFilesState(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case multiFilePicker.SelectedFileNodeMsg:
		// The app will now send messages about the transfer progress
		m.appController.AppEvents() <- senderEvent.SendFilesMsg{
			Files: msg.Files,
		}
	}
	newFpModel, cmd := m.sender.fp.Update(msg)
	m.sender.fp = newFpModel.(multiFilePicker.Model)
	return cmd
}

func (m *model) senderView() string {
	var result strings.Builder

	// Show theme selector if visible (overlay)
	if m.sender.themeSelector.IsVisible() {
		return m.sender.themeSelector.Render()
	}

	// Show performance panel if visible (overlay)
	if m.sender.performancePanel.IsVisible() {
		return m.sender.performancePanel.Render()
	}

	// Breadcrumb navigation (if not empty and layout allows)
	if len(m.sender.breadcrumb.GetItems()) > 0 && m.sender.responsiveLayout.GetConfig().ShowBreadcrumb {
		result.WriteString(m.sender.breadcrumb.Render())
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
		if !m.sender.responsiveLayout.IsCompactMode() {
			mainContent += "Use arrow keys to navigate, Enter to select."
		}
	case selectingFiles:
		receiverInfo := fmt.Sprintf("Receiver: %s", style.HighlightFontStyle.Render(m.sender.selectedService.Name))
		if m.sender.responsiveLayout.IsCompactMode() {
			receiverInfo = m.sender.responsiveLayout.TruncateText(receiverInfo)
		}
		mainContent = receiverInfo + "\n" + m.sender.fp.View() + "\n"
	case waitingForReceiverConfirmation:
		receiverName := m.sender.selectedService.Name
		if m.sender.responsiveLayout.IsCompactMode() {
			receiverName = m.sender.responsiveLayout.TruncateText(receiverName)
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
	result.WriteString(m.sender.responsiveLayout.AdaptiveContainer(mainContent, ""))

	// Add enhanced UI components
	result.WriteString("\n")

	// Show context menu if visible
	if m.sender.contextMenu.IsVisible() {
		result.WriteString("\n")
		result.WriteString(m.sender.contextMenu.Render())
		result.WriteString("\n")
	}

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

	// Show help panel (if layout allows)
	if !m.sender.responsiveLayout.IsCompactMode() {
		result.WriteString("\n")
		result.WriteString(m.sender.helpPanel.Render())
	}

	// Status bar at the bottom (if layout allows)
	if m.sender.responsiveLayout.GetConfig().ShowStatusBar {
		result.WriteString("\n")
		m.sender.updateStatusBar()
		result.WriteString(m.sender.statusBar.Render())
	}

	// Keyboard hints at the very bottom (always show but adapt to layout)
	result.WriteString("\n")
	hints := m.sender.keyboardManager.RenderHints()
	if m.sender.responsiveLayout.IsCompactMode() {
		// Truncate hints for compact mode
		maxWidth := m.sender.responsiveLayout.GetContentWidth()
		hints = m.sender.responsiveLayout.FormatText(hints, maxWidth)
	}
	result.WriteString(hints)

	// Add theme switch and performance hints
	if !m.sender.responsiveLayout.IsCompactMode() {
		result.WriteString(" | T=Theme")
		if m.sender.state != sendingFiles && m.sender.state != transferPaused {
			result.WriteString(" | P=Performance")
		}
	}

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

	// Header with receiver info (adapt to layout)
	receiverName := m.sender.selectedService.Name
	if m.sender.responsiveLayout.IsCompactMode() {
		receiverName = m.sender.responsiveLayout.TruncateText(receiverName)
	}

	if m.sender.responsiveLayout.ShouldShowIcons() {
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
	if m.sender.realTimeStats != nil && m.sender.responsiveLayout.ShouldShowDetails() {
		result.WriteString(m.sender.realTimeStats.Render())
		result.WriteString("\n")
	}

	// Transfer rate chart (compact sparkline)
	if m.sender.sparkLine != nil {
		if m.sender.responsiveLayout.ShouldShowIcons() {
			result.WriteString("📈 Rate: ")
		} else {
			result.WriteString("Rate: ")
		}

		// Adjust sparkline width based on layout
		sparklineWidth := 40
		if m.sender.responsiveLayout.IsCompactMode() {
			sparklineWidth = m.sender.responsiveLayout.GetContentWidth() - 10
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

	// Status messages (show latest, compact in small layouts)
	if m.sender.statusIndicator != nil {
		m.sender.statusIndicator.SetCompact(m.sender.responsiveLayout.IsCompactMode())
		statusMsg := m.sender.statusIndicator.Render()
		if statusMsg != "" {
			result.WriteString(statusMsg)
			result.WriteString("\n\n")
		}
	}

	// Control hints (adapt to layout)
	if m.sender.responsiveLayout.IsCompactMode() {
		result.WriteString(style.FileStyle.Render("P=Pause | C=Cancel"))
	} else {
		result.WriteString(style.FileStyle.Render("Controls: P=Pause | C=Cancel | 1-5=Stats Views | ?=Help"))
	}

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

// handleRefresh handles refresh actions
func (m *model) handleRefresh() tea.Cmd {
	switch m.sender.state {
	case findingReceivers:
		// Restart discovery
		return m.initSender()
	case selectingReceiver:
		// Refresh receiver list
		return m.initSender()
	default:
		// For other states, just show a quick tip
		m.sender.quickTip.Show("Refresh not available in current state", "info", 3)
		return nil
	}
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
		m.updateSelectingReceiverState(keyMsg)
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
	s.statusBar.Clear()

	// Left side - current state and progress
	switch s.state {
	case findingReceivers:
		s.statusBar.AddLeftItem("Discovering...", "🔍", style.FileStyle)
	case selectingReceiver:
		s.statusBar.AddLeftItem(fmt.Sprintf("%d receivers found", len(s.services)), "📡", style.FileStyle)
	case selectingFiles:
		s.statusBar.AddLeftItem("Select files", "📁", style.FileStyle)
	case waitingForReceiverConfirmation:
		s.statusBar.AddLeftItem("Waiting for confirmation", "⏳", style.FileStyle)
	case sendingFiles:
		if s.transferProgress != nil {
			progress := fmt.Sprintf("%.1f%%", s.transferProgress.OverallProgress)
			s.statusBar.AddLeftItem(progress, "🚀", style.SuccessStyle)
		} else {
			s.statusBar.AddLeftItem("Transferring", "🚀", style.FileStyle)
		}
	case transferPaused:
		s.statusBar.AddLeftItem("Paused", "⏸️", lipgloss.NewStyle().Foreground(lipgloss.Color("214")))
	case transferComplete:
		s.statusBar.AddLeftItem("Complete", "✅", style.SuccessStyle)
	case transferFailed:
		s.statusBar.AddLeftItem("Failed", "❌", style.ErrorStyle)
	}

	// Center - current file or receiver info
	if s.state == sendingFiles && s.transferProgress != nil && s.transferProgress.CurrentFile != "" {
		filename := s.transferProgress.CurrentFile
		if len(filename) > 30 {
			filename = filename[:27] + "..."
		}
		s.statusBar.AddCenterItem(filename, "📄", style.FileStyle)
	} else if s.selectedService.Name != "" {
		receiverName := s.selectedService.Name
		if len(receiverName) > 20 {
			receiverName = receiverName[:17] + "..."
		}
		s.statusBar.AddCenterItem(receiverName, "📡", style.HighlightFontStyle)
	}

	// Right side - transfer rate or time
	if s.state == sendingFiles && s.transferProgress != nil {
		rate := formatRate(s.transferProgress.TransferRate)
		s.statusBar.AddRightItem(rate, "⚡", style.FileStyle)
	} else {
		// Show current time
		currentTime := time.Now().Format("15:04:05")
		s.statusBar.AddRightItem(currentTime, "🕐", style.FileStyle)
	}
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
