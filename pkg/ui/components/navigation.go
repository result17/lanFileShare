package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rescp17/lanFileSharer/internal/style"
)

// NavigationMode represents different navigation modes
type NavigationMode int

const (
	NavigationModeList NavigationMode = iota
	NavigationModeGrid
	NavigationModeTree
	NavigationModeTable
)

// BreadcrumbItem represents an item in the breadcrumb navigation
type BreadcrumbItem struct {
	Label     string
	Value     string
	Icon      string
	Clickable bool
}

// Breadcrumb provides breadcrumb navigation
type Breadcrumb struct {
	items     []BreadcrumbItem
	maxItems  int
	separator string
	style     lipgloss.Style
}

// NewBreadcrumb creates a new breadcrumb navigation
func NewBreadcrumb(maxItems int) *Breadcrumb {
	return &Breadcrumb{
		items:     make([]BreadcrumbItem, 0),
		maxItems:  maxItems,
		separator: " › ",
		style:     style.FileStyle,
	}
}

// AddItem adds an item to the breadcrumb
func (b *Breadcrumb) AddItem(label, value, icon string, clickable bool) {
	item := BreadcrumbItem{
		Label:     label,
		Value:     value,
		Icon:      icon,
		Clickable: clickable,
	}

	b.items = append(b.items, item)

	// Keep only the most recent items
	if len(b.items) > b.maxItems {
		b.items = b.items[len(b.items)-b.maxItems:]
	}
}

// PopItem removes the last item from the breadcrumb
func (b *Breadcrumb) PopItem() *BreadcrumbItem {
	if len(b.items) == 0 {
		return nil
	}

	item := b.items[len(b.items)-1]
	b.items = b.items[:len(b.items)-1]
	return &item
}

// Clear clears all items from the breadcrumb
func (b *Breadcrumb) Clear() {
	b.items = b.items[:0]
}

// GetItems returns all breadcrumb items
func (b *Breadcrumb) GetItems() []BreadcrumbItem {
	return b.items
}

// Render renders the breadcrumb navigation
func (b *Breadcrumb) Render() string {
	if len(b.items) == 0 {
		return ""
	}

	var result strings.Builder

	for i, item := range b.items {
		if i > 0 {
			result.WriteString(b.style.Render(b.separator))
		}

		// Add icon if present
		if item.Icon != "" {
			result.WriteString(item.Icon + " ")
		}

		// Style the label based on whether it's clickable and if it's the last item
		if i == len(b.items)-1 {
			// Last item (current location) - highlight
			result.WriteString(style.HighlightFontStyle.Render(item.Label))
		} else if item.Clickable {
			// Clickable items - use link style
			result.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Underline(true).Render(item.Label))
		} else {
			// Regular items
			result.WriteString(b.style.Render(item.Label))
		}
	}

	return result.String()
}

// TabBar represents a tab-based navigation
type TabBar struct {
	tabs        []TabItem
	activeIndex int
	style       TabBarStyle
}

// TabItem represents a single tab
type TabItem struct {
	Label      string
	Value      string
	Icon       string
	Enabled    bool
	Badge      string
	BadgeStyle lipgloss.Style
}

// TabBarStyle defines the styling for the tab bar
type TabBarStyle struct {
	ActiveTab   lipgloss.Style
	InactiveTab lipgloss.Style
	Separator   string
	Border      bool
}

// NewTabBar creates a new tab bar
func NewTabBar() *TabBar {
	return &TabBar{
		tabs:        make([]TabItem, 0),
		activeIndex: 0,
		style: TabBarStyle{
			ActiveTab:   style.HighlightFontStyle.Copy().Bold(true),
			InactiveTab: style.FileStyle,
			Separator:   " │ ",
			Border:      true,
		},
	}
}

// AddTab adds a new tab
func (tb *TabBar) AddTab(label, value, icon string, enabled bool) {
	tab := TabItem{
		Label:   label,
		Value:   value,
		Icon:    icon,
		Enabled: enabled,
	}
	tb.tabs = append(tb.tabs, tab)
}

// SetActiveTab sets the active tab by index
func (tb *TabBar) SetActiveTab(index int) bool {
	if index >= 0 && index < len(tb.tabs) && tb.tabs[index].Enabled {
		tb.activeIndex = index
		return true
	}
	return false
}

// SetActiveTabByValue sets the active tab by value
func (tb *TabBar) SetActiveTabByValue(value string) bool {
	for i, tab := range tb.tabs {
		if tab.Value == value && tab.Enabled {
			tb.activeIndex = i
			return true
		}
	}
	return false
}

// GetActiveTab returns the currently active tab
func (tb *TabBar) GetActiveTab() *TabItem {
	if tb.activeIndex >= 0 && tb.activeIndex < len(tb.tabs) {
		return &tb.tabs[tb.activeIndex]
	}
	return nil
}

// NextTab moves to the next enabled tab
func (tb *TabBar) NextTab() bool {
	for i := tb.activeIndex + 1; i < len(tb.tabs); i++ {
		if tb.tabs[i].Enabled {
			tb.activeIndex = i
			return true
		}
	}
	// Wrap around to the beginning
	for i := 0; i < tb.activeIndex; i++ {
		if tb.tabs[i].Enabled {
			tb.activeIndex = i
			return true
		}
	}
	return false
}

// PrevTab moves to the previous enabled tab
func (tb *TabBar) PrevTab() bool {
	for i := tb.activeIndex - 1; i >= 0; i-- {
		if tb.tabs[i].Enabled {
			tb.activeIndex = i
			return true
		}
	}
	// Wrap around to the end
	for i := len(tb.tabs) - 1; i > tb.activeIndex; i-- {
		if tb.tabs[i].Enabled {
			tb.activeIndex = i
			return true
		}
	}
	return false
}

// SetTabBadge sets a badge for a specific tab
func (tb *TabBar) SetTabBadge(index int, badge string, badgeStyle lipgloss.Style) {
	if index >= 0 && index < len(tb.tabs) {
		tb.tabs[index].Badge = badge
		tb.tabs[index].BadgeStyle = badgeStyle
	}
}

// Render renders the tab bar
func (tb *TabBar) Render() string {
	if len(tb.tabs) == 0 {
		return ""
	}

	var result strings.Builder

	if tb.style.Border {
		result.WriteString("┌")
		for i, tab := range tb.tabs {
			if i > 0 {
				result.WriteString("┬")
			}

			tabWidth := len(tab.Label) + 2 // padding
			if tab.Icon != "" {
				tabWidth += 2 // icon + space
			}
			if tab.Badge != "" {
				tabWidth += len(tab.Badge) + 2 // badge + brackets
			}

			result.WriteString(strings.Repeat("─", tabWidth))
		}
		result.WriteString("┐\n")
	}

	// Tab labels
	result.WriteString("│")
	for i, tab := range tb.tabs {
		if i > 0 {
			result.WriteString("│")
		}

		var tabContent strings.Builder
		tabContent.WriteString(" ")

		// Add icon
		if tab.Icon != "" {
			tabContent.WriteString(tab.Icon + " ")
		}

		// Add label
		tabContent.WriteString(tab.Label)

		// Add badge
		if tab.Badge != "" {
			badgeText := fmt.Sprintf("[%s]", tab.Badge)
			tabContent.WriteString(" " + tab.BadgeStyle.Render(badgeText))
		}

		tabContent.WriteString(" ")

		// Apply styling
		if i == tb.activeIndex {
			result.WriteString(tb.style.ActiveTab.Render(tabContent.String()))
		} else if tab.Enabled {
			result.WriteString(tb.style.InactiveTab.Render(tabContent.String()))
		} else {
			disabledStyle := tb.style.InactiveTab.Copy().Foreground(lipgloss.Color("240"))
			result.WriteString(disabledStyle.Render(tabContent.String()))
		}
	}
	result.WriteString("│\n")

	if tb.style.Border {
		result.WriteString("└")
		for i, tab := range tb.tabs {
			if i > 0 {
				result.WriteString("┴")
			}

			tabWidth := len(tab.Label) + 2 // padding
			if tab.Icon != "" {
				tabWidth += 2 // icon + space
			}
			if tab.Badge != "" {
				tabWidth += len(tab.Badge) + 2 // badge + brackets
			}

			result.WriteString(strings.Repeat("─", tabWidth))
		}
		result.WriteString("┘")
	}

	return result.String()
}

// StatusBar provides a status bar with navigation information
type StatusBar struct {
	leftItems   []StatusItem
	rightItems  []StatusItem
	centerItems []StatusItem
	width       int
	style       lipgloss.Style
}

// StatusItem represents an item in the status bar
type StatusItem struct {
	Text      string
	Icon      string
	Style     lipgloss.Style
	Clickable bool
	Action    string
}

// NewStatusBar creates a new status bar
func NewStatusBar(width int) *StatusBar {
	return &StatusBar{
		leftItems:   make([]StatusItem, 0),
		rightItems:  make([]StatusItem, 0),
		centerItems: make([]StatusItem, 0),
		width:       width,
		style:       style.FileStyle,
	}
}

// AddLeftItem adds an item to the left side of the status bar
func (sb *StatusBar) AddLeftItem(text, icon string, itemStyle lipgloss.Style) {
	item := StatusItem{
		Text:  text,
		Icon:  icon,
		Style: itemStyle,
	}
	sb.leftItems = append(sb.leftItems, item)
}

// AddRightItem adds an item to the right side of the status bar
func (sb *StatusBar) AddRightItem(text, icon string, itemStyle lipgloss.Style) {
	item := StatusItem{
		Text:  text,
		Icon:  icon,
		Style: itemStyle,
	}
	sb.rightItems = append(sb.rightItems, item)
}

// AddCenterItem adds an item to the center of the status bar
func (sb *StatusBar) AddCenterItem(text, icon string, itemStyle lipgloss.Style) {
	item := StatusItem{
		Text:  text,
		Icon:  icon,
		Style: itemStyle,
	}
	sb.centerItems = append(sb.centerItems, item)
}

// Clear clears all items from the status bar
func (sb *StatusBar) Clear() {
	sb.leftItems = sb.leftItems[:0]
	sb.rightItems = sb.rightItems[:0]
	sb.centerItems = sb.centerItems[:0]
}

// SetWidth sets the width of the status bar
func (sb *StatusBar) SetWidth(width int) {
	sb.width = width
}

// Render renders the status bar
func (sb *StatusBar) Render() string {
	if sb.width <= 0 {
		return ""
	}

	// Render items
	leftText := sb.renderItems(sb.leftItems)
	rightText := sb.renderItems(sb.rightItems)
	centerText := sb.renderItems(sb.centerItems)

	// Calculate spacing
	leftLen := lipgloss.Width(leftText)
	rightLen := lipgloss.Width(rightText)
	centerLen := lipgloss.Width(centerText)

	availableWidth := sb.width - leftLen - rightLen

	var result strings.Builder
	result.WriteString(leftText)

	if centerLen > 0 && availableWidth >= centerLen {
		// Center the center text
		leftPadding := (availableWidth - centerLen) / 2
		rightPadding := availableWidth - centerLen - leftPadding

		result.WriteString(strings.Repeat(" ", leftPadding))
		result.WriteString(centerText)
		result.WriteString(strings.Repeat(" ", rightPadding))
	} else {
		// Fill remaining space
		result.WriteString(strings.Repeat(" ", availableWidth))
	}

	result.WriteString(rightText)

	return sb.style.Width(sb.width).Render(result.String())
}

// renderItems renders a list of status items
func (sb *StatusBar) renderItems(items []StatusItem) string {
	if len(items) == 0 {
		return ""
	}

	var result strings.Builder
	for i, item := range items {
		if i > 0 {
			result.WriteString(" ")
		}

		var itemText strings.Builder
		if item.Icon != "" {
			itemText.WriteString(item.Icon + " ")
		}
		itemText.WriteString(item.Text)

		result.WriteString(item.Style.Render(itemText.String()))
	}

	return result.String()
}

// ContextualMenu represents a context-sensitive menu
type ContextualMenu struct {
	items         []MenuItem
	visible       bool
	selectedIndex int
	title         string
	position      MenuPosition
	maxWidth      int
}

// MenuItem represents a menu item
type MenuItem struct {
	Label     string
	Value     string
	Icon      string
	Enabled   bool
	Separator bool
	Submenu   *ContextualMenu
	Action    KeyAction
	Shortcut  string
}

// MenuPosition represents the position of a menu
type MenuPosition struct {
	X int
	Y int
}

// NewContextualMenu creates a new contextual menu
func NewContextualMenu(title string) *ContextualMenu {
	return &ContextualMenu{
		items:         make([]MenuItem, 0),
		visible:       false,
		selectedIndex: 0,
		title:         title,
		maxWidth:      40,
	}
}

// AddItem adds an item to the menu
func (cm *ContextualMenu) AddItem(label, value, icon string, enabled bool, action KeyAction, shortcut string) {
	item := MenuItem{
		Label:    label,
		Value:    value,
		Icon:     icon,
		Enabled:  enabled,
		Action:   action,
		Shortcut: shortcut,
	}
	cm.items = append(cm.items, item)
}

// AddSeparator adds a separator to the menu
func (cm *ContextualMenu) AddSeparator() {
	item := MenuItem{
		Separator: true,
	}
	cm.items = append(cm.items, item)
}

// Show shows the menu at the specified position
func (cm *ContextualMenu) Show(x, y int) {
	cm.visible = true
	cm.position = MenuPosition{X: x, Y: y}
	cm.selectedIndex = 0
}

// Hide hides the menu
func (cm *ContextualMenu) Hide() {
	cm.visible = false
}

// IsVisible returns whether the menu is visible
func (cm *ContextualMenu) IsVisible() bool {
	return cm.visible
}

// Navigate handles navigation within the menu
func (cm *ContextualMenu) Navigate(action KeyAction) bool {
	if !cm.visible {
		return false
	}

	switch action {
	case KeyActionNavigateUp:
		for i := cm.selectedIndex - 1; i >= 0; i-- {
			if !cm.items[i].Separator && cm.items[i].Enabled {
				cm.selectedIndex = i
				return true
			}
		}
	case KeyActionNavigateDown:
		for i := cm.selectedIndex + 1; i < len(cm.items); i++ {
			if !cm.items[i].Separator && cm.items[i].Enabled {
				cm.selectedIndex = i
				return true
			}
		}
	case KeyActionSelect:
		if cm.selectedIndex >= 0 && cm.selectedIndex < len(cm.items) {
			item := cm.items[cm.selectedIndex]
			if item.Enabled && !item.Separator {
				cm.Hide()
				return true
			}
		}
	}

	return false
}

// GetSelectedItem returns the currently selected menu item
func (cm *ContextualMenu) GetSelectedItem() *MenuItem {
	if cm.selectedIndex >= 0 && cm.selectedIndex < len(cm.items) {
		return &cm.items[cm.selectedIndex]
	}
	return nil
}

// Render renders the contextual menu
func (cm *ContextualMenu) Render() string {
	if !cm.visible || len(cm.items) == 0 {
		return ""
	}

	var result strings.Builder

	// Calculate menu width
	width := len(cm.title) + 4
	for _, item := range cm.items {
		if !item.Separator {
			itemWidth := len(item.Label) + 4
			if item.Icon != "" {
				itemWidth += 2
			}
			if item.Shortcut != "" {
				itemWidth += len(item.Shortcut) + 2
			}
			if itemWidth > width {
				width = itemWidth
			}
		}
	}

	if width > cm.maxWidth {
		width = cm.maxWidth
	}

	// Top border
	result.WriteString("┌" + strings.Repeat("─", width-2) + "┐\n")

	// Title
	if cm.title != "" {
		titleText := fmt.Sprintf(" %s ", cm.title)
		padding := width - len(titleText) - 2
		result.WriteString("│" + titleText + strings.Repeat(" ", padding) + "│\n")
		result.WriteString("├" + strings.Repeat("─", width-2) + "┤\n")
	}

	// Menu items
	for i, item := range cm.items {
		if item.Separator {
			result.WriteString("├" + strings.Repeat("─", width-2) + "┤\n")
			continue
		}

		var itemText strings.Builder
		itemText.WriteString(" ")

		// Icon
		if item.Icon != "" {
			itemText.WriteString(item.Icon + " ")
		}

		// Label
		itemText.WriteString(item.Label)

		// Shortcut
		if item.Shortcut != "" {
			// Right-align shortcut
			currentLen := itemText.Len()
			shortcutLen := len(item.Shortcut)
			padding := width - currentLen - shortcutLen - 3
			if padding > 0 {
				itemText.WriteString(strings.Repeat(" ", padding))
				itemText.WriteString(item.Shortcut)
			}
		}

		itemText.WriteString(" ")

		// Apply styling
		line := "│" + itemText.String()

		// Pad to width
		currentWidth := len(line)
		if currentWidth < width-1 {
			line += strings.Repeat(" ", width-1-currentWidth)
		}
		line += "│"

		if i == cm.selectedIndex && item.Enabled {
			result.WriteString(style.HighlightFontStyle.Render(line) + "\n")
		} else if item.Enabled {
			result.WriteString(line + "\n")
		} else {
			disabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			result.WriteString(disabledStyle.Render(line) + "\n")
		}
	}

	// Bottom border
	result.WriteString("└" + strings.Repeat("─", width-2) + "┘")

	return result.String()
}
