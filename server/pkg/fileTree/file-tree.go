package fileTree

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/internal/util"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/rescp17/lanFileSharer/internal/style"
)


// KeyMap defines the keybindings for the file tree.
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	GoToParent key.Binding
	GoToChild  key.Binding
	Quit       key.Binding
}

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
	title string
	nodes []fileInfo.FileNode
	keys  KeyMap
	// history is a stack that keeps track of the parent nodes, allowing for "back" navigation.
	history [][]fileInfo.FileNode
	cursor  int
	width   int
	height  int
}

// NewFileTree creates a new file tree model.
func NewFileTree(title string, nodes []fileInfo.FileNode) Model {
	return Model{
		title: title,
		nodes: nodes,
		keys:  DefaultKeyMap,
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.nodes)-1 {
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

	// Title
	s.WriteString(style.TitleStyle.Render(m.title))
	s.WriteString("\n\n")

	nameWidth := 40
	sizeWidth := 20
	typeWidth := 35

	// Header
	s.WriteString(style.HeaderStyle.Render(util.PadRight("", 1)))
	s.WriteString(style.HeaderStyle.Render(util.PadRight("Name", nameWidth)))
	s.WriteString(style.HeaderStyle.Render(util.PadRight("Size", sizeWidth)))
	s.WriteString(style.HeaderStyle.Render(util.PadRight("Type", typeWidth)))
	s.WriteString("\n\n")

	// File list
	for i, node := range m.nodes {
		cursor := style.NoCursorStyle.String()
		if m.cursor == i {
			cursor = style.CursorStyle.String()
		}

		name := node.Name
		sizeStr := ""
		typeStr := "<DIR>"

		if node.IsDir {
			name = style.DirStyle.Render(name + "/")
		} else {
			name = style.FileStyle.Render(name)
			sizeStr = util.FormatSize(node.Size)
			typeStr = node.MimeType
		}

		s.WriteString(cursor)
		s.WriteString(util.PadRight(name, nameWidth))
		s.WriteString(util.PadRight(sizeStr, sizeWidth))
		s.WriteString(util.PadRight(typeStr, typeWidth))
		s.WriteString("\n\n")
	}

	// Help view
	help := fmt.Sprintf("\n%s  %s  %s  %s  %s",
		m.keys.Up.Help().Key+"/"+m.keys.Up.Help().Desc,
		m.keys.Down.Help().Key+"/"+m.keys.Down.Help().Desc,
		m.keys.GoToChild.Help().Key+"/"+m.keys.GoToChild.Help().Desc,
		m.keys.GoToParent.Help().Key+"/"+m.keys.GoToParent.Help().Desc,
		m.keys.Quit.Help().Key+"/"+m.keys.Quit.Help().Desc,
	)
	s.WriteString(style.HelpStyle.Render(help))

	return style.DocStyle.Render(s.String())
}

// GetSelectedNode returns the currently selected FileNode.
// This can be called after the TUI exits to get the user's choice.
func (m *Model) GetSelectedNode() *fileInfo.FileNode {
	if len(m.nodes) > 0 && m.cursor < len(m.nodes) {
		return &m.nodes[m.cursor]
	}
	return nil
}
