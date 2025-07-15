package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	receiverEvent "github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/pkg/fileTree"
)

// receiverState defines the different states of the receiver UI
type receiverState int

const (
	awaitingConnection receiverState = iota
	showingFileNodes
	receivingFiles
	receiveComplete
	receiveFailed
)

type receiverModel struct {
	state     receiverState
	spinner   spinner.Model
	port      int
	fileTree  fileTree.Model
	lastError error
}

type KeyMap struct {
	Accept key.Binding
	Reject key.Binding
}

// DefaultKeyMap provides sensible default keybindings.
var DefaultKeyMap = KeyMap{
	Accept:  key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "Accept")),
	Reject: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "Reject")),
}

func initReceiverModel(app AppController, port int) receiverModel {
	s := style.NewSpinner()

	return receiverModel{
		spinner: s,
		port:    port,
		state:   awaitingConnection,
	}
}

func (m *model) initReceiver() tea.Cmd {
	return tea.Batch(
		m.receiver.spinner.Tick,
		m.listenForAppMessages(),
	)
}

func (m model) receiverView() string {
	switch m.receiver.state {
	case awaitingConnection:
		return fmt.Sprintf("\n\n %s Awaiting sender connection on port %d...", m.receiver.spinner.View(), m.receiver.port)
	case showingFileNodes:
		return fmt.Sprintf("%s\nPress Y ", m.receiver.fileTree.View())
	case receiveFailed:
		return fmt.Sprintf("\nAn error occurred: %v\n", style.ErrorStyle.Render(m.receiver.lastError.Error()))
	default:
		return "Internal error: unknown receiver state"
	}
}

func (m *model) updateReceiver(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle high-priority messages first
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	case receiverEvent.ErrorMsg:
		m.receiver.lastError = msg.Err
		m.receiver.state = receiveFailed
		return m, m.listenForAppMessages()
	}

	// Handle state-specific updates
	switch m.receiver.state {
	case receiveFailed:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
			// Restart the receiver UI
			*m = InitialModel(Receiver, m.receiver.port)
			return m, m.Init()
		}
	case showingFileNodes:
		newFileTree, cmd := m.receiver.fileTree.Update(msg)
		m.receiver.fileTree = newFileTree.(fileTree.Model)
		// TODO accept or not accept
		cmds = append(cmds, cmd)
	case awaitingConnection:
		switch msg := msg.(type) {
		case receiverEvent.FileNodeUpdateMsg:
			m.receiver.state = showingFileNodes
			m.receiver.fileTree = fileTree.NewFileTree("Received files info:", msg.Nodes)
			cmds = append(cmds, m.listenForAppMessages())
		}
	}

	var spinCmd tea.Cmd
	m.receiver.spinner, spinCmd = m.receiver.spinner.Update(msg)
	cmds = append(cmds, spinCmd)

	return m, tea.Batch(cmds...)
}
