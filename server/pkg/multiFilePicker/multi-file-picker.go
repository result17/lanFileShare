package multiFilePicker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Key Map ---
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	ToggleSelect key.Binding
	Confirm      key.Binding
	Quit         key.Binding
}

var DefaultKeyMap = KeyMap{
	Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
	Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
	ToggleSelect: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle select")),
	Confirm:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm selection")),
	Quit:         key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q/esc", "quit")),
}

// --- Model ---
type Model struct {
	// The directory we are listing
	path string
	// The files and directories in the current path
	items []fs.DirEntry
	// A map of selected file/dir paths
	selected map[string]struct{}
	// Index of the cursor
	cursor int
	// Key bindings
	keys KeyMap
	// Quitting status
	quitting bool
	// Final choice
	choice []string
}

func initialModel(path string) Model {
	// Read the directory
	items, err := os.ReadDir(path)
	if err != nil {
		// Handle error appropriately in a real app
		panic(err)
	}

	return Model{
		path:     path,
		items:    items,
		selected: make(map[string]struct{}),
		keys:     DefaultKeyMap,
	}
}

// --- Bubble Tea Methods ---
func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.ToggleSelect):
			// Toggle selection for the item under the cursor
			item := m.items[m.cursor]
			path := filepath.Join(m.path, item.Name())

			if _, ok := m.selected[path]; ok {
				// It's selected, so deselect it
				delete(m.selected, path)
			} else {
				// It's not selected, so select it
				m.selected[path] = struct{}{}
			}

		case key.Matches(msg, m.keys.Confirm):
			// Get all selected paths
			var selectedPaths []string
			for path := range m.selected {
				// If a directory is selected, walk it and add all files
				info, err := os.Stat(path)
				if err == nil && info.IsDir() {
					filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
						if err == nil && !d.IsDir() {
							selectedPaths = append(selectedPaths, p)
						}
						return nil
					})
				} else if err == nil && !info.IsDir() {
					selectedPaths = append(selectedPaths, path)
				}
			}
			m.choice = selectedPaths
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		if len(m.choice) > 0 {
			return fmt.Sprintf("Selected files:\n%s\n", strings.Join(m.choice, "\n"))
		}
		return "No files selected. Bye!\n"
	}

	var s strings.Builder
	s.WriteString("Select files and folders (space to toggle, enter to confirm):\n\n")

	// Styles
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).SetString("> ")
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("[x] ")
	deselectedStyle := lipgloss.NewStyle().SetString("[ ] ")
	dirStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99"))

	for i, item := range m.items {
		// Cursor
		if m.cursor == i {
			s.WriteString(cursorStyle.String())
		} else {
			s.WriteString("  ")
		}

		// Selection status
		path := filepath.Join(m.path, item.Name())
		if _, ok := m.selected[path]; ok {
			s.WriteString(selectedStyle.String())
		} else {
			s.WriteString(deselectedStyle.String())
		}

		// File/Dir Name
		if item.IsDir() {
			s.WriteString(dirStyle.Render(item.Name() + "/"))
		} else {
			s.WriteString(item.Name())
		}
		s.WriteString("\n")
	}

	s.WriteString("\n" + m.helpView())
	return s.String()
}

func (m Model) helpView() string {
	helps := []string{
		m.keys.Up.Help().Key + " " + m.keys.Up.Help().Desc,
		m.keys.Down.Help().Key + " " + m.keys.Down.Help().Desc,
		m.keys.ToggleSelect.Help().Key + " " + m.keys.ToggleSelect.Help().Desc,
		m.keys.Confirm.Help().Key + " " + m.keys.Confirm.Help().Desc,
		m.keys.Quit.Help().Key + " " + m.keys.Quit.Help().Desc,
	}
	return "\n" + strings.Join(helps, "  ")
}

// Run starts the file picker and returns the selected file paths.
func Run(startDir string) ([]string, error) {
	p := tea.NewProgram(initialModel(startDir))
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running program: %w", err)
	}

	// The final model is returned from p.Run()
	m := finalModel.(Model)
	return m.choice, nil
}
