package style

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// --- Reusable Colors ---
var (
	colorPink      = lipgloss.Color("205")
	colorDarkGray  = lipgloss.Color("240")
	colorLightGray = lipgloss.Color("229")
	colorBlue      = lipgloss.Color("57")
	colorCyan      = lipgloss.Color("212")
	colorPurple    = lipgloss.Color("99")
	colorRed       = lipgloss.Color("196")
	colorGreen     = lipgloss.Color("42")
)

// --- General Purpose Styles ---
var (
	ErrorStyle   = lipgloss.NewStyle().Foreground(colorRed)
	SuccessStyle = lipgloss.NewStyle().Foreground(colorGreen)
)

var (
	SafeReset = "\x1b[39m\x1b[22m\x1b[23m\x1b[24m\x1b[25m\x1b[27m\x1b[28m\x1b[29m"
)

var (
	BaseStyle          = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(colorDarkGray)
	HighlightFontStyle = lipgloss.NewStyle().Foreground(colorCyan)
	ThemeStyle         = lipgloss.NewStyle().Foreground(colorPurple)
)

// --- File Tree And Multi File Picker Styles ---
var (
	DocStyle        = lipgloss.NewStyle().Margin(1, 2)
	TitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(colorPink)
	CursorStyle     = lipgloss.NewStyle().Foreground(colorCyan).SetString("> ")
	NoCursorStyle   = lipgloss.NewStyle().SetString("  ")
	DirStyle        = lipgloss.NewStyle().Foreground(colorPurple)
	FileStyle       = lipgloss.NewStyle().Foreground(colorLightGray)
	HelpStyle       = lipgloss.NewStyle().Foreground(colorCyan).Faint(true)
	HeaderStyle     = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	SelectedStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorGreen).SetString("[x] ")
	DeselectedStyle = lipgloss.NewStyle().SetString("[ ] ")
)

// --- Common Components ---

// NewSpinner creates a spinner with a consistent style.
func NewSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle()
	return s
}

// NewTableStyles returns the default styles for tables, with our custom selection style.
func NewTableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Selected = styles.Selected.Foreground(colorLightGray).Background(colorBlue).Bold(false)
	return styles
}

// renderWithSafeReset renders text with a style but uses a safe reset that preserves background color
func RenderWithSafeReset(style lipgloss.Style, text string) string {
	// Get the styled text
	styled := style.Render(text)

	// Replace the full reset \x1b[0m with a selective reset that preserves background
	// \x1b[39m resets foreground color to default
	// \x1b[22m resets bold/faint
	// \x1b[23m resets italic
	// \x1b[24m resets underline
	// \x1b[25m resets blink
	// \x1b[27m resets reverse
	// \x1b[28m resets hidden
	// \x1b[29m resets strikethrough
	
	// Replace all occurrences of the full reset with safe reset
	result := strings.ReplaceAll(styled, "\x1b[0m", SafeReset)

	return result
}
// SafeRenderStyle is a wrapper around lipgloss.Style that provides safe rendering

// CreateSafeCursorStyle creates a cursor style that won't reset background colors
// This is specifically designed for textinput components with background colors
func CreateSafeCursorStyle() lipgloss.Style {
	// Create a style that only sets foreground color, avoiding any reset sequences
	return lipgloss.NewStyle().
		Foreground(colorCyan).
		Inline(true) // This helps prevent unwanted resets
}
