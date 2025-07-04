package multiFilePicker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeBrowse mode = iota
	modeInput
)

// --- Key Map ---
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	ToggleSelect key.Binding
	ToggleInput  key.Binding
	Confirm      key.Binding
	Quit         key.Binding
}

var DefaultKeyMap = KeyMap{
	Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
	Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
	ToggleSelect: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle select")),
	ToggleInput:  key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "input path")),
	Confirm:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Quit:         key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc/ctrl+c", "quit/back")),
}

// --- Model ---
type Model struct {
	path     string
	items    []fs.DirEntry
	selected map[string]struct{}
	cursor   int
	keys     KeyMap
	quitting bool
	choice   []string
	mode     mode
	input    textinput.Model
	inputErr error
	height   int // For viewport height
	offset   int // For scrolling
}

func InitialModel() Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))

	return Model{
		path:     "",              // Initially empty
		items:    []fs.DirEntry{}, // Initially empty
		selected: make(map[string]struct{}),
		keys:     DefaultKeyMap,
		mode:     modeInput, // Start in input mode
		input:    ti,
	}
}

// --- Bubble Tea Methods ---
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global quit
		if key.Matches(msg, m.keys.Quit) {
			if m.mode == modeInput {
				// If we're in input mode and haven't loaded a path yet, quit.
				if m.path == "" {
					m.quitting = true
					return m, tea.Quit
				}
				// Otherwise, go back to browsing the currently loaded path.
				m.mode = modeBrowse
				m.input.Blur()
				m.input.Reset()
				m.inputErr = nil
				return m, nil
			}
			// If in browse mode, quit.
			m.quitting = true
			return m, tea.Quit
		}

		// Mode-specific updates
		switch m.mode {
		case modeBrowse:
			return m.updateBrowse(msg)
		case modeInput:
			model, cmd := m.updateInput(msg)
			if updated, ok := model.(*Model); ok {
				return *updated, cmd
			}
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.ToggleInput):
		m.mode = modeInput
		m.input.Focus()
		return m, textinput.Blink

	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
			// Scroll up if cursor is at the top of the viewport
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}

	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.items)-1 {
			m.cursor++
			// Scroll down if cursor is at the bottom of the viewport
			// The number of visible items is roughly m.height - header lines
			visibleItems := m.height - 10 // Adjust this based on your header's height
			if visibleItems < 1 {
				visibleItems = 1
			}
			if m.cursor >= m.offset+visibleItems {
				m.offset = m.cursor - visibleItems + 1
			}
		}

	case key.Matches(msg, m.keys.ToggleSelect):
		item := m.items[m.cursor]
		path := filepath.Join(m.path, item.Name())
		if _, ok := m.selected[path]; ok {
			delete(m.selected, path)
		} else {
			m.selected[path] = struct{}{}
		}

	case key.Matches(msg, m.keys.Confirm):
		if len(m.selected) > 0 {
			m.choice = getSelectedPaths(m.selected)
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if key.Matches(msg, m.keys.Confirm) {
		path := m.input.Value()
		absPath, err := filepath.Abs(path)
		if err != nil {
			m.inputErr = fmt.Errorf("invalid path: %w", err)
			return m, nil
		}

		info, err := os.Stat(absPath)
		if err != nil {
			m.inputErr = fmt.Errorf("path does not exist: %s", absPath)
			return m, nil
		}
		if !info.IsDir() {
			m.inputErr = fmt.Errorf("path is not a directory: %s", absPath)
			return m, nil
		}

		// Path is a valid directory, load its contents
		items, err := os.ReadDir(absPath)
		if err != nil {
			m.inputErr = fmt.Errorf("could not read directory: %w", err)
			return m, nil
		}

		m.path = absPath
		m.items = items
		m.mode = modeBrowse
		m.input.Reset()
		m.inputErr = nil
		m.cursor = 0 // Reset cursor
		m.offset = 0 // Reset scroll offset
		return m, nil
	}

	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		if len(m.choice) > 0 {
			return fmt.Sprintf("Selected:\n%s\n", strings.Join(m.choice, "\n"))
		}
		return "No selection. Bye!\n"
	}

	var s strings.Builder

	// Header
	s.WriteString("Enter a path to browse, or select files below. " + m.helpView() + "\n \n")
	s.WriteString(m.input.View())
	if m.inputErr != nil {
		s.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.inputErr.Error()))
	}
	s.WriteString("\n\n")

	if m.path != "" {
		s.WriteString(fmt.Sprintf("Browsing: %s\n", m.path))
	}
	s.WriteString("Select files and folders:\n")

	// Viewport logic
	headerHeight := 6 // Approximate number of lines in the header
	if m.inputErr != nil {
		headerHeight++
	}
	visibleItems := m.height - headerHeight
	if visibleItems < 1 {
		visibleItems = 20 // Default if height is not yet set
	}


	start := m.offset
	end := m.offset + visibleItems
	if end > len(m.items) {
		end = len(m.items)
	}

	// Render visible items
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).SetString("> ")
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("[x] ")
	deselectedStyle := lipgloss.NewStyle().SetString("[ ] ")
	dirStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99"))

	// Ensure we don't render a slice with a negative start index
	if start < 0 {
		start = 0
	}

	slice := m.items
	if start < len(m.items) {
		slice = m.items[start:end]
	} else if len(m.items) == 0 {
		slice = []fs.DirEntry{}
	}

	for i, item := range slice {
		actualIndex := start + i
		if m.cursor == actualIndex {
			s.WriteString(cursorStyle.String())
		} else {
			s.WriteString("  ")
		}

		path := filepath.Join(m.path, item.Name())
		if _, ok := m.selected[path]; ok {
			s.WriteString(selectedStyle.String())
		} else {
			s.WriteString(deselectedStyle.String())
		}

		if item.IsDir() {
			s.WriteString(dirStyle.Render(item.Name() + "/"))
		} else {
			s.WriteString(item.Name())
		}
		s.WriteString("\n")
	}

	// Scroll indicator
	if len(m.items) > visibleItems {
		s.WriteString(fmt.Sprintf("... %d/%d ...\n", m.cursor+1, len(m.items)))
	}

	return s.String()
}

func (m Model) helpView() string {
	return lipgloss.NewStyle().Faint(true).Render(
		fmt.Sprintf("'%s' to browse, '%s' to confirm, '%s' to quit",
			m.keys.ToggleInput.Help().Key, m.keys.Confirm.Help().Key, m.keys.Quit.Help().Key),
	)
}

func getSelectedPaths(selection map[string]struct{}) []string {
	var paths []string
	for path := range selection {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					paths = append(paths, p)
				}
				return nil
			})
		} else if err == nil && !info.IsDir() {
			paths = append(paths, path)
		}
	}
	return paths
}

// Run starts the file picker and returns the selected file paths.
func Run() ([]string, error) {
	p := tea.NewProgram(InitialModel())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running program: %w", err)
	}

	m := finalModel.(Model)
	// If user quits without confirming, choice might be empty.
	// If they confirmed via browser, we need to populate it now.
	if len(m.choice) == 0 && len(m.selected) > 0 {
		m.choice = getSelectedPaths(m.selected)
	}

	return m.choice, nil
}
