package components

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ColorScheme represents a color scheme for the application
type ColorScheme struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	// Primary colors
	Primary    string `json:"primary"`
	Secondary  string `json:"secondary"`
	Accent     string `json:"accent"`
	Background string `json:"background"`
	Surface    string `json:"surface"`

	// Text colors
	TextPrimary   string `json:"text_primary"`
	TextSecondary string `json:"text_secondary"`
	TextMuted     string `json:"text_muted"`

	// Status colors
	Success string `json:"success"`
	Warning string `json:"warning"`
	Error   string `json:"error"`
	Info    string `json:"info"`

	// UI element colors
	Border    string `json:"border"`
	Highlight string `json:"highlight"`
	Selection string `json:"selection"`
	Progress  string `json:"progress"`
}

// Theme represents a complete theme configuration
type Theme struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	ColorScheme ColorScheme `json:"color_scheme"`
	DarkMode    bool        `json:"dark_mode"`

	// Typography
	FontFamily string  `json:"font_family"`
	FontSize   int     `json:"font_size"`
	LineHeight float64 `json:"line_height"`

	// Layout
	Padding      int  `json:"padding"`
	Margin       int  `json:"margin"`
	BorderRadius int  `json:"border_radius"`
	CompactMode  bool `json:"compact_mode"`

	// Animation
	AnimationEnabled bool `json:"animation_enabled"`
	AnimationSpeed   int  `json:"animation_speed"` // milliseconds
}

// ThemeManager manages themes and provides styling utilities
type ThemeManager struct {
	currentTheme    *Theme
	availableThemes map[string]*Theme
	configDir       string
	styles          *ThemeStyles
}

// ThemeStyles contains pre-computed lipgloss styles for the current theme
type ThemeStyles struct {
	// Base styles
	Base       lipgloss.Style
	Background lipgloss.Style
	Surface    lipgloss.Style

	// Text styles
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Body     lipgloss.Style
	Caption  lipgloss.Style
	Muted    lipgloss.Style

	// Status styles
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Info    lipgloss.Style

	// UI element styles
	Button      lipgloss.Style
	ButtonHover lipgloss.Style
	Input       lipgloss.Style
	Border      lipgloss.Style
	Highlight   lipgloss.Style
	Selection   lipgloss.Style
	Progress    lipgloss.Style

	// Component styles
	Header  lipgloss.Style
	Footer  lipgloss.Style
	Sidebar lipgloss.Style
	Card    lipgloss.Style
	Badge   lipgloss.Style
}

// NewThemeManager creates a new theme manager
func NewThemeManager(configDir string) *ThemeManager {
	tm := &ThemeManager{
		availableThemes: make(map[string]*Theme),
		configDir:       configDir,
	}

	// Initialize built-in themes
	tm.initializeBuiltinThemes()

	// Load custom themes from config directory
	tm.loadCustomThemes()

	// Set default theme
	tm.SetTheme("default")

	return tm
}

// initializeBuiltinThemes creates the built-in themes
func (tm *ThemeManager) initializeBuiltinThemes() {
	// Default theme (dark)
	defaultTheme := &Theme{
		Name:        "default",
		Description: "Default dark theme",
		DarkMode:    true,
		ColorScheme: ColorScheme{
			Name:          "Default Dark",
			Description:   "Default dark color scheme",
			Primary:       "#58a6ff",
			Secondary:     "#8b949e",
			Accent:        "#f2cc60",
			Background:    "#0d1117",
			Surface:       "#21262d",
			TextPrimary:   "#c9d1d9",
			TextSecondary: "#8b949e",
			TextMuted:     "#6e7681",
			Success:       "#3fb950",
			Warning:       "#d29922",
			Error:         "#f85149",
			Info:          "#58a6ff",
			Border:        "#30363d",
			Highlight:     "#388bfd26",
			Selection:     "#388bfd26",
			Progress:      "#58a6ff",
		},
		FontFamily:       "monospace",
		FontSize:         14,
		LineHeight:       1.4,
		Padding:          1,
		Margin:           1,
		BorderRadius:     0,
		CompactMode:      false,
		AnimationEnabled: true,
		AnimationSpeed:   200,
	}

	// Light theme
	lightTheme := &Theme{
		Name:        "light",
		Description: "Light theme",
		DarkMode:    false,
		ColorScheme: ColorScheme{
			Name:          "Light",
			Description:   "Light color scheme",
			Primary:       "#0969da",
			Secondary:     "#656d76",
			Accent:        "#bf8700",
			Background:    "#ffffff",
			Surface:       "#f6f8fa",
			TextPrimary:   "#24292f",
			TextSecondary: "#656d76",
			TextMuted:     "#8c959f",
			Success:       "#1a7f37",
			Warning:       "#9a6700",
			Error:         "#cf222e",
			Info:          "#0969da",
			Border:        "#d0d7de",
			Highlight:     "#ddf4ff",
			Selection:     "#ddf4ff",
			Progress:      "#0969da",
		},
		FontFamily:       "monospace",
		FontSize:         14,
		LineHeight:       1.4,
		Padding:          1,
		Margin:           1,
		BorderRadius:     0,
		CompactMode:      false,
		AnimationEnabled: true,
		AnimationSpeed:   200,
	}

	// High contrast theme
	highContrastTheme := &Theme{
		Name:        "high-contrast",
		Description: "High contrast theme for accessibility",
		DarkMode:    true,
		ColorScheme: ColorScheme{
			Name:          "High Contrast",
			Description:   "High contrast color scheme",
			Primary:       "#ffffff",
			Secondary:     "#c0c0c0",
			Accent:        "#ffff00",
			Background:    "#000000",
			Surface:       "#1a1a1a",
			TextPrimary:   "#ffffff",
			TextSecondary: "#c0c0c0",
			TextMuted:     "#808080",
			Success:       "#00ff00",
			Warning:       "#ffff00",
			Error:         "#ff0000",
			Info:          "#00ffff",
			Border:        "#ffffff",
			Highlight:     "#ffff00",
			Selection:     "#0000ff",
			Progress:      "#00ff00",
		},
		FontFamily:       "monospace",
		FontSize:         14,
		LineHeight:       1.6,
		Padding:          2,
		Margin:           2,
		BorderRadius:     0,
		CompactMode:      false,
		AnimationEnabled: false,
		AnimationSpeed:   0,
	}

	// Compact theme
	compactTheme := &Theme{
		Name:        "compact",
		Description: "Compact theme for small terminals",
		DarkMode:    true,
		ColorScheme: ColorScheme{
			Name:          "Compact Dark",
			Description:   "Compact dark color scheme",
			Primary:       "#58a6ff",
			Secondary:     "#8b949e",
			Accent:        "#f2cc60",
			Background:    "#0d1117",
			Surface:       "#21262d",
			TextPrimary:   "#c9d1d9",
			TextSecondary: "#8b949e",
			TextMuted:     "#6e7681",
			Success:       "#3fb950",
			Warning:       "#d29922",
			Error:         "#f85149",
			Info:          "#58a6ff",
			Border:        "#30363d",
			Highlight:     "#388bfd26",
			Selection:     "#388bfd26",
			Progress:      "#58a6ff",
		},
		FontFamily:       "monospace",
		FontSize:         12,
		LineHeight:       1.2,
		Padding:          0,
		Margin:           0,
		BorderRadius:     0,
		CompactMode:      true,
		AnimationEnabled: false,
		AnimationSpeed:   0,
	}

	tm.availableThemes["default"] = defaultTheme
	tm.availableThemes["light"] = lightTheme
	tm.availableThemes["high-contrast"] = highContrastTheme
	tm.availableThemes["compact"] = compactTheme
}

// loadCustomThemes loads custom themes from the config directory
func (tm *ThemeManager) loadCustomThemes() {
	if tm.configDir == "" {
		return
	}

	themesDir := filepath.Join(tm.configDir, "themes")
	if _, err := os.Stat(themesDir); os.IsNotExist(err) {
		return
	}

	files, err := os.ReadDir(themesDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			themePath := filepath.Join(themesDir, file.Name())
			if theme := tm.loadThemeFromFile(themePath); theme != nil {
				tm.availableThemes[theme.Name] = theme
			}
		}
	}
}

// loadThemeFromFile loads a theme from a JSON file
func (tm *ThemeManager) loadThemeFromFile(path string) *Theme {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return nil
	}

	return &theme
}

// SetTheme sets the current theme
func (tm *ThemeManager) SetTheme(name string) error {
	theme, exists := tm.availableThemes[name]
	if !exists {
		return fmt.Errorf("theme '%s' not found", name)
	}

	tm.currentTheme = theme
	tm.updateStyles()
	return nil
}

// GetCurrentTheme returns the current theme
func (tm *ThemeManager) GetCurrentTheme() *Theme {
	return tm.currentTheme
}

// GetAvailableThemes returns all available themes
func (tm *ThemeManager) GetAvailableThemes() map[string]*Theme {
	return tm.availableThemes
}

// GetStyles returns the current theme styles
func (tm *ThemeManager) GetStyles() *ThemeStyles {
	return tm.styles
}

// updateStyles updates the lipgloss styles based on the current theme
func (tm *ThemeManager) updateStyles() {
	if tm.currentTheme == nil {
		return
	}

	cs := tm.currentTheme.ColorScheme

	tm.styles = &ThemeStyles{
		// Base styles
		Base: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextPrimary)).
			Background(lipgloss.Color(cs.Background)),

		Background: lipgloss.NewStyle().
			Background(lipgloss.Color(cs.Background)),

		Surface: lipgloss.NewStyle().
			Background(lipgloss.Color(cs.Surface)).
			Foreground(lipgloss.Color(cs.TextPrimary)),

		// Text styles
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Primary)).
			Bold(true),

		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Secondary)).
			Bold(true),

		Body: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextPrimary)),

		Caption: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextSecondary)),

		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextMuted)),

		// Status styles
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Success)),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Warning)),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Error)),

		Info: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Info)),

		// UI element styles
		Button: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextPrimary)).
			Background(lipgloss.Color(cs.Primary)).
			Padding(0, 2),

		ButtonHover: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Background)).
			Background(lipgloss.Color(cs.Accent)).
			Padding(0, 2),

		Input: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextPrimary)).
			Background(lipgloss.Color(cs.Surface)).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(cs.Border)),

		Border: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(cs.Border)),

		Highlight: lipgloss.NewStyle().
			Background(lipgloss.Color(cs.Highlight)).
			Foreground(lipgloss.Color(cs.TextPrimary)),

		Selection: lipgloss.NewStyle().
			Background(lipgloss.Color(cs.Selection)).
			Foreground(lipgloss.Color(cs.TextPrimary)),

		Progress: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Progress)),

		// Component styles
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Primary)).
			Background(lipgloss.Color(cs.Surface)).
			Bold(true).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextSecondary)).
			Background(lipgloss.Color(cs.Surface)).
			Padding(0, 1),

		Sidebar: lipgloss.NewStyle().
			Background(lipgloss.Color(cs.Surface)).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(cs.Border)),

		Card: lipgloss.NewStyle().
			Background(lipgloss.Color(cs.Surface)).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(cs.Border)).
			Padding(1),

		Badge: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Background)).
			Background(lipgloss.Color(cs.Accent)).
			Padding(0, 1),
	}
}

// SaveTheme saves a theme to the config directory
func (tm *ThemeManager) SaveTheme(theme *Theme) error {
	if tm.configDir == "" {
		return fmt.Errorf("config directory not set")
	}

	themesDir := filepath.Join(tm.configDir, "themes")
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.json", theme.Name)
	path := filepath.Join(themesDir, filename)

	return os.WriteFile(path, data, 0644)
}

// CreateCustomTheme creates a new custom theme based on an existing theme
func (tm *ThemeManager) CreateCustomTheme(baseName, newName, description string) (*Theme, error) {
	baseTheme, exists := tm.availableThemes[baseName]
	if !exists {
		return nil, fmt.Errorf("base theme '%s' not found", baseName)
	}

	// Create a copy of the base theme
	customTheme := *baseTheme
	customTheme.Name = newName
	customTheme.Description = description

	// Add to available themes
	tm.availableThemes[newName] = &customTheme

	return &customTheme, nil
}

// ThemeSelector provides a UI for selecting themes
type ThemeSelector struct {
	themeManager  *ThemeManager
	themes        []string
	selectedIndex int
	visible       bool
	previewMode   bool
	originalTheme string
}

// NewThemeSelector creates a new theme selector
func NewThemeSelector(themeManager *ThemeManager) *ThemeSelector {
	ts := &ThemeSelector{
		themeManager:  themeManager,
		themes:        make([]string, 0),
		selectedIndex: 0,
		visible:       false,
		previewMode:   false,
	}

	// Populate theme list
	for name := range themeManager.GetAvailableThemes() {
		ts.themes = append(ts.themes, name)
	}

	// Set current theme as selected
	currentTheme := themeManager.GetCurrentTheme()
	if currentTheme != nil {
		for i, name := range ts.themes {
			if name == currentTheme.Name {
				ts.selectedIndex = i
				break
			}
		}
	}

	return ts
}

// Show shows the theme selector
func (ts *ThemeSelector) Show() {
	ts.visible = true
	ts.originalTheme = ts.themeManager.GetCurrentTheme().Name
}

// Hide hides the theme selector
func (ts *ThemeSelector) Hide() {
	ts.visible = false
	if ts.previewMode {
		// Restore original theme
		ts.themeManager.SetTheme(ts.originalTheme)
		ts.previewMode = false
	}
}

// IsVisible returns whether the theme selector is visible
func (ts *ThemeSelector) IsVisible() bool {
	return ts.visible
}

// Navigate handles navigation within the theme selector
func (ts *ThemeSelector) Navigate(action KeyAction) bool {
	if !ts.visible {
		return false
	}

	switch action {
	case KeyActionNavigateUp:
		if ts.selectedIndex > 0 {
			ts.selectedIndex--
			if ts.previewMode {
				ts.previewTheme()
			}
			return true
		}
	case KeyActionNavigateDown:
		if ts.selectedIndex < len(ts.themes)-1 {
			ts.selectedIndex++
			if ts.previewMode {
				ts.previewTheme()
			}
			return true
		}
	case KeyActionSelect:
		// Apply selected theme
		ts.applyTheme()
		ts.Hide()
		return true
	case KeyActionToggleMode:
		// Toggle preview mode
		ts.previewMode = !ts.previewMode
		if ts.previewMode {
			ts.previewTheme()
		} else {
			ts.themeManager.SetTheme(ts.originalTheme)
		}
		return true
	case KeyActionCancel:
		ts.Hide()
		return true
	}

	return false
}

// previewTheme previews the currently selected theme
func (ts *ThemeSelector) previewTheme() {
	if ts.selectedIndex >= 0 && ts.selectedIndex < len(ts.themes) {
		themeName := ts.themes[ts.selectedIndex]
		ts.themeManager.SetTheme(themeName)
	}
}

// applyTheme applies the currently selected theme
func (ts *ThemeSelector) applyTheme() {
	if ts.selectedIndex >= 0 && ts.selectedIndex < len(ts.themes) {
		themeName := ts.themes[ts.selectedIndex]
		ts.themeManager.SetTheme(themeName)
		ts.originalTheme = themeName
	}
}

// GetSelectedTheme returns the currently selected theme name
func (ts *ThemeSelector) GetSelectedTheme() string {
	if ts.selectedIndex >= 0 && ts.selectedIndex < len(ts.themes) {
		return ts.themes[ts.selectedIndex]
	}
	return ""
}

// Render renders the theme selector
func (ts *ThemeSelector) Render() string {
	if !ts.visible {
		return ""
	}

	styles := ts.themeManager.GetStyles()
	if styles == nil {
		return "Theme selector not available"
	}

	var result strings.Builder

	// Title
	title := "🎨 Select Theme"
	if ts.previewMode {
		title += " (Preview Mode)"
	}
	result.WriteString(styles.Header.Render(title))
	result.WriteString("\n\n")

	// Theme list
	for i, themeName := range ts.themes {
		theme := ts.themeManager.availableThemes[themeName]
		if theme == nil {
			continue
		}

		var line strings.Builder

		// Selection indicator
		if i == ts.selectedIndex {
			line.WriteString("▶ ")
		} else {
			line.WriteString("  ")
		}

		// Theme name
		line.WriteString(theme.Name)

		// Theme description
		if theme.Description != "" {
			line.WriteString(" - ")
			line.WriteString(theme.Description)
		}

		// Current theme indicator
		currentTheme := ts.themeManager.GetCurrentTheme()
		if currentTheme != nil && currentTheme.Name == theme.Name {
			line.WriteString(" (current)")
		}

		// Apply styling
		if i == ts.selectedIndex {
			result.WriteString(styles.Selection.Render(line.String()))
		} else {
			result.WriteString(styles.Body.Render(line.String()))
		}
		result.WriteString("\n")
	}

	// Instructions
	result.WriteString("\n")
	instructions := []string{
		"↑/↓ Navigate",
		"Enter Apply",
		"Space Preview",
		"Esc Cancel",
	}
	result.WriteString(styles.Caption.Render(strings.Join(instructions, " | ")))

	// Wrap in container
	container := styles.Card.Render(result.String())

	return container
}
