package ui

import (
	"context"
	"errors"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	appevents "github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	receiverApp "github.com/rescp17/lanFileSharer/pkg/receiver"
	senderApp "github.com/rescp17/lanFileSharer/pkg/sender"
	"github.com/rescp17/lanFileSharer/pkg/ui/components"
)

// tickMsg is a message sent periodically to trigger UI updates.
type tickMsg time.Time

// tick is a command that sends a tickMsg after a specified duration.
func tick(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type Mode int

const (
	None Mode = iota
	Sender
	Receiver
	FatalError
)

type model struct {
	mode                 Mode
	appController        AppController
	sender               senderModel
	receiver             receiverModel
	ctx                  context.Context
	cancel               context.CancelFunc
	statusIndicator      *components.StatusIndicator // Global status indicator
	quickTip             *components.QuickTip        // Global quick tip
	themeManager         *components.ThemeManager
	themeSelector        *components.ThemeSelector
	performanceOptimizer *components.PerformanceOptimizer
	performancePanel     *components.PerformancePanel
	contextMenu          *components.ContextualMenu
	helpPanel            *components.HelpPanel        // Global Help Panel
	errorHandler         *components.ErrorHandler     // Global Error Handler
	retryDialog          *components.RetryDialog      // Global Retry Dialog
	statusBar            *components.StatusBar        // Global Status Bar
	breadcrumb           *components.Breadcrumb       // Global Breadcrumb
	keyboardManager      *components.KeyboardManager  // Global Keyboard Manager
	responsiveLayout     *components.ResponsiveLayout // Global Responsive Layout
}

func InitialModel(m Mode, port int, outputPath string) model {
	var appController AppController
	var sender senderModel
	var receiver receiverModel

	switch m {
	case Sender:
		appController = senderApp.NewApp(&discovery.MDNSAdapter{})
		sender = initSenderModel(appController)
	case Receiver:
		appController = receiverApp.NewApp(port, outputPath)
		receiver = initReceiverModel(port, appController)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// init global components
	themeManager := components.NewThemeManager("")
	keyboardManager := components.NewKeyboardManager()
	keyboardManager.SetContext("discovery")
	themeSelector := components.NewThemeSelector(themeManager)
	performanceOptimizer := components.NewPerformanceOptimizer()
	performancePanel := components.NewPerformancePanel(performanceOptimizer)
	contextMenu := components.NewContextualMenu("Actions")
	helpPanel := components.NewHelpPanelWithKeyboard(keyboardManager)
	quickTip := components.NewQuickTip()
	errorHandler := components.NewErrorHandler(3, true, 5*time.Second)
	retryDialog := components.NewRetryDialog(errorHandler)
	statusBar := components.NewStatusBar(80)
	breadcrumb := components.NewBreadcrumb(5)
	responsiveLayout := components.NewResponsiveLayout(themeManager)
	statusIndicator := components.NewStatusIndicator(5, true) // Keep 5 messages, show timestamps

	themeSelector.SetOnHidden(keyboardManager.RestoreLastContext)

	helpPanel.SetCompact(true)

	// start performance collection
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			performanceOptimizer.CollectMetrics()
		}
	}()

	return model{
		mode:                 m,
		appController:        appController,
		sender:               sender,
		receiver:             receiver,
		ctx:                  ctx,
		cancel:               cancel,
		statusIndicator:      statusIndicator,
		quickTip:             quickTip,
		themeManager:         themeManager,
		themeSelector:        themeSelector,
		performanceOptimizer: performanceOptimizer,
		performancePanel:     performancePanel,
		contextMenu:          contextMenu,
		helpPanel:            helpPanel,
		errorHandler:         errorHandler,
		retryDialog:          retryDialog,
		statusBar:            statusBar,
		breadcrumb:           breadcrumb,
		keyboardManager:      keyboardManager,
		responsiveLayout:     responsiveLayout,
	}
}

func (m model) Init() tea.Cmd {

	var initCmd tea.Cmd
	switch m.mode {
	case Sender:
		initCmd = m.initSender()
	case Receiver:
		initCmd = m.initReceiver()
	}

	runCmd := func() tea.Msg {
		if err := m.appController.Run(m.ctx); err != nil {
			slog.Error("App runtime error", "error", err)

			if errors.Is(err, context.Canceled) {
				return appevents.AppFinishedMsg{}
			}
			return appevents.Error{Err: err}
		}
		return appevents.AppFinishedMsg{}
	}

	// Start the global UI tick
	tickCmd := tick(time.Second)

	return tea.Batch(initCmd, runCmd, tickCmd)
}

func (m model) View() string {
	// Show theme selector if visible (overlay)
	if m.themeSelector.IsVisible() {
		return m.themeSelector.Render()
	}

	// Show performance panel if visible (overlay)
	if m.performancePanel.IsVisible() {
		return m.performancePanel.Render()
	}

	var s string

	if m.themeSelector != nil && m.themeSelector.IsVisible() {
		return m.themeSelector.Render()
	}
	if m.performancePanel != nil && m.performancePanel.IsVisible() {
		return m.performancePanel.Render()
	}

	switch m.mode {
	case Sender:
		s += m.senderView()
	case Receiver:
		s += m.receiverView()
	case FatalError:
		return m.errorHandler.Render()
	default:
		return ""
	}

	if m.contextMenu != nil && m.contextMenu.IsVisible() {
		s += "\n" + m.contextMenu.Render()
	}

	// Add help panel (context-sensitive help system)
	// Remove extra newlines to achieve compact layout with senderView
	helpContent := m.helpPanel.Render()
	if helpContent != "" {
		s += m.responsiveLayout.AdaptiveContainer(helpContent, "")
	}

	return s
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.QuitMsg:
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	case appevents.Error:
		m.renderFatalErr(msg.Err)
		return m, nil
	case appevents.AppFinishedMsg:
		return m, tea.Quit
	case tickMsg:
		return m, tick(time.Second)
	case tea.KeyMsg:
		// Process the key through the keyboard manager
		action := m.keyboardManager.ProcessKey(msg)
		if newModel, cmd, handled := m.handleGlobalAction(action); handled {
			return newModel, cmd
		}

	// Handle overlay components (theme selector, context menu, retry dialog)
	if newModel, cmd, handled := m.handleOverlayComponents(action); handled {
		return newModel, cmd
	}

	case tea.WindowSizeMsg:
		// Update responsive layout
		m.responsiveLayout.Update(msg)
		// Update status bar width
		m.statusBar.SetWidth(msg.Width)
	}

	// Handle mode-specific updates
	var modeCmd tea.Cmd
	switch m.mode {
	case Sender:
		var newModel tea.Model
		newModel, modeCmd = m.updateSender(msg)
		if newModel, ok := newModel.(model); ok {
			m = newModel
		}
	case Receiver:
		var newModel tea.Model
		newModel, modeCmd = m.updateReceiver(msg)
		if newModel, ok := newModel.(model); ok {
			m = newModel
		}
	}

	if modeCmd != nil {
		cmds = append(cmds, modeCmd)
	}

	return m, tea.Batch(cmds...)
}

// handleRefresh handles refresh actions
func (m *model) handleRefresh() tea.Cmd {
	switch m.mode {
	case Sender:
		switch m.sender.state {
		case findingReceivers:
			// Restart discovery
			return m.initSender()
		case selectingReceiver:
			// Refresh receiver list
			return m.initSender()
		default:
			// Show quick tip using global quickTip
			m.quickTip.Show("Refresh not available in current state", "info", 3)
			return nil
		}
	case Receiver:
		// TODO
		return nil
	}
	return nil
}

func (m model) handleGlobalAction(action components.KeyAction) (model, tea.Cmd, bool) {
	// Use keyboard manager to handle the action with better cohesion
	return m.handleAction(action)
}

// handleAction provides a more cohesive way to handle keyboard actions
func (m *model) handleAction(action components.KeyAction) (model, tea.Cmd, bool) {
	switch action {
	case components.KeyActionQuit:
		return *m, tea.Quit, true
	case components.KeyActionHelp:
		m.helpPanel.Toggle()
		return *m, nil, true
	case components.KeyActionRefresh:
		return *m, m.handleRefresh(), true
	case components.KeyActionTheme:
		if m.themeSelector != nil {
			m.themeSelector.Show()
			m.keyboardManager.SetContext("theme_selector")
		}
		return *m, nil, true
	case components.KeyActionShowPerformance:
		if m.sender.state != sendingFiles && m.sender.state != transferPaused {
			if m.performancePanel != nil {
				m.performancePanel.Show()
			}
			return *m, nil, true
		}
	}
	return *m, nil, false
}

// listenForAppMessages is a command that listens for messages from the app controller.
func (m *model) listenForAppMessages() tea.Cmd {
	return func() tea.Msg {
		return <-m.appController.UIMessages()
	}
}

// handleOverlayComponents handles all overlay components (theme selector, context menu, retry dialog)
func (m *model) handleOverlayComponents(action components.KeyAction) (tea.Model, tea.Cmd, bool) {
	// Handle theme selector if visible
	if m.themeSelector != nil && m.themeSelector.IsVisible() {
		if m.keyboardManager.GetContext() == "theme_selector" && m.themeSelector.Navigate(action) {
			return m, nil, true
		}
		// Theme selector is visible but didn't handle this action
		return m, nil, true
	}

	// Handle context menu if visible
	if m.contextMenu != nil && m.contextMenu.IsVisible() {
		if m.contextMenu.Navigate(action) {
			selectedItem := m.contextMenu.GetSelectedItem()
			if selectedItem != nil {
				return m, m.handleMenuAction(selectedItem.Action), true
			}
		}
		// Context menu is visible but didn't handle this action
		return m, nil, true
	}

	// Handle retry dialog if visible
	if m.retryDialog.IsVisible() {
		if m.handleRetryAction(action) {
			return m, nil, true
		}
		// Retry dialog is visible but didn't handle this action
		return m, nil, true
	}
	
	// No overlay components are visible, don't handle the action
	return m, nil, false
}

// handleRetryAction handles retry dialog actions with better cohesion
func (m *model) handleRetryAction(action components.KeyAction) bool {
	switch action {
	case components.KeyActionRetry:
		if m.errorHandler.CanRetry() {
			m.errorHandler.IncrementRetry()
			m.retryDialog.Hide()
			return true
		}
	case components.KeyActionCancel:
		m.retryDialog.Hide()
		return true
	}
	return false
}

func (m *model) renderFatalErr(err error) {
	m.errorHandler.AddError(components.ErrorTypeUnknown, "Application Error", err.Error(), true)
	m.mode = FatalError
}
