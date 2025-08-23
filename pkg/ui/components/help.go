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
	Key         string
	Description string
	Important   bool
}

// HelpPanel provides context-sensitive help and keyboard shortcuts
type HelpPanel struct {
	context     HelpContext
	visible     bool
	compact     bool
	customItems []HelpItem
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
func (hp *HelpPanel) AddCustomItem(key, description string, important bool) {
	hp.customItems = append(hp.customItems, HelpItem{
		Key:         key,
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
	return style.FileStyle.Render("Press '?' for help")
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
		result.WriteString(fmt.Sprintf("%s=%s",
			style.HighlightFontStyle.Render(item.Key),
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

		result.WriteString(fmt.Sprintf("│ %s %s\n",
			keyStyle.Render(fmt.Sprintf("%-12s", item.Key)),
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
		return "Finding Receivers"
	case HelpContextSenderSelection:
		return "Select Receiver"
	case HelpContextFileSelection:
		return "Select Files to Send"
	case HelpContextTransfer:
		return "File Transfer in Progress"
	case HelpContextReceiver:
		return "Receiving Files"
	case HelpContextError:
		return "Error Recovery"
	default:
		return "General"
	}
}

// getHelpItems returns the help items for the current context
func (hp *HelpPanel) getHelpItems() []HelpItem {
	var items []HelpItem

	// Add context-specific items
	switch hp.context {
	case HelpContextSenderDiscovery:
		items = []HelpItem{
			{"Ctrl+C", "Quit application", true},
			{"R", "Refresh/restart discovery", false},
			{"?", "Toggle help", false},
		}

	case HelpContextSenderSelection:
		items = []HelpItem{
			{"↑/↓", "Navigate receivers", true},
			{"Enter", "Select receiver", true},
			{"R", "Refresh receiver list", false},
			{"Ctrl+C", "Quit application", false},
			{"?", "Toggle help", false},
		}

	case HelpContextFileSelection:
		items = []HelpItem{
			{"↑/↓", "Navigate files/folders", true},
			{"Enter", "Select/deselect file", true},
			{"Space", "Select/deselect file", true},
			{"→", "Enter folder", false},
			{"←", "Go back", false},
			{"Tab", "Confirm selection", true},
			{"Esc", "Cancel", false},
			{"?", "Toggle help", false},
		}

	case HelpContextTransfer:
		items = []HelpItem{
			{"P", "Pause transfer", true},
			{"R", "Resume transfer (if paused)", true},
			{"C", "Cancel transfer", true},
			{"Ctrl+C", "Quit application", false},
			{"?", "Toggle help", false},
		}

	case HelpContextReceiver:
		items = []HelpItem{
			{"Y", "Accept incoming transfer", true},
			{"N", "Reject incoming transfer", true},
			{"Ctrl+C", "Quit application", false},
			{"?", "Toggle help", false},
		}

	case HelpContextError:
		items = []HelpItem{
			{"R", "Retry operation", true},
			{"Enter", "Try again", true},
			{"C", "Cancel", false},
			{"Q", "Quit application", false},
			{"?", "Toggle help", false},
		}

	default: // HelpContextMain
		items = []HelpItem{
			{"S", "Start as sender", true},
			{"R", "Start as receiver", true},
			{"Q", "Quit application", true},
			{"?", "Toggle help", false},
		}
	}

	// Add custom items
	items = append(items, hp.customItems...)

	return items
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
