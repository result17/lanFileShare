package ui

import (
	"context"
	"errors"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	appevents "github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/internal/style"
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
)

type model struct {
	mode                 Mode
	appController        AppController
	sender               senderModel
	receiver             receiverModel
	ctx                  context.Context
	cancel               context.CancelFunc
	err                  error
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
	themeSelector := components.NewThemeSelector(themeManager)
	performanceOptimizer := components.NewPerformanceOptimizer()
	performancePanel := components.NewPerformancePanel(performanceOptimizer)
	contextMenu := components.NewContextualMenu("Actions")
	helpPanel := components.NewHelpPanel()
	quickTip := components.NewQuickTip()
	errorHandler := components.NewErrorHandler(3, true, 5*time.Second)
	retryDialog := components.NewRetryDialog(errorHandler)
	statusBar := components.NewStatusBar(80)
	breadcrumb := components.NewBreadcrumb(5)
	keyboardManager := components.NewKeyboardManager()
	keyboardManager.SetContext("discovery")
	responsiveLayout := components.NewResponsiveLayout(themeManager)
	statusIndicator := components.NewStatusIndicator(5, true) // Keep 5 messages, show timestamps

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
	if m.err != nil {
		return style.ErrorStyle.Render(m.err.Error()) + "\n\nPress ctrl+c to quit."
	}

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
	default:
		return ""
	}
	
	if m.contextMenu != nil && m.contextMenu.IsVisible() {
		s += "\n" + m.contextMenu.Render() + "\n"
	}
	s += "\nPress ctrl + c to quit"
	return s
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.QuitMsg:
		// This is sent on Ctrl+C by default.
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	case appevents.Error:
		m.err = msg.Err
		return m, tea.Quit
	case appevents.AppFinishedMsg:
		return m, tea.Quit
	case tickMsg:
		// This is our global tick. We can update components that need periodic refresh here.
		// For example, the status bar time.
		if m.mode == Sender {
			m.sender.updateStatusBar()
		}
		// Always restart the tick.
		return m, tick(time.Second)
	}

	switch m.mode {
	case Sender:
		return m.updateSender(msg)
	case Receiver:
		return m.updateReceiver(msg)
	}

	return m, nil
}

func (m model) handleToggleHelpPanel() tea.Cmd {
	m.helpPanel.Toggle()
	return nil
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
	switch action {
	case components.KeyActionQuit:
		return m, tea.Quit, true
	case components.KeyActionHelp:
		return m, m.handleToggleHelpPanel(), true
	case components.KeyActionRefresh:
		return m, m.handleRefresh(), true
	default:
		return m, nil, false
	}
}
