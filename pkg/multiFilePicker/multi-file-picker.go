package multiFilePicker

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gabriel-vasile/mimetype"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/internal/util"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

type mode int
type SelectedFileNodeMsg struct {
	Files []fileInfo.FileNode
}

const (
	modeBrowse mode = iota
	modeInput
)

// --- Key Map ---
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Left         key.Binding // Page up
	Right        key.Binding // Page down
	ToggleSelect key.Binding
	ToggleInput  key.Binding
	Confirm      key.Binding
	Quit         key.Binding
}

var DefaultKeyMap = KeyMap{
	Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
	Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
	Left:         key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "page up")),
	Right:        key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "page down")),
	ToggleSelect: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle select")),
	ToggleInput:  key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "input path")),
	Confirm:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Quit:         key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc/ctrl+c", "quit/back")),
}

// --- Model ---
type Model struct {
	path     string
	lastPath string // For relative path resolution
	items    []fs.DirEntry
	selected map[string]struct{}
	cursor   int
	keys     KeyMap
	quitting bool
	mode     mode
	input    textinput.Model
	inputErr error
	height   int // For viewport height
	offset   int // For scrolling
	files    []*fileInfo.FileNode
	// OnSelect func([]*fileInfo.FileNode) tea.Cmd // Callback for when files are selected
}

func InitialModel() Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 80
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))

	wd, err := os.Getwd()
	if err != nil {
		log.Printf("Could not get working directory: %v", err)
		wd = "" // Fallback to empty string
	}

	return Model{
		path:     "",              // Initially empty
		lastPath: wd,              // Start with the working directory
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
		return m, nil
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
			// If the cursor moved above the visible viewport, scroll up
			if m.cursor < m.offset {
				m.offset--
			}
		}

	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.items)-1 {
			m.cursor++
			// If the cursor moved below the visible viewport, scroll down
			visibleItems := m.visibleItems()
			if m.cursor >= m.offset+visibleItems {
				m.offset++
			}
		}

	case key.Matches(msg, m.keys.Right): // Page down
		visibleItems := m.visibleItems()
		// Move cursor down by one page
		m.cursor += visibleItems
		if m.cursor >= len(m.items) {
			m.cursor = len(m.items) - 1
		}
		// Scroll the view down by one page
		m.offset += visibleItems
		if m.offset > len(m.items)-visibleItems {
			m.offset = len(m.items) - visibleItems
		}
		// Ensure the cursor is within the visible viewport
		if m.cursor >= m.offset+visibleItems {
			m.offset = m.cursor - visibleItems + 1
		}

	case key.Matches(msg, m.keys.Left): // Page up
		visibleItems := m.visibleItems()
		// Move cursor up by one page
		m.cursor -= visibleItems
		if m.cursor < 0 {
			m.cursor = 0
		}
		// Scroll the view up by one page
		m.offset -= visibleItems
		if m.offset < 0 {
			m.offset = 0
		}
		// Ensure the cursor is within the visible viewport
		if m.cursor < m.offset {
			m.offset = m.cursor
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
			files := getSelectedFileNodes(m.selected)
			// Fallback to sending a message if no callback is provided.
			return m, func() tea.Msg {
				return SelectedFileNodeMsg{Files: files}
			}
		}
	}
	return m, nil
}

func (m *Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if key.Matches(msg, m.keys.Confirm) {
		path := m.input.Value()
		// Resolve path relative to the last path
		if !filepath.IsAbs(path) {
			path = filepath.Join(m.lastPath, path)
		}

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
		m.lastPath = absPath // Update last path
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
	var s strings.Builder

	// Header
	s.WriteString("Enter a path to browse, or select files below. " + m.helpView() + "\n \n")
	s.WriteString(m.input.View())
	if m.inputErr != nil {
		s.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.inputErr.Error()))
	}
	s.WriteString("\n\n")

	if m.path == "" {
		return s.String()
	}

	if m.path != "" {
		s.WriteString(fmt.Sprintf("Browsing: %s\n\n", m.path))
	}

	// Table column widths
	nameWidth := 36
	typeWidth := 30
	timeWidth := 20
	sizeWidth := 16

	// Table header: pad first, then style
	headerStyle := lipgloss.NewStyle().Bold(true)
	s.WriteString(
		headerStyle.Render(util.PadRight("", 5)) + " " +
			headerStyle.Render(util.PadRight("Name", nameWidth)) + " " +
			headerStyle.Render(util.PadRight("Last Modified", timeWidth)) + " " +
			headerStyle.Render(util.PadRight("Size", sizeWidth)) +
			headerStyle.Render(util.PadRight("Type", typeWidth)) + "\n\n",
	)

	visibleItems := m.visibleItems()

	start := m.offset
	end := m.offset + visibleItems
	if end > len(m.items) {
		end = len(m.items)
	}

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
			s.WriteString(style.CursorStyle.String())
		} else {
			s.WriteString("  ")
		}

		path := filepath.Join(m.path, item.Name())
		if _, ok := m.selected[path]; ok {
			s.WriteString(style.SelectedStyle.String())
		} else {
			s.WriteString(style.DeselectedStyle.String())
		}

		info, err := item.Info()
		modTime := ""
		size := ""

		if err == nil {
			modTime = info.ModTime().Format("2006-01-02 15:04:05")
			if info.IsDir() {
				size = "<DIR>"
			} else {
				size = util.FormatSize(info.Size())
			}
		}
		nameStr := item.Name()
		if item.IsDir() {
			nameStr = nameStr + "/"
		}
		typeStr := ""
		mime, err := mimetype.DetectFile(path)
		if err == nil {
			typeStr = mime.String()
		}

		// Pad right first, then add style
		nameCell := util.PadRight(nameStr, nameWidth)
		typeCell := util.PadRight(typeStr, typeWidth)
		timeCell := util.PadRight(modTime, timeWidth)
		sizeCell := util.PadRight(size, sizeWidth)

		if item.IsDir() {
			nameCell = style.DirStyle.Render(nameCell)
		}
		// For regular files, don't add nameCol.Render, just output the padded nameCell
		s.WriteString(nameCell + " " +
			timeCell + " " +
			sizeCell + "" +
			typeCell + "\n\n")
	}

	// Scroll indicator
	if len(m.items) > visibleItems {
		s.WriteString(fmt.Sprintf("\n... %d/%d ...\n", m.cursor+1, len(m.items)))
	}

	return s.String()
}

func (m Model) helpView() string {
	return lipgloss.NewStyle().Faint(true).Render(
		fmt.Sprintf("Use '%s'/'%s' to page, '%s' to browse, '%s' to confirm, '%s' to quit",
			m.keys.Left.Help().Key, m.keys.Right.Help().Key, m.keys.ToggleInput.Help().Key, m.keys.Confirm.Help().Key, m.keys.Quit.Help().Key),
	)
}

func getSelectedFileNodes(selection map[string]struct{}) []fileInfo.FileNode {
	var files []fileInfo.FileNode
	for path := range selection {
		info, err := fileInfo.CreateNode(path)
		if err != nil {
			log.Printf("Failed to create fileNode, %v", err)
			continue
		}
		files = append(files, info)
	}
	return files
}

func (m *Model) SetPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}
	items, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Errorf("could not read directory: %w", err)
	}

	sort.Slice(m.items, func(i, j int) bool {
		return m.items[i].Name() < m.items[j].Name()
	})
	m.path = absPath
	m.lastPath = absPath // Also update the last path
	m.items = items
	m.cursor = 0
	m.offset = 0
	m.inputErr = nil
	m.mode = modeBrowse
	return nil
}

func (m *Model) visibleItems() int {
	headerHeight := 8
	if m.inputErr != nil {
		headerHeight++
	}
	// Each item now takes up 2 lines
	visible := (m.height - headerHeight) / 2
	if visible < 1 {
		// Fallback to a reasonable number, maybe half of the original
		visible = 8
	}
	return visible
}
