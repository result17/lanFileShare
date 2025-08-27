package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// LayoutBreakpoint represents different screen size breakpoints
type LayoutBreakpoint int

const (
	BreakpointXSmall LayoutBreakpoint = iota // < 40 cols
	BreakpointSmall                          // 40-79 cols
	BreakpointMedium                         // 80-119 cols
	BreakpointLarge                          // 120-159 cols
	BreakpointXLarge                         // >= 160 cols
)

// LayoutMode represents different layout modes
type LayoutMode int

const (
	LayoutModeAuto LayoutMode = iota // Automatically choose based on screen size
	LayoutModeCompact                // Force compact layout
	LayoutModeNormal                 // Force normal layout
	LayoutModeExpanded               // Force expanded layout
)

// ViewportSize represents the current viewport dimensions
type ViewportSize struct {
	Width  int
	Height int
}

// LayoutConfig contains configuration for responsive layout
type LayoutConfig struct {
	MinWidth       int
	MaxWidth       int
	MinHeight      int
	MaxHeight      int
	Padding        int
	Margin         int
	CompactMode    bool
	ShowSidebar    bool
	ShowStatusBar  bool
	ShowBreadcrumb bool
	ColumnCount    int
}

// ResponsiveLayout manages responsive layout behavior
type ResponsiveLayout struct {
	viewport    ViewportSize
	breakpoint  LayoutBreakpoint
	mode        LayoutMode
	config      LayoutConfig
	themeManager *ThemeManager
	
	// Layout regions
	headerHeight    int
	footerHeight    int
	sidebarWidth    int
	contentPadding  int
	
	// Adaptive settings
	showDetails     bool
	showIcons       bool
	showTimestamps  bool
	maxItems        int
	truncateLength  int
}

// NewResponsiveLayout creates a new responsive layout manager
func NewResponsiveLayout(themeManager *ThemeManager) *ResponsiveLayout {
	return &ResponsiveLayout{
		viewport: ViewportSize{Width: 80, Height: 24}, // Default terminal size
		mode:     LayoutModeAuto,
		config: LayoutConfig{
			MinWidth:       40,
			MaxWidth:       200,
			MinHeight:      10,
			MaxHeight:      100,
			Padding:        1,
			Margin:         1,
			CompactMode:    false,
			ShowSidebar:    true,
			ShowStatusBar:  true,
			ShowBreadcrumb: true,
			ColumnCount:    1,
		},
		themeManager:   themeManager,
		headerHeight:   3,
		footerHeight:   2,
		sidebarWidth:   20,
		contentPadding: 2,
		showDetails:    true,
		showIcons:      true,
		showTimestamps: true,
		maxItems:       50,
		truncateLength: 30,
	}
}

// Update updates the layout based on new viewport size
func (rl *ResponsiveLayout) Update(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		rl.SetViewportSize(msg.Width, msg.Height)
	}
}

// SetViewportSize sets the viewport size and updates layout accordingly
func (rl *ResponsiveLayout) SetViewportSize(width, height int) {
	rl.viewport.Width = width
	rl.viewport.Height = height
	rl.updateBreakpoint()
	rl.updateLayoutConfig()
}

// GetViewportSize returns the current viewport size
func (rl *ResponsiveLayout) GetViewportSize() ViewportSize {
	return rl.viewport
}

// updateBreakpoint determines the current breakpoint based on viewport width
func (rl *ResponsiveLayout) updateBreakpoint() {
	switch {
	case rl.viewport.Width < 40:
		rl.breakpoint = BreakpointXSmall
	case rl.viewport.Width < 80:
		rl.breakpoint = BreakpointSmall
	case rl.viewport.Width < 120:
		rl.breakpoint = BreakpointMedium
	case rl.viewport.Width < 160:
		rl.breakpoint = BreakpointLarge
	default:
		rl.breakpoint = BreakpointXLarge
	}
}

// updateLayoutConfig updates layout configuration based on current breakpoint
func (rl *ResponsiveLayout) updateLayoutConfig() {
	switch rl.breakpoint {
	case BreakpointXSmall:
		rl.config.CompactMode = true
		rl.config.ShowSidebar = false
		rl.config.ShowBreadcrumb = false
		rl.config.Padding = 0
		rl.config.Margin = 0
		rl.config.ColumnCount = 1
		rl.showDetails = false
		rl.showIcons = false
		rl.showTimestamps = false
		rl.maxItems = 10
		rl.truncateLength = 15
		rl.sidebarWidth = 0
		rl.contentPadding = 0
		
	case BreakpointSmall:
		rl.config.CompactMode = true
		rl.config.ShowSidebar = false
		rl.config.ShowBreadcrumb = true
		rl.config.Padding = 0
		rl.config.Margin = 0
		rl.config.ColumnCount = 1
		rl.showDetails = false
		rl.showIcons = true
		rl.showTimestamps = false
		rl.maxItems = 20
		rl.truncateLength = 20
		rl.sidebarWidth = 0
		rl.contentPadding = 1
		
	case BreakpointMedium:
		rl.config.CompactMode = false
		rl.config.ShowSidebar = false
		rl.config.ShowBreadcrumb = true
		rl.config.Padding = 1
		rl.config.Margin = 1
		rl.config.ColumnCount = 1
		rl.showDetails = true
		rl.showIcons = true
		rl.showTimestamps = true
		rl.maxItems = 30
		rl.truncateLength = 25
		rl.sidebarWidth = 0
		rl.contentPadding = 1
		
	case BreakpointLarge:
		rl.config.CompactMode = false
		rl.config.ShowSidebar = true
		rl.config.ShowBreadcrumb = true
		rl.config.Padding = 1
		rl.config.Margin = 1
		rl.config.ColumnCount = 2
		rl.showDetails = true
		rl.showIcons = true
		rl.showTimestamps = true
		rl.maxItems = 40
		rl.truncateLength = 30
		rl.sidebarWidth = 20
		rl.contentPadding = 2
		
	case BreakpointXLarge:
		rl.config.CompactMode = false
		rl.config.ShowSidebar = true
		rl.config.ShowBreadcrumb = true
		rl.config.Padding = 2
		rl.config.Margin = 2
		rl.config.ColumnCount = 3
		rl.showDetails = true
		rl.showIcons = true
		rl.showTimestamps = true
		rl.maxItems = 50
		rl.truncateLength = 40
		rl.sidebarWidth = 25
		rl.contentPadding = 2
	}
	
	// Override with theme settings if compact theme is active
	if rl.themeManager != nil {
		theme := rl.themeManager.GetCurrentTheme()
		if theme != nil && theme.CompactMode {
			rl.config.CompactMode = true
			rl.config.Padding = 0
			rl.config.Margin = 0
			rl.showDetails = false
			rl.contentPadding = 0
		}
	}
}

// GetBreakpoint returns the current breakpoint
func (rl *ResponsiveLayout) GetBreakpoint() LayoutBreakpoint {
	return rl.breakpoint
}

// GetConfig returns the current layout configuration
func (rl *ResponsiveLayout) GetConfig() LayoutConfig {
	return rl.config
}

// IsCompactMode returns whether compact mode is active
func (rl *ResponsiveLayout) IsCompactMode() bool {
	return rl.config.CompactMode
}

// ShouldShowSidebar returns whether the sidebar should be shown
func (rl *ResponsiveLayout) ShouldShowSidebar() bool {
	return rl.config.ShowSidebar
}

// ShouldShowDetails returns whether detailed information should be shown
func (rl *ResponsiveLayout) ShouldShowDetails() bool {
	return rl.showDetails
}

// ShouldShowIcons returns whether icons should be shown
func (rl *ResponsiveLayout) ShouldShowIcons() bool {
	return rl.showIcons
}

// ShouldShowTimestamps returns whether timestamps should be shown
func (rl *ResponsiveLayout) ShouldShowTimestamps() bool {
	return rl.showTimestamps
}

// GetMaxItems returns the maximum number of items to display
func (rl *ResponsiveLayout) GetMaxItems() int {
	return rl.maxItems
}

// GetTruncateLength returns the length at which to truncate text
func (rl *ResponsiveLayout) GetTruncateLength() int {
	return rl.truncateLength
}

// GetContentWidth returns the available width for content
func (rl *ResponsiveLayout) GetContentWidth() int {
	width := rl.viewport.Width
	
	// Subtract sidebar width
	if rl.config.ShowSidebar {
		width -= rl.sidebarWidth + 1 // +1 for border
	}
	
	// Subtract padding and margins
	width -= (rl.config.Padding + rl.config.Margin) * 2
	
	if width < 10 {
		width = 10 // Minimum content width
	}
	
	return width
}

// GetContentHeight returns the available height for content
func (rl *ResponsiveLayout) GetContentHeight() int {
	height := rl.viewport.Height
	
	// Subtract header and footer
	height -= rl.headerHeight + rl.footerHeight
	
	// Subtract status bar if shown
	if rl.config.ShowStatusBar {
		height -= 1
	}
	
	// Subtract breadcrumb if shown
	if rl.config.ShowBreadcrumb {
		height -= 1
	}
	
	// Subtract padding and margins
	height -= (rl.config.Padding + rl.config.Margin) * 2
	
	if height < 5 {
		height = 5 // Minimum content height
	}
	
	return height
}

// TruncateText truncates text based on current layout settings
func (rl *ResponsiveLayout) TruncateText(text string) string {
	if len(text) <= rl.truncateLength {
		return text
	}
	return text[:rl.truncateLength-3] + "..."
}

// FormatText formats text based on current layout settings
func (rl *ResponsiveLayout) FormatText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = rl.GetContentWidth()
	}
	
	if len(text) <= maxWidth {
		return text
	}
	
	if maxWidth <= 3 {
		return "..."
	}
	
	return text[:maxWidth-3] + "..."
}

// CreateColumns creates a multi-column layout
func (rl *ResponsiveLayout) CreateColumns(items []string) string {
	if len(items) == 0 {
		return ""
	}
	
	columnCount := rl.config.ColumnCount
	if columnCount <= 1 {
		return strings.Join(items, "\n")
	}
	
	contentWidth := rl.GetContentWidth()
	columnWidth := (contentWidth - (columnCount - 1)) / columnCount // -1 for separators
	
	if columnWidth < 10 {
		// Fall back to single column if columns would be too narrow
		return strings.Join(items, "\n")
	}
	
	var result strings.Builder
	rows := (len(items) + columnCount - 1) / columnCount // Ceiling division
	
	for row := 0; row < rows; row++ {
		var columns []string
		for col := 0; col < columnCount; col++ {
			index := col*rows + row
			if index < len(items) {
				item := rl.FormatText(items[index], columnWidth)
				columns = append(columns, fmt.Sprintf("%-*s", columnWidth, item))
			} else {
				columns = append(columns, strings.Repeat(" ", columnWidth))
			}
		}
		result.WriteString(strings.Join(columns, " "))
		if row < rows-1 {
			result.WriteString("\n")
		}
	}
	
	return result.String()
}

// CreateGrid creates a grid layout for items
func (rl *ResponsiveLayout) CreateGrid(items []GridItem, itemWidth, itemHeight int) string {
	if len(items) == 0 {
		return ""
	}
	
	contentWidth := rl.GetContentWidth()
	contentHeight := rl.GetContentHeight()
	
	// Calculate how many items fit per row
	itemsPerRow := contentWidth / (itemWidth + 1) // +1 for spacing
	if itemsPerRow < 1 {
		itemsPerRow = 1
	}
	
	// Calculate how many rows we can display
	maxRows := contentHeight / (itemHeight + 1) // +1 for spacing
	if maxRows < 1 {
		maxRows = 1
	}
	
	maxItems := itemsPerRow * maxRows
	if len(items) > maxItems {
		items = items[:maxItems]
	}
	
	var result strings.Builder
	for i, item := range items {
		if i > 0 && i%itemsPerRow == 0 {
			result.WriteString("\n")
			// Add vertical spacing
			for j := 0; j < itemHeight; j++ {
				result.WriteString("\n")
			}
		} else if i > 0 {
			result.WriteString(" ") // Horizontal spacing
		}
		
		result.WriteString(item.Render(itemWidth, itemHeight))
	}
	
	return result.String()
}

// GridItem represents an item that can be rendered in a grid
type GridItem interface {
	Render(width, height int) string
}

// AdaptiveContainer creates a container that adapts to the current layout
func (rl *ResponsiveLayout) AdaptiveContainer(content string, title string) string {
	if rl.themeManager == nil {
		return content
	}
	
	styles := rl.themeManager.GetStyles()
	if styles == nil {
		return content
	}
	
	containerStyle := styles.Card
	
	// Adjust container based on layout
	if rl.config.CompactMode {
		containerStyle = containerStyle.Padding(0).Margin(0)
	} else {
		containerStyle = containerStyle.
			Padding(rl.config.Padding).
			Margin(rl.config.Margin)
	}
	
	// Set width
	contentWidth := rl.GetContentWidth()
	containerStyle = containerStyle.Width(contentWidth)
	
	// Add title if provided and space allows
	if title != "" && !rl.config.CompactMode {
		titleStyle := styles.Header
		titleContent := titleStyle.Render(title)
		content = titleContent + "\n" + content
	}
	
	return containerStyle.Render(content)
}

// GetBreakpointName returns the name of the current breakpoint
func (rl *ResponsiveLayout) GetBreakpointName() string {
	switch rl.breakpoint {
	case BreakpointXSmall:
		return "xs"
	case BreakpointSmall:
		return "sm"
	case BreakpointMedium:
		return "md"
	case BreakpointLarge:
		return "lg"
	case BreakpointXLarge:
		return "xl"
	default:
		return "unknown"
	}
}

// GetLayoutInfo returns information about the current layout
func (rl *ResponsiveLayout) GetLayoutInfo() map[string]interface{} {
	return map[string]interface{}{
		"viewport_width":    rl.viewport.Width,
		"viewport_height":   rl.viewport.Height,
		"breakpoint":        rl.GetBreakpointName(),
		"compact_mode":      rl.config.CompactMode,
		"show_sidebar":      rl.config.ShowSidebar,
		"show_details":      rl.showDetails,
		"show_icons":        rl.showIcons,
		"show_timestamps":   rl.showTimestamps,
		"content_width":     rl.GetContentWidth(),
		"content_height":    rl.GetContentHeight(),
		"column_count":      rl.config.ColumnCount,
		"max_items":         rl.maxItems,
		"truncate_length":   rl.truncateLength,
	}
}
