package components

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/internal/style"
)

// KeyAction represents a keyboard action
type KeyAction int

const (
	KeyActionNone KeyAction = iota
	KeyActionQuit
	KeyActionHelp
	KeyActionPause
	KeyActionResume
	KeyActionCancel
	KeyActionRetry
	KeyActionRefresh
	KeyActionNavigateUp
	KeyActionNavigateDown
	KeyActionNavigateLeft
	KeyActionNavigateRight
	KeyActionSelect
	KeyActionBack
	KeyActionConfirm
	KeyActionToggleMode
	KeyActionStatsOverview
	KeyActionStatsDetailed
	KeyActionStatsFiles
	KeyActionStatsNetwork
	KeyActionStatsEfficiency
	KeyActionSpeedUp
	KeyActionSlowDown
	KeyActionFullscreen
	KeyActionMinimize
)

// KeyBinding represents a key binding configuration
type KeyBinding struct {
	Keys        []string
	Action      KeyAction
	Description string
	Context     string
	Enabled     bool
	Global      bool // Whether this binding works in all contexts
}

// KeyboardManager manages keyboard shortcuts and navigation
type KeyboardManager struct {
	contextBindings map[string][]KeyBinding
	globalBindings  []KeyBinding
	currentContext  string
	enabled         bool
	showHints       bool
	lastKeyTime     time.Time
	keySequence     []string
	maxSequenceLen  int
}

// NewKeyboardManager creates a new keyboard manager
func NewKeyboardManager() *KeyboardManager {
	km := &KeyboardManager{
		contextBindings: make(map[string][]KeyBinding),
		globalBindings:  make([]KeyBinding, 0),
		enabled:         true,
		showHints:       true,
		maxSequenceLen:  3,
		keySequence:     make([]string, 0),
	}

	// Initialize default key bindings
	km.initializeDefaultBindings()
	return km
}

// initializeDefaultBindings sets up the default keyboard shortcuts
func (km *KeyboardManager) initializeDefaultBindings() {
	// Global bindings (work in all contexts)
	globalBindings := []KeyBinding{
		{[]string{"q", "ctrl+c"}, KeyActionQuit, "Quit application", "global", true, true},
		{[]string{"?"}, KeyActionHelp, "Toggle help", "global", true, true},
		{[]string{"f11"}, KeyActionFullscreen, "Toggle fullscreen", "global", true, true},
		{[]string{"ctrl+r"}, KeyActionRefresh, "Refresh", "global", true, true},
	}

	// Context-specific bindings
	contextBindings := map[string][]KeyBinding{
		"discovery": {
			{[]string{"r"}, KeyActionRefresh, "Refresh discovery", "discovery", true, false},
			{[]string{"esc"}, KeyActionBack, "Go back", "discovery", true, false},
		},
		"selection": {
			{[]string{"up", "k"}, KeyActionNavigateUp, "Navigate up", "selection", true, false},
			{[]string{"down", "j"}, KeyActionNavigateDown, "Navigate down", "selection", true, false},
			{[]string{"enter", "space"}, KeyActionSelect, "Select item", "selection", true, false},
			{[]string{"esc"}, KeyActionBack, "Go back", "selection", true, false},
		},
		"file_selection": {
			{[]string{"up", "k"}, KeyActionNavigateUp, "Navigate up", "file_selection", true, false},
			{[]string{"down", "j"}, KeyActionNavigateDown, "Navigate down", "file_selection", true, false},
			{[]string{"left", "h"}, KeyActionNavigateLeft, "Go back/collapse", "file_selection", true, false},
			{[]string{"right", "l"}, KeyActionNavigateRight, "Enter/expand", "file_selection", true, false},
			{[]string{"space"}, KeyActionSelect, "Toggle selection", "file_selection", true, false},
			{[]string{"enter", "tab"}, KeyActionConfirm, "Confirm selection", "file_selection", true, false},
			{[]string{"esc"}, KeyActionBack, "Cancel", "file_selection", true, false},
			{[]string{"a"}, KeyActionSelect, "Select all", "file_selection", true, false},
			{[]string{"ctrl+a"}, KeyActionSelect, "Select all", "file_selection", true, false},
		},
		"transfer": {
			{[]string{"p"}, KeyActionPause, "Pause transfer", "transfer", true, false},
			{[]string{"r"}, KeyActionResume, "Resume transfer", "transfer", true, false},
			{[]string{"c"}, KeyActionCancel, "Cancel transfer", "transfer", true, false},
			{[]string{"1"}, KeyActionStatsOverview, "Overview stats", "transfer", true, false},
			{[]string{"2"}, KeyActionStatsDetailed, "Detailed stats", "transfer", true, false},
			{[]string{"3"}, KeyActionStatsFiles, "File stats", "transfer", true, false},
			{[]string{"4"}, KeyActionStatsNetwork, "Network stats", "transfer", true, false},
			{[]string{"5"}, KeyActionStatsEfficiency, "Efficiency stats", "transfer", true, false},
			{[]string{"+"}, KeyActionSpeedUp, "Increase priority", "transfer", true, false},
			{[]string{"-"}, KeyActionSlowDown, "Decrease priority", "transfer", true, false},
		},
		"paused": {
			{[]string{"r", "space"}, KeyActionResume, "Resume transfer", "paused", true, false},
			{[]string{"c"}, KeyActionCancel, "Cancel transfer", "paused", true, false},
		},
		"error": {
			{[]string{"r", "enter"}, KeyActionRetry, "Retry operation", "error", true, false},
			{[]string{"c", "esc"}, KeyActionCancel, "Cancel", "error", true, false},
		},
		"complete": {
			{[]string{"enter"}, KeyActionConfirm, "Continue", "complete", true, false},
			{[]string{"esc"}, KeyActionBack, "Go back", "complete", true, false},
		},
	}

	// Register all bindings
	for _, binding := range globalBindings {
		km.AddGlobalBinding(binding)
	}

	for context, bindings := range contextBindings {
		for _, binding := range bindings {
			km.AddContextBinding(context, binding)
		}
	}
}

// AddGlobalBinding adds a new global key binding
func (km *KeyboardManager) AddGlobalBinding(binding KeyBinding) {
	binding.Global = true
	km.globalBindings = append(km.globalBindings, binding)
}

// AddContextBinding adds a context-specific key binding
func (km *KeyboardManager) AddContextBinding(context string, binding KeyBinding) {
	if _, exists := km.contextBindings[context]; !exists {
		km.contextBindings[context] = make([]KeyBinding, 0)
	}
	km.contextBindings[context] = append(km.contextBindings[context], binding)
}

// SetContext sets the current context for key bindings
func (km *KeyboardManager) SetContext(context string) {
	km.currentContext = context
}

// GetContext returns the current context
func (km *KeyboardManager) GetContext() string {
	return km.currentContext
}

// ProcessKey processes a key press and returns the corresponding action
func (km *KeyboardManager) ProcessKey(keyMsg tea.KeyMsg) KeyAction {
	if !km.enabled {
		return KeyActionNone
	}

	keyStr := keyMsg.String()

	// Check context-specific bindings first
	if contextBindings, ok := km.contextBindings[km.currentContext]; ok {
		for _, binding := range contextBindings {
			if !binding.Enabled {
				continue
			}
			for _, key := range binding.Keys {
				if key == keyStr {
					return binding.Action
				}
			}
		}
	}

	// Check global bindings if no context-specific match was found
	for _, binding := range km.globalBindings {
		if !binding.Enabled {
			continue
		}
		for _, key := range binding.Keys {
			if key == keyStr {
				return binding.Action
			}
		}
	}

	return KeyActionNone
}

func (km *KeyboardManager) ProcessSpecAction(action KeyAction) tea.KeyMsg {
	switch action {
	case KeyActionNavigateUp:
		return tea.KeyMsg{
			Type: tea.KeyUp,
		}
	case KeyActionNavigateDown:
		return tea.KeyMsg{
			Type: tea.KeyDown,
		}
	case KeyActionSelect:
		return tea.KeyMsg{
			Type: tea.KeyEnter,
		}
	}
	return tea.KeyMsg{}
}

// GetActiveBindings returns all active bindings for the current context
func (km *KeyboardManager) GetActiveBindings() []KeyBinding {
	var active []KeyBinding

	// Add global bindings
	for _, binding := range km.globalBindings {
		if binding.Enabled {
			active = append(active, binding)
		}
	}

	// Add context-specific bindings
	if contextBindings, exists := km.contextBindings[km.currentContext]; exists {
		for _, binding := range contextBindings {
			if binding.Enabled {
				active = append(active, binding)
			}
		}
	}

	return active
}

// EnableBinding enables or disables a specific binding
func (km *KeyboardManager) EnableBinding(keys []string, enabled bool) {
	keyMap := make(map[string]struct{})
	for _, k := range keys {
		keyMap[k] = struct{}{}
	}

	// Helper function to check for key match
	hasKey := func(bindingKeys []string) bool {
		for _, bk := range bindingKeys {
			if _, ok := keyMap[bk]; ok {
				return true
			}
		}
		return false
	}

	// Update global bindings
	for i, binding := range km.globalBindings {
		if hasKey(binding.Keys) {
			km.globalBindings[i].Enabled = enabled
		}
	}

	// Update context bindings
	for context, bindings := range km.contextBindings {
		for i, binding := range bindings {
			if hasKey(binding.Keys) {
				km.contextBindings[context][i].Enabled = enabled
			}
		}
	}
}

// SetEnabled enables or disables the entire keyboard manager
func (km *KeyboardManager) SetEnabled(enabled bool) {
	km.enabled = enabled
}

// IsEnabled returns whether the keyboard manager is enabled
func (km *KeyboardManager) IsEnabled() bool {
	return km.enabled
}

// SetShowHints sets whether to show keyboard hints
func (km *KeyboardManager) SetShowHints(show bool) {
	km.showHints = show
}

// RenderHints renders keyboard hints for the current context
func (km *KeyboardManager) RenderHints() string {
	if !km.showHints {
		return ""
	}

	activeBindings := km.GetActiveBindings()
	if len(activeBindings) == 0 {
		return ""
	}

	// Group bindings by importance
	important := make([]KeyBinding, 0)
	normal := make([]KeyBinding, 0)

	for _, binding := range activeBindings {
		// Consider global bindings and primary actions as important
		if binding.Global || 
		   binding.Action == KeyActionSelect || 
		   binding.Action == KeyActionConfirm ||
		   binding.Action == KeyActionPause ||
		   binding.Action == KeyActionResume {
			important = append(important, binding)
		} else {
			normal = append(normal, binding)
		}
	}

	var result strings.Builder

	// Show important bindings first
	if len(important) > 0 {
		for i, binding := range important {
			if i > 0 {
				result.WriteString(" | ")
			}
			
			// Use the first key as the display key
			displayKey := binding.Keys[0]
			result.WriteString(fmt.Sprintf("%s=%s",
				style.HighlightFontStyle.Render(displayKey),
				binding.Description))
		}
	}

	// Add normal bindings if there's space
	if len(normal) > 0 && len(important) < 4 {
		remaining := 4 - len(important)
		if remaining > len(normal) {
			remaining = len(normal)
		}

		for i := 0; i < remaining; i++ {
			if len(important) > 0 || i > 0 {
				result.WriteString(" | ")
			}
			
			displayKey := normal[i].Keys[0]
			result.WriteString(fmt.Sprintf("%s=%s",
				style.FileStyle.Render(displayKey),
				normal[i].Description))
		}
	}

	return style.FileStyle.Render(result.String())
}

// RenderFullHelp renders the complete help for the current context
func (km *KeyboardManager) RenderFullHelp() string {
	activeBindings := km.GetActiveBindings()
	if len(activeBindings) == 0 {
		return "No keyboard shortcuts available"
	}

	var result strings.Builder

	result.WriteString("┌─────────────────────────────────────────────────────────────────────────────────┐\n")
	result.WriteString("│ ⌨️  Keyboard Shortcuts\n")
	result.WriteString("├─────────────────────────────────────────────────────────────────────────────────┤\n")

	// Group by context
	contextGroups := make(map[string][]KeyBinding)
	for _, binding := range activeBindings {
		context := binding.Context
		if binding.Global {
			context = "Global"
		}
		contextGroups[context] = append(contextGroups[context], binding)
	}

	first := true
	for context, bindings := range contextGroups {
		if !first {
			result.WriteString("├─────────────────────────────────────────────────────────────────────────────────┤\n")
		}
		first = false

		result.WriteString(fmt.Sprintf("│ %s\n", style.HeaderStyle.Render(context)))
		
		for _, binding := range bindings {
			keys := strings.Join(binding.Keys, ", ")
			result.WriteString(fmt.Sprintf("│ %s %s\n",
				style.HighlightFontStyle.Render(fmt.Sprintf("%-" + "15s", keys)),
				binding.Description))
		}
	}

	result.WriteString("└─────────────────────────────────────────────────────────────────────────────────┘")

	return result.String()
}

// NavigationState represents the current navigation state
type NavigationState struct {
	CurrentIndex    int
	MaxIndex        int
	CanGoBack       bool
	CanGoForward    bool
	SelectionMode   bool
	MultiSelect     bool
	SelectedItems   map[int]bool
	History         []string
	HistoryIndex    int
}

// NewNavigationState creates a new navigation state
func NewNavigationState(maxIndex int) *NavigationState {
	return &NavigationState{
		CurrentIndex:  0,
		MaxIndex:      maxIndex,
		SelectedItems: make(map[int]bool),
		History:       make([]string, 0),
		HistoryIndex:  -1,
	}
}

// Navigate handles navigation actions
func (ns *NavigationState) Navigate(action KeyAction) bool {
	switch action {
	case KeyActionNavigateUp:
		if ns.CurrentIndex > 0 {
			ns.CurrentIndex--
			return true
		}
	case KeyActionNavigateDown:
		if ns.CurrentIndex < ns.MaxIndex {
			ns.CurrentIndex++
			return true
		}
	case KeyActionNavigateLeft:
		if ns.CanGoBack {
			return true
		}
	case KeyActionNavigateRight:
		if ns.CanGoForward {
			return true
		}
	case KeyActionSelect:
		if ns.MultiSelect {
			ns.SelectedItems[ns.CurrentIndex] = !ns.SelectedItems[ns.CurrentIndex]
		} else {
			// Clear previous selections in single-select mode
			ns.SelectedItems = make(map[int]bool)
			ns.SelectedItems[ns.CurrentIndex] = true
		}
		return true
	}
	return false
}

// SetMaxIndex updates the maximum index
func (ns *NavigationState) SetMaxIndex(maxIndex int) {
	ns.MaxIndex = maxIndex
	if ns.CurrentIndex > maxIndex {
		ns.CurrentIndex = maxIndex
	}
}

// GetSelectedIndices returns all selected indices
func (ns *NavigationState) GetSelectedIndices() []int {
	var indices []int
	for index, selected := range ns.SelectedItems {
		if selected {
			indices = append(indices, index)
		}
	}
	return indices
}

// ClearSelection clears all selections
func (ns *NavigationState) ClearSelection() {
	ns.SelectedItems = make(map[int]bool)
}

// AddToHistory adds a state to the navigation history
func (ns *NavigationState) AddToHistory(state string) {
	// Remove any history after current position
	if ns.HistoryIndex < len(ns.History)-1 {
		ns.History = ns.History[:ns.HistoryIndex+1]
	}
	
	ns.History = append(ns.History, state)
	ns.HistoryIndex = len(ns.History) - 1
	
	// Limit history size
	if len(ns.History) > 50 {
		ns.History = ns.History[1:]
		ns.HistoryIndex--
	}
}

// CanGoBackInHistory returns whether we can go back in history
func (ns *NavigationState) CanGoBackInHistory() bool {
	return ns.HistoryIndex > 0
}

// CanGoForwardInHistory returns whether we can go forward in history
func (ns *NavigationState) CanGoForwardInHistory() bool {
	return ns.HistoryIndex < len(ns.History)-1
}

// GoBackInHistory goes back one step in history
func (ns *NavigationState) GoBackInHistory() string {
	if ns.CanGoBackInHistory() {
		ns.HistoryIndex--
		return ns.History[ns.HistoryIndex]
	}
	return ""
}

// GoForwardInHistory goes forward one step in history
func (ns *NavigationState) GoForwardInHistory() string {
	if ns.CanGoForwardInHistory() {
		ns.HistoryIndex++
		return ns.History[ns.HistoryIndex]
	}
	return ""
}
