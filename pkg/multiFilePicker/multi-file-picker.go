package multiFilePicker

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/internal/util"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/rescp17/lanFileSharer/pkg/ui/components"
)

type mode int
type sortType int
type SelectedFileNodeMsg struct {
	Files []fileInfo.FileNode
}

const (
	modeBrowse mode = iota
	modeInput
)

const (
	sortByName sortType = iota
	sortBySize
	sortByDate
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
	BackUp       key.Binding
	Quit         key.Binding
	SelectAll    key.Binding
	DeselectAll  key.Binding
	InvertSelect key.Binding
	SortBySize   key.Binding
	SortByName   key.Binding
	SortByDate   key.Binding
	SearchToggle key.Binding
}

var DefaultKeyMap = KeyMap{
	Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
	Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
	Left:         key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "page up")),
	Right:        key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "page down")),
	ToggleSelect: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle select")),
	ToggleInput:  key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "input path")),
	Confirm:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm/navigate")),
	Quit:         key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc/ctrl+c", "quit/back")),
	SelectAll:    key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "select all")),
	DeselectAll:  key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "deselect all")),
	InvertSelect: key.NewBinding(key.WithKeys("ctrl+i"), key.WithHelp("ctrl+i", "invert selection")),
	SortBySize:   key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "sort by size")),
	SortByName:   key.NewBinding(key.WithKeys("ctrl+n"), key.WithHelp("ctrl+n", "sort by name")),
	SortByDate:   key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "sort by date")),
	SearchToggle: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "toggle search")),
}

// --- Model ---
type Model struct {
	path       string
	lastPath   string // For relative path resolution
	items      []components.NodeDisplayItem
	selected   map[string]struct{}
	cursor     int
	keys       KeyMap
	quitting   bool
	mode       mode
	sortType   sortType
	sortAsc    bool // true for ascending, false for descending
	input      textinput.Model
	inputErr   error
	height     int // For viewport height
	offset     int // For scrolling
	files      []*fileInfo.FileNode
	searchMode bool
	// OnSelect func([]*fileInfo.FileNode) tea.Cmd // Callback for when files are selected
}

func InitialModel() Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 80

	ti.Cursor.Style = style.CreateSafeCursorStyle()

	wd, err := os.Getwd()
	if err != nil {
		log.Printf("Could not get working directory: %v", err)
		wd = "" // Fallback to empty string
	}

	return Model{
		path:     "",                             // Initially empty
		lastPath: wd,                             // Start with the working directory
		items:    []components.NodeDisplayItem{}, // Initially empty
		selected: make(map[string]struct{}),
		keys:     DefaultKeyMap,
		mode:     modeInput, // Start in input mode
		input:    ti,
	}
}

// sortItems sorts the items based on the current sortType and sortAsc
func (m *Model) sortItems() {
	sort.Slice(m.items, func(i, j int) bool {
		// Directories always come first
		if m.items[i].IsDir != m.items[j].IsDir {
			return m.items[i].IsDir
		}

		var less bool
		switch m.sortType {
		case sortByName:
			less = strings.ToLower(m.items[i].Name) < strings.ToLower(m.items[j].Name)
		case sortBySize:
			if m.items[i].Size == "<DIR>" && m.items[j].Size == "<DIR>" {
				less = m.items[i].Name < m.items[j].Name
			} else if m.items[i].Size == "<DIR>" {
				less = true
			} else if m.items[j].Size == "<DIR>" {
				less = false
			} else {
				// Parse sizes for comparison (simplified - in real implementation you'd need to parse the size strings)
				less = m.items[i].Size < m.items[j].Size
			}
		case sortByDate:
			less = m.items[i].ModTime < m.items[j].ModTime
		default:
			less = m.items[i].Name < m.items[j].Name
		}

		if !m.sortAsc {
			less = !less
		}
		return less
	})
}

func (m *Model) loadDirectory(path string) ([]components.NodeDisplayItem, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.lastPath, path)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		m.inputErr = fmt.Errorf("invalid path: %w", err)
		slog.Error("invalid path", "error", err)
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		inputErr := fmt.Errorf("path does not exist: %s, %v", absPath, err)
		m.inputErr = inputErr
		slog.Error("path does not exist", "path", absPath, "error", err)
		return nil, inputErr
	}
	if !info.IsDir() {
		inputErr := fmt.Errorf("path is not a directory: %s", absPath)
		m.inputErr = inputErr
		slog.Error("path is not a directory", "path", absPath)
		return nil, inputErr
	}

	// Path is a valid directory, load its contents
	entries, err := os.ReadDir(absPath)
	if err != nil {
		m.inputErr = fmt.Errorf("could not read directory: %w", err)
		return nil, err
	}

	newItems := make([]components.NodeDisplayItem, len(entries))

	for i, entry := range entries {
		newItems[i] = components.GetNodeDisplayItemFromDirEntity(entry, absPath)
	}

	// Sort items: directories first, then files, both alphabetically
	sort.Slice(newItems, func(i, j int) bool {
		if newItems[i].IsDir != newItems[j].IsDir {
			return newItems[i].IsDir // Directories first
		}
		return newItems[i].Name < newItems[j].Name
	})

	return newItems, nil
}

// --- Bubble Tea Methods ---
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) IsBrowseMode() bool {
	return m.mode == modeBrowse
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

//nolint:gocyclo
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
			newOffset := len(m.items) - visibleItems
			if newOffset < 0 {
				newOffset = 0
			}
			m.offset = newOffset
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
		if m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if _, ok := m.selected[item.Path]; ok {
				delete(m.selected, item.Path)
			} else {
				m.selected[item.Path] = struct{}{}
			}
		}

	case key.Matches(msg, m.keys.Confirm):
		// If we have selected files, return them
		if len(m.selected) > 0 {
			files := getSelectedFileNodes(m.selected)
			return m, func() tea.Msg {
				return SelectedFileNodeMsg{Files: files}
			}
		}

		// If no files selected but cursor is on a directory, navigate into it
		if m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.IsDir {
				items, err := m.loadDirectory(item.Path)
				if err != nil {
					// Error is already set in m.inputErr by loadDirectory
					return m, nil
				}

				m.path = item.Path
				m.lastPath = item.Path
				m.items = items
				m.cursor = 0
				m.offset = 0
				m.inputErr = nil
				return m, nil
			}
		}

	case key.Matches(msg, m.keys.SelectAll):
		for _, item := range m.items {
			if !item.IsDir { // Only select files, not directories
				m.selected[item.Path] = struct{}{}
			}
		}

	case key.Matches(msg, m.keys.DeselectAll):
		m.selected = make(map[string]struct{})

	case key.Matches(msg, m.keys.InvertSelect):
		newSelected := make(map[string]struct{})
		for _, item := range m.items {
			if !item.IsDir {
				if _, ok := m.selected[item.Path]; !ok {
					newSelected[item.Path] = struct{}{}
				}
			}
		}
		m.selected = newSelected

	case key.Matches(msg, m.keys.SortByName):
		m.sortType = sortByName
		m.sortAsc = !m.sortAsc
		m.sortItems()

	case key.Matches(msg, m.keys.SortBySize):
		m.sortType = sortBySize
		m.sortAsc = !m.sortAsc
		m.sortItems()

	case key.Matches(msg, m.keys.SortByDate):
		m.sortType = sortByDate
		m.sortAsc = !m.sortAsc
		m.sortItems()

	case key.Matches(msg, m.keys.SearchToggle):
		m.searchMode = !m.searchMode
		if m.searchMode {
			m.input.Placeholder = "Search files..."
			m.mode = modeInput
			m.input.Focus()
		} else {
			m.input.Placeholder = ""
			m.mode = modeBrowse
			m.input.Blur()
		}
	}
	return m, nil
}

func (m *Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if key.Matches(msg, m.keys.Confirm) {
		path := m.input.Value()

		// Load directory - all path validation is handled in loadDirectory
		items, err := m.loadDirectory(path)
		if err != nil {
			// Error is already set in m.inputErr by loadDirectory
			return m, nil
		}

		// Get absolute path for display
		absPath := path
		if !filepath.IsAbs(path) {
			absPath = filepath.Join(m.lastPath, path)
		}
		absPath, _ = filepath.Abs(absPath) // We know this works since loadDirectory succeeded

		m.path = absPath
		m.lastPath = absPath
		m.items = items
		m.mode = modeBrowse
		m.input.Reset()
		m.inputErr = nil
		m.cursor = 0
		m.offset = 0
		return m, nil
	}

	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

//nolint:gocyclo
func (m Model) View() string {
	var s strings.Builder

	// Header
	s.WriteString("Enter a path to browse, or select files below. " + m.helpView() + "\n \n")

	// Use safe rendering for textinput if you have a background color
	// If you don't have a background color for the textinput, just use the original
	textInputView := m.input.View()
	// If textinput has a background color, use safe rendering:
	// textInputView = style.RenderTextInputWithSafeBackground(textInputView, lipgloss.Color("your-bg-color"))
	// For now, just use the safe reset function:
	textInputView = style.RenderWithSafeReset(lipgloss.NewStyle(), textInputView)
	s.WriteString(textInputView)

	if m.inputErr != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		s.WriteString("\n" + style.RenderWithSafeReset(errorStyle, m.inputErr.Error()))
	}
	s.WriteString("\n\n")

	if m.path == "" {
		return s.String()
	}

	if m.path != "" {
		s.WriteString(fmt.Sprintf("📂 %s\n\n", m.path))
	}

	// Table column widths
	nameWidth := 34
	typeWidth := 20
	timeWidth := 19
	sizeWidth := 14

	// Table header: pad first, then style
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	s.WriteString(
		style.RenderWithSafeReset(headerStyle, util.PadRight("", 5)) + " " +
			style.RenderWithSafeReset(headerStyle, util.PadRight("Name", nameWidth)) + " " +
			style.RenderWithSafeReset(headerStyle, util.PadRight("Last Modified", timeWidth)) + " " +
			style.RenderWithSafeReset(headerStyle, util.PadRight("Size", sizeWidth)) +
			style.RenderWithSafeReset(headerStyle, util.PadRight("Type", typeWidth)) + "\n\n",
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
		slice = []components.NodeDisplayItem{}
	}

	for i, item := range slice {
		actualIndex := start + i
		if m.cursor == actualIndex {
			s.WriteString("▶ ")
		} else {
			s.WriteString("  ")
		}

		if _, ok := m.selected[item.Path]; ok {
			s.WriteString("✓ ")
		} else {
			s.WriteString("  ")
		}

		// Add emoji based on item type
		nameStr := components.GetIconForItem(item) + " " + item.Name

		// Pad right first, then add style
		nameCell := util.PadRight(nameStr, nameWidth+2) // +2 for emoji and spaces
		typeCell := util.PadRight(item.RenderType, typeWidth)
		timeCell := util.PadRight(item.ModTime, timeWidth)
		sizeCell := util.PadRight(item.Size, sizeWidth)

		if item.IsDir {
			dirNameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
			nameCell = style.RenderWithSafeReset(dirNameStyle, nameCell)
			// dirTypeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
			// typeCell = style.RenderWithSafeReset(dirTypeStyle, typeCell)
		} else {
			fileNameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
			nameCell = style.RenderWithSafeReset(fileNameStyle, nameCell)
			// typeCell = style.RenderWithSafeReset(components.GetColorForFileType(item), typeCell)
		}

		timeCellStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
		s.WriteString(nameCell + " " +
			style.RenderWithSafeReset(timeCellStyle, timeCell) + " " +
			sizeCell + " " +
			typeCell + "\n\n")
	}

	// Scroll indicator
	if len(m.items) > visibleItems {
		scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Italic(true)
		s.WriteString(style.RenderWithSafeReset(scrollStyle, fmt.Sprintf("\n... %d/%d ...\n", m.cursor+1, len(m.items))))
	}

	// Footer with selection count
	if len(m.selected) > 0 {
		footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
		s.WriteString(style.RenderWithSafeReset(footerStyle, fmt.Sprintf("\nSelected: %d file(s)", len(m.selected))))
	}

	return s.String()
}

func (m Model) helpView() string {
	helpStyle := lipgloss.NewStyle().Faint(true)
	return style.RenderWithSafeReset(helpStyle,
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
	items, err := m.loadDirectory(path)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

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
