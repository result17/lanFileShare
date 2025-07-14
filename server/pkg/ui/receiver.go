package ui

import (
	"fmt"

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
	state    receiverState
	spinner  spinner.Model
	port     int
	fileTree fileTree.Model
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
		return m.receiver.fileTree.View()
	default:
		return "Internal error: unknown receiver state"
	}
}

func (m *model) updateReceiver(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// In the future, we might want to send a QuitAppMsg here.
			return m, tea.Quit
		}
	case receiverEvent.FileNodeUpdateMsg:
		m.receiver.state = showingFileNodes
		m.receiver.fileTree = fileTree.NewFileTree("Received files:", msg.Nodes)
		return m, m.listenForAppMessages()
	case receiverEvent.ErrorMsg:
		// In the future, we can add a receiveFailed state and display the error.
		// For now, just quit.
		return m, tea.Quit
	}

	// If we are showing the file tree, pass messages to it.
	if m.receiver.state == showingFileNodes {
		newFileTree, cmd := m.receiver.fileTree.Update(msg)
		m.receiver.fileTree = newFileTree.(fileTree.Model)
		cmds = append(cmds, cmd)
	}

	var spinCmd tea.Cmd
	m.receiver.spinner, spinCmd = m.receiver.spinner.Update(msg)
	cmds = append(cmds, spinCmd)

	return m, tea.Batch(cmds...)
}
