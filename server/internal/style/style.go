package style

import (
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
)


// --- General Purpose Styles ---
var (
	ErrorStyle = lipgloss.NewStyle().Foreground(colorRed)
)


// --- Sender Styles ---
var (
	BaseStyle          = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(colorDarkGray)
	HighlightFontStyle = lipgloss.NewStyle().Foreground(colorCyan)
)

// --- File Tree Styles ---
var (
	DocStyle      = lipgloss.NewStyle().Margin(1, 2)
	TitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorPink)
	CursorStyle   = lipgloss.NewStyle().Foreground(colorCyan).SetString("> ")
	NoCursorStyle = lipgloss.NewStyle().SetString("  ")
	DirStyle      = lipgloss.NewStyle().Foreground(colorPurple)
	FileStyle     = lipgloss.NewStyle().Foreground(colorLightGray)
	HelpStyle     = lipgloss.NewStyle().Faint(true)
	HeaderStyle   = lipgloss.NewStyle().Bold(true).Padding(0, 1)
)


// --- Common Components ---

// NewSpinner creates a spinner with a consistent style.
func NewSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorPink)
	return s
}

// NewTableStyles returns the default styles for tables, with our custom selection style.
func NewTableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Selected = styles.Selected.Foreground(colorLightGray).Background(colorBlue).Bold(false)
	return styles
}
