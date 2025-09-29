package fileTree

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/internal/util"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/rescp17/lanFileSharer/pkg/ui/components"
)

// KeyMap defines the keybindings for the file tree.
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	GoToParent key.Binding
	GoToChild  key.Binding
	Quit       key.Binding
}

const PAGE_VISIBLE_ITEM_COUNT = 8

// DefaultKeyMap provides sensible default keybindings.
var DefaultKeyMap = KeyMap{
	Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	GoToParent: key.NewBinding(key.WithKeys("backspace", "h", "b"), key.WithHelp("←/h/b", "back")),
	GoToChild:  key.NewBinding(key.WithKeys("enter", "l"), key.WithHelp("→/l/enter", "open")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// Model represents the state of the file tree TUI.
type Model struct {
	nodes       []fileInfo.FileNode
	activeItems []components.NodeDisplayItem
	keys        KeyMap
	// history is a stack that keeps track of the parent nodes, allowing for "back" navigation.
	history [][]fileInfo.FileNode
	cursor  int
	height  int // For viewport height
	offset  int // For scrolling
}

// NewFileTree creates a new file tree model.
func NewFileTree(title string, nodes []fileInfo.FileNode) Model {
	var itemsCount int
	if len(nodes) > PAGE_VISIBLE_ITEM_COUNT {
		itemsCount = PAGE_VISIBLE_ITEM_COUNT
	} else {
		itemsCount = len(nodes)
	}

	items := make([]components.NodeDisplayItem, itemsCount)

	for i := range items {
		items[i] = components.GetNodeDisplayItemFromFileNode(nodes[i])
	}

	return Model{
		nodes: nodes,
		activeItems: items,
		keys:        DefaultKeyMap,
		// Pre-allocate a bit of capacity for the history stack
		history: make([][]fileInfo.FileNode, 0, 5),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model's state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.activeItems)-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.GoToParent):
			if len(m.history) > 0 {
				// Pop from the history stack to go back to the parent.
				lastIndex := len(m.history) - 1
				m.nodes = m.history[lastIndex]
				m.history = m.history[:lastIndex] // Slice off the last element
				m.cursor = 0
			}

		case key.Matches(msg, m.keys.GoToChild):
			if len(m.nodes) == 0 {
				return m, nil
			}
			selectedNode := m.nodes[m.cursor]
			if selectedNode.IsDir && len(selectedNode.Children) > 0 {
				// Push the current view onto the history stack.
				m.history = append(m.history, m.nodes)
				// Move into the child directory.
				m.nodes = selectedNode.Children
				m.cursor = 0
			}
		}
	}

	return m, nil
}

// View renders the UI.
func (m Model) View() string {
	var s strings.Builder

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
	length := len(m.activeItems)

	if end > length {
		end = length
	}

	// Ensure we don't render a slice with a negative start index
	if start < 0 {
		start = 0
	}

	slice := m.activeItems

	for i, item := range slice {
		actualIndex := start + i
		if m.cursor == actualIndex {
			s.WriteString("▶ ")
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
			nameCell = style.RenderWithSafeReset(dirNameStyle, nameCell) + lipgloss.NewStyle().Render("")
			// dirTypeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
			// typeCell = style.RenderWithSafeReset(dirTypeStyle, typeCell)
		} else {
			fileNameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
			nameCell = style.RenderWithSafeReset(fileNameStyle, nameCell) + lipgloss.NewStyle().Render("")
			// typeCell = style.RenderWithSafeReset(components.GetColorForFileType(item), typeCell)
		}

		timeCellStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
		s.WriteString(nameCell + " " +
			style.RenderWithSafeReset(timeCellStyle, timeCell) + " " +
			sizeCell + " " +
			typeCell + "\n\n")
	}

	// Scroll indicator
	if length > visibleItems {
		scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Italic(true)
		s.WriteString(style.RenderWithSafeReset(scrollStyle, fmt.Sprintf("\n... %d/%d ...\n", m.cursor+1, length)))
	}

	return s.String()
}

func (m *Model) visibleItems() int {
	headerHeight := 8
	// Each item now takes up 2 lines
	visible := (m.height - headerHeight) / 2
	if visible < 1 {
		// Fallback to a reasonable number, maybe half of the original
		visible = 8
	}
	return visible
}
