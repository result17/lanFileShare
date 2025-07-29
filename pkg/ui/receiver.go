package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	appevents "github.com/rescp17/lanFileSharer/internal/app_events"
	receiverEvent "github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/pkg/fileTree"
)

// receiverState defines the different states of the receiver UI
type receiverState int

const (
	awaitingConnection receiverState = iota
	awaitingConfirmation
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
	Accept: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "Accept")),
	Reject: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "Reject")),
}

func initReceiverModel(port int) receiverModel {
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
	case awaitingConfirmation:
		help := fmt.Sprintf("  %s/%s  %s/%s \n",
			DefaultKeyMap.Accept.Help().Key, DefaultKeyMap.Accept.Help().Desc,
			DefaultKeyMap.Reject.Help().Key, DefaultKeyMap.Reject.Help().Desc,
		)
		return fmt.Sprintf("%s\n%s", m.receiver.fileTree.View(), style.HelpStyle.Render(help))
	case receivingFiles:
		return fmt.Sprintf("\n\n %s Receiving files...", m.receiver.spinner.View())
	case receiveComplete: // Add this new case
		return "\nFile transfer complete!\n\nPress Enter to exit."
	case receiveFailed:
		return fmt.Sprintf("\nAn error occurred: %v\n\nPress Enter to restart.", style.ErrorStyle.Render(m.receiver.lastError.Error()))
	default:
		return "Internal error: unknown receiver state"
	}
}

func (m *model) resetReceiver() (tea.Model, tea.Cmd) {
	m.receiver = initReceiverModel(m.receiver.port)
	return m, m.Init()
}

func (m *model) updateReceiver(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case appevents.AppErrorMsg:
		m.receiver.lastError = msg.Err
		m.receiver.state = receiveFailed
		return m, nil

	case tea.KeyMsg:
		switch m.receiver.state {
		case receiveFailed:
			if msg.Type == tea.KeyEnter {
				return m.resetReceiver()
			}
		case receiveComplete:
			if msg.Type == tea.KeyEnter {
				return m, tea.Quit
			}
		case awaitingConfirmation:
			switch {
			case key.Matches(msg, DefaultKeyMap.Accept):
				m.appController.AppEvents() <- receiverEvent.FileRequestAccepted{}
				m.receiver.state = receivingFiles
				return m, nil
			case key.Matches(msg, DefaultKeyMap.Reject):
				m.appController.AppEvents() <- receiverEvent.FileRequestRejected{}
				return m.resetReceiver()
			default:
				newFileTree, cmd := m.receiver.fileTree.Update(msg)
				m.receiver.fileTree = newFileTree.(fileTree.Model)
				cmds = append(cmds, cmd)
			}
		}

	case receiverEvent.FileNodeUpdateMsg:
		if m.receiver.state == awaitingConnection {
			m.receiver.state = awaitingConfirmation // Transition to confirmation state
			m.receiver.fileTree = fileTree.NewFileTree("Received files info:", msg.Nodes)
		}
	case receiverEvent.TransferCompleteMsg:
		m.receiver.state = receiveComplete
		return m, nil // Stop listening for other app messages
	}

	var spinCmd tea.Cmd
	if m.receiver.state == awaitingConnection || m.receiver.state == receivingFiles {
		m.receiver.spinner, spinCmd = m.receiver.spinner.Update(msg)
	}
	cmds = append(cmds, spinCmd)

	return m, tea.Batch(cmds...)
}
