package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rescp17/lanFileSharer/internal/style"
)

// HelpContext represents different contexts where help can be shown
type HelpContext int

const (
	HelpContextMain HelpContext = iota
	HelpContextSenderDiscovery
	HelpContextSenderSelection
	HelpContextFileSelection
	HelpContextTransfer
	HelpContextReceiver
	HelpContextError
)

// HelpItem represents a single help item
type HelpItem struct {
	Action      KeyAction
	Description string
	Important   bool
}

// HelpPanel provides context-sensitive help and keyboard shortcuts
type HelpPanel struct {
	context        HelpContext
	visible        bool
	compact        bool
	customItems    []HelpItem
	keyboardManager *KeyboardManager // Reference to keyboard manager
}

// NewHelpPanel creates a new help panel
func NewHelpPanel() *HelpPanel {
	return &HelpPanel{
		context:     HelpContextMain,
		visible:     false,
		compact:     true,
		customItems: make([]HelpItem, 0),
	}
}

// NewHelpPanelWithKeyboard creates a new help panel with keyboard manager reference
func NewHelpPanelWithKeyboard(km *KeyboardManager) *HelpPanel {
	return &HelpPanel{
		context:        HelpContextMain,
		visible:        false,
		compact:        true,
		customItems:    make([]HelpItem, 0),
		keyboardManager: km,
	}
}

// SetKeyboardManager sets the keyboard manager reference
func (hp *HelpPanel) SetKeyboardManager(km *KeyboardManager) {
	hp.keyboardManager = km
}

// SetContext sets the current help context
func (hp *HelpPanel) SetContext(context HelpContext) {
	hp.context = context
}

// SetVisible sets the visibility of the help panel
func (hp *HelpPanel) SetVisible(visible bool) {
	hp.visible = visible
}

// SetCompact sets whether to use compact display mode
func (hp *HelpPanel) SetCompact(compact bool) {
	hp.compact = compact
}

// AddCustomItem adds a custom help item
func (hp *HelpPanel) AddCustomItem(action KeyAction, description string, important bool) {
	hp.customItems = append(hp.customItems, HelpItem{
		Action:      action,
		Description: description,
		Important:   important,
	})
}

// ClearCustomItems clears all custom help items
func (hp *HelpPanel) ClearCustomItems() {
	hp.customItems = hp.customItems[:0]
}

// Toggle toggles the visibility of the help panel
func (hp *HelpPanel) Toggle() {
	hp.visible = !hp.visible
}

// IsVisible returns whether the help panel is visible
func (hp *HelpPanel) IsVisible() bool {
	return hp.visible
}

// Render renders the help panel
func (hp *HelpPanel) Render() string {
	if !hp.visible {
		return hp.renderCompactHint()
	}

	if hp.compact {
		return hp.renderCompact()
	}
	return hp.renderFull()
}

// renderCompactHint renders a small hint when help is not visible
func (hp *HelpPanel) renderCompactHint() string {
	return style.FileStyle.Render("❓ Press '?' for help")
}

// renderCompact renders a compact help display
func (hp *HelpPanel) renderCompact() string {
	items := hp.getHelpItems()
	if len(items) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString("💡 ")

	// Show only the most important items in compact mode
	importantItems := make([]HelpItem, 0)
	for _, item := range items {
		if item.Important {
			importantItems = append(importantItems, item)
		}
	}

	if len(importantItems) == 0 {
		// If no important items, show first few items
		for i, item := range items {
			if i >= 3 {
				break
			}
			importantItems = append(importantItems, item)
		}
	}

	for i, item := range importantItems {
		if i > 0 {
			result.WriteString(" | ")
		}
		keys := hp.getActionKeys(item.Action)
		keyDisplay := hp.formatKeyDisplay(keys)
		result.WriteString(fmt.Sprintf("%s=%s",
			style.HighlightFontStyle.Render(keyDisplay),
			item.Description))
	}

	result.WriteString(" | ?=Help")
	return style.FileStyle.Render(result.String())
}

// renderFull renders the full help panel
func (hp *HelpPanel) renderFull() string {
	items := hp.getHelpItems()
	if len(items) == 0 {
		return ""
	}

	var result strings.Builder

	// Header
	result.WriteString("┌─────────────────────────────────────────────────────────────────────────────────┐\n")
	result.WriteString("│ 💡 Help & Keyboard Shortcuts\n")
	result.WriteString("├─────────────────────────────────────────────────────────────────────────────────┤\n")

	// Context-specific title
	contextTitle := hp.getContextTitle()
	if contextTitle != "" {
		result.WriteString(fmt.Sprintf("│ %s\n", style.HeaderStyle.Render(contextTitle)))
		result.WriteString("├─────────────────────────────────────────────────────────────────────────────────┤\n")
	}

	// Help items
	for _, item := range items {
		keyStyle := style.HighlightFontStyle
		if item.Important {
			keyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
		}

		keys := hp.getActionKeys(item.Action)
		keyDisplay := hp.formatKeyDisplay(keys)
		result.WriteString(fmt.Sprintf("│ %s %s\n",
			keyStyle.Render(fmt.Sprintf("%-12s", keyDisplay)),
			item.Description))
	}

	// Footer
	result.WriteString("├─────────────────────────────────────────────────────────────────────────────────┤\n")
	result.WriteString("│ Press '?' again to close help\n")
	result.WriteString("└─────────────────────────────────────────────────────────────────────────────────┘")

	return result.String()
}

// getContextTitle returns the title for the current context
func (hp *HelpPanel) getContextTitle() string {
	switch hp.context {
	case HelpContextSenderDiscovery:
		return "🔍 Finding Receivers"
	case HelpContextSenderSelection:
		return "📡 Select Receiver"
	case HelpContextFileSelection:
		return "📁 Select Files to Send"
	case HelpContextTransfer:
		return "🚀 File Transfer in Progress"
	case HelpContextReceiver:
		return "📥 Receiving Files"
	case HelpContextError:
		return "❌ Error Recovery"
	default:
		return "🏠 General"
	}
}

// getHelpItems returns the help items for the current context
func (hp *HelpPanel) getHelpItems() []HelpItem {
	var items []HelpItem

	// Use keyboard manager if available to get dynamic help items
	if hp.keyboardManager != nil {
		availableActions := hp.keyboardManager.GetAvailableActions()
		for _, action := range availableActions {
			// Filter actions based on context for better relevance
			if hp.isActionRelevantToContext(action) {
				description := hp.keyboardManager.GetActionDescription(action)
				important := hp.keyboardManager.IsActionImportant(action)
				items = append(items, HelpItem{
					Action:      action,
					Description: description,
					Important:   important,
				})
			}
		}
	} else {
		// Fallback to static items if no keyboard manager
		items = hp.getStaticHelpItems()
	}

	// Add custom items
	items = append(items, hp.customItems...)

	return items
}

// getStaticHelpItems returns static help items as fallback
func (hp *HelpPanel) getStaticHelpItems() []HelpItem {
	// Add context-specific items based on KeyAction
	switch hp.context {
	case HelpContextSenderDiscovery:
		return []HelpItem{
			{KeyActionQuit, "🚪 Quit application", true},
			{KeyActionRefresh, "🔄 Refresh/restart discovery", false},
			{KeyActionHelp, "Toggle help", false},
		}

	case HelpContextSenderSelection:
		return []HelpItem{
			{KeyActionNavigateUp, "⬆️⬇️ Navigate receivers", true},
			{KeyActionSelect, "✅ Select receiver", true},
			{KeyActionRefresh, "🔄 Refresh receiver list", false},
			{KeyActionQuit, "🚪 Quit application", false},
			{KeyActionHelp, "Toggle help", false},
		}

	case HelpContextFileSelection:
		return []HelpItem{
			{KeyActionNavigateUp, "⬆️⬇️ Navigate files/folders", true},
			{KeyActionSelect, "✅ Select/deselect file", true},
			{KeyActionSelect, "✅ Select/deselect file", true},
			{KeyActionNavigateRight, "➡️ Enter folder", false},
			{KeyActionNavigateLeft, "⬅️ Go back", false},
			{KeyActionConfirm, "📋 Confirm selection", true},
			{KeyActionBack, "❌ Cancel", false},
			{KeyActionHelp, "Toggle help", false},
		}

	case HelpContextTransfer:
		return []HelpItem{
			{KeyActionPause, "⏸️ Pause transfer", true},
			{KeyActionResume, "▶️ Resume transfer (if paused)", true},
			{KeyActionCancel, "❌ Cancel transfer", true},
			{KeyActionStatsOverview, "📊 Overview statistics", false},
			{KeyActionStatsDetailed, "📈 Detailed statistics", false},
			{KeyActionStatsFiles, "📁 File statistics", false},
			{KeyActionStatsNetwork, "🌐 Network statistics", false},
			{KeyActionStatsEfficiency, "⚡ Efficiency metrics", false},
			{KeyActionQuit, "🚪 Quit application", false},
			{KeyActionHelp, "Toggle help", false},
		}

	case HelpContextReceiver:
		return []HelpItem{
			{KeyActionSelect, "✅ Accept incoming transfer", true},
			{KeyActionCancel, "❌ Reject incoming transfer", true},
			{KeyActionQuit, "🚪 Quit application", false},
			{KeyActionHelp, "Toggle help", false},
		}

	case HelpContextError:
		return []HelpItem{
			{KeyActionRetry, "🔄 Retry operation", true},
			{KeyActionRetry, "🔄 Try again", true},
			{KeyActionCancel, "❌ Cancel", false},
			{KeyActionQuit, "🚪 Quit application", false},
			{KeyActionHelp, "Toggle help", false},
		}

	default: // HelpContextMain
		return []HelpItem{
			{KeyActionQuit, "🚪 Quit application", true},
			{KeyActionHelp, "Toggle help", false},
		}
	}
}

// isActionRelevantToContext checks if an action is relevant to the current context
func (hp *HelpPanel) isActionRelevantToContext(action KeyAction) bool {
	// Global actions are always relevant
	globalActions := []KeyAction{
		KeyActionQuit,
		KeyActionHelp,
		KeyActionTheme,
		KeyActionShowPerformance,
		KeyActionFullscreen,
	}
	
	for _, globalAction := range globalActions {
		if action == globalAction {
			return true
		}
	}
	
	// Context-specific relevance
	switch hp.context {
	case HelpContextSenderDiscovery:
		return action == KeyActionRefresh
		
	case HelpContextSenderSelection:
		return action == KeyActionNavigateUp || action == KeyActionNavigateDown || 
		       action == KeyActionSelect || action == KeyActionRefresh
		       
	case HelpContextFileSelection:
		return action == KeyActionNavigateUp || action == KeyActionNavigateDown ||
		       action == KeyActionNavigateLeft || action == KeyActionNavigateRight ||
		       action == KeyActionSelect || action == KeyActionConfirm || action == KeyActionBack
		       
	case HelpContextTransfer:
		return action == KeyActionPause || action == KeyActionResume || action == KeyActionCancel ||
		       action == KeyActionStatsOverview || action == KeyActionStatsDetailed ||
		       action == KeyActionStatsFiles || action == KeyActionStatsNetwork || 
		       action == KeyActionStatsEfficiency
		       
	case HelpContextReceiver:
		return action == KeyActionSelect || action == KeyActionCancel
		
	case HelpContextError:
		return action == KeyActionRetry || action == KeyActionCancel
		
	default:
		return true
	}
}

// QuickTip represents a contextual tip or hint
type QuickTip struct {
	message   string
	tipType   string // "info", "warning", "success", "error"
	visible   bool
	timeout   int // seconds to auto-hide, 0 = no timeout
	countdown int
}

// NewQuickTip creates a new quick tip
func NewQuickTip() *QuickTip {
	return &QuickTip{
		visible: false,
	}
}

// Show shows a quick tip
func (qt *QuickTip) Show(message, tipType string, timeout int) {
	qt.message = message
	qt.tipType = tipType
	qt.visible = true
	qt.timeout = timeout
	qt.countdown = timeout
}

// Hide hides the quick tip
func (qt *QuickTip) Hide() {
	qt.visible = false
}

// IsVisible returns whether the tip is visible
func (qt *QuickTip) IsVisible() bool {
	return qt.visible
}

// Update updates the tip countdown
func (qt *QuickTip) Update() {
	if qt.timeout > 0 && qt.countdown > 0 {
		qt.countdown--
		if qt.countdown <= 0 {
			qt.Hide()
		}
	}
}

// Render renders the quick tip
func (qt *QuickTip) Render() string {
	if !qt.visible {
		return ""
	}

	icon := qt.getTipIcon()
	tipStyle := qt.getTipStyle()

	return fmt.Sprintf("%s %s", icon, tipStyle.Render(qt.message))
}

// getTipIcon returns the appropriate icon for the tip type
func (qt *QuickTip) getTipIcon() string {
	switch qt.tipType {
	case "info":
		return "💡"
	case "warning":
		return "⚠️"
	case "success":
		return "✅"
	case "error":
		return "❌"
	default:
		return "💡"
	}
}

// getTipStyle returns the appropriate style for the tip type
func (qt *QuickTip) getTipStyle() lipgloss.Style {
	switch qt.tipType {
	case "info":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // Blue
	case "warning":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	case "success":
		return style.SuccessStyle
	case "error":
		return style.ErrorStyle
	default:
		return style.FileStyle
	}
}

// TutorialStep represents a step in an interactive tutorial
type TutorialStep struct {
	Title       string
	Description string
	Action      string
	Completed   bool
}

// Tutorial provides an interactive tutorial system
type Tutorial struct {
	steps       []TutorialStep
	currentStep int
	active      bool
	completed   bool
}

// NewTutorial creates a new tutorial
func NewTutorial() *Tutorial {
	return &Tutorial{
		steps:       make([]TutorialStep, 0),
		currentStep: 0,
		active:      false,
		completed:   false,
	}
}

// AddStep adds a step to the tutorial
func (t *Tutorial) AddStep(title, description, action string) {
	t.steps = append(t.steps, TutorialStep{
		Title:       title,
		Description: description,
		Action:      action,
		Completed:   false,
	})
}

// Start starts the tutorial
func (t *Tutorial) Start() {
	t.active = true
	t.currentStep = 0
	t.completed = false
}

// Stop stops the tutorial
func (t *Tutorial) Stop() {
	t.active = false
}

// NextStep advances to the next tutorial step
func (t *Tutorial) NextStep() {
	if t.currentStep < len(t.steps) {
		t.steps[t.currentStep].Completed = true
		t.currentStep++

		if t.currentStep >= len(t.steps) {
			t.completed = true
			t.active = false
		}
	}
}

// IsActive returns whether the tutorial is active
func (t *Tutorial) IsActive() bool {
	return t.active
}

// IsCompleted returns whether the tutorial is completed
func (t *Tutorial) IsCompleted() bool {
	return t.completed
}

// GetCurrentStep returns the current tutorial step
func (t *Tutorial) GetCurrentStep() *TutorialStep {
	if t.currentStep < len(t.steps) {
		return &t.steps[t.currentStep]
	}
	return nil
}

// Render renders the tutorial
func (t *Tutorial) Render() string {
	if !t.active {
		return ""
	}

	currentStep := t.GetCurrentStep()
	if currentStep == nil {
		return ""
	}

	var result strings.Builder

	result.WriteString("┌─────────────────────────────────────────────────────────────────────────────────┐\n")
	result.WriteString(fmt.Sprintf("│ 🎓 Tutorial - Step %d/%d\n", t.currentStep+1, len(t.steps)))
	result.WriteString("├─────────────────────────────────────────────────────────────────────────────────┤\n")
	result.WriteString(fmt.Sprintf("│ %s\n", style.HeaderStyle.Render(currentStep.Title)))
	result.WriteString("├─────────────────────────────────────────────────────────────────────────────────┤\n")
	result.WriteString(fmt.Sprintf("│ %s\n", currentStep.Description))
	result.WriteString("├─────────────────────────────────────────────────────────────────────────────────┤\n")
	result.WriteString(fmt.Sprintf("│ 👉 %s\n", style.HighlightFontStyle.Render(currentStep.Action)))
	result.WriteString("├─────────────────────────────────────────────────────────────────────────────────┤\n")
	result.WriteString("│ Press 'Esc' to skip tutorial\n")
	result.WriteString("└─────────────────────────────────────────────────────────────────────────────────┘")

	return result.String()
}

// getActionKeys returns the keyboard keys for a given action
func (hp *HelpPanel) getActionKeys(action KeyAction) []string {
	// Use keyboard manager if available
	if hp.keyboardManager != nil {
		return hp.keyboardManager.GetKeysForAction(action)
	}
	
	// Fallback to basic mapping if no keyboard manager
	switch action {
	case KeyActionQuit:
		return []string{"q", "ctrl+c"}
	case KeyActionHelp:
		return []string{"?"}
	case KeyActionRefresh:
		return []string{"r", "ctrl+r"}
	case KeyActionNavigateUp:
		return []string{"up", "k"}
	case KeyActionNavigateDown:
		return []string{"down", "j"}
	case KeyActionNavigateLeft:
		return []string{"left", "h"}
	case KeyActionNavigateRight:
		return []string{"right", "l"}
	case KeyActionSelect:
		return []string{"enter", "space"}
	case KeyActionConfirm:
		return []string{"enter", "tab"}
	case KeyActionBack:
		return []string{"esc"}
	case KeyActionCancel:
		return []string{"c", "esc"}
	case KeyActionPause:
		return []string{"p"}
	case KeyActionResume:
		return []string{"r", "space"}
	case KeyActionRetry:
		return []string{"r", "enter"}
	case KeyActionStatsOverview:
		return []string{"1"}
	case KeyActionStatsDetailed:
		return []string{"2"}
	case KeyActionStatsFiles:
		return []string{"3"}
	case KeyActionStatsNetwork:
		return []string{"4"}
	case KeyActionStatsEfficiency:
		return []string{"5"}
	case KeyActionTheme:
		return []string{"t", "T"}
	case KeyActionShowPerformance:
		return []string{"p", "P"}
	case KeyActionFullscreen:
		return []string{"f11"}
	default:
		return []string{"unknown"}
	}
}

// formatKeyDisplay formats the key display for help
func (hp *HelpPanel) formatKeyDisplay(keys []string) string {
	if len(keys) == 0 {
		return "unknown"
	}
	
	// For display, we'll show the first key or a combination
	if len(keys) == 1 {
		return keys[0]
	}
	
	// For multiple keys, show them separated by /
	return strings.Join(keys, "/")
}
