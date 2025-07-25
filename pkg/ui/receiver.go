package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/internal/app_events"
	receiverEvent "github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/pkg/fileTree"
)

// receiverState defines the different states of the receiver UI
type receiverState int

const (
	awaitingConnection receiverState = iota
	showFileNodes
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
	case awaitingConfirmation:
		help := fmt.Sprintf("  %s/%s  %s/%s \n",
			DefaultKeyMap.Accept.Help().Key, DefaultKeyMap.Accept.Help().Desc,
			DefaultKeyMap.Reject.Help().Key, DefaultKeyMap.Reject.Help().Desc,
		)
		return fmt.Sprintf("%s\n%s", m.receiver.fileTree.View(), style.HelpStyle.Render(help))
	case receivingFiles:
		return fmt.Sprintf("\n\n %s Receiving files...", m.receiver.spinner.View())
	case receiveFailed:
		return fmt.Sprintf("\nAn error occurred: %v\n\nPress Enter to restart.", style.ErrorStyle.Render(m.receiver.lastError.Error()))
	default:
		return "Internal error: unknown receiver state"
	}
}

func (m *model) updateReceiver(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case app_events.ErrorMsg:
		m.receiver.lastError = msg.Err
		m.receiver.state = receiveFailed
		return m, m.listenForAppMessages()

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.receiver.state {
		case receiveFailed:
			if msg.String() == "enter" {
				*m = InitialModel(Receiver, m.receiver.port)
				return m, m.Init()
			}
		case awaitingConfirmation:
			switch {
			case key.Matches(msg, DefaultKeyMap.Accept):
				m.appController.AppEvents() <- receiverEvent.AcceptFileRequestEvent{}
				m.receiver.state = receivingFiles
				return m, m.listenForAppMessages()
			case key.Matches(msg, DefaultKeyMap.Reject):
				m.appController.AppEvents() <- receiverEvent.RejectFileRequestEvent{}
				*m = InitialModel(Receiver, m.receiver.port)
				return m, m.Init()
			default:
				newFileTree, cmd := m.receiver.fileTree.Update(msg)
				m.receiver.fileTree = newFileTree.(fileTree.Model)
				cmds = append(cmds, cmd)
			}
		}

	case receiverEvent.FileNodeUpdateMsg:
		if m.receiver.state == awaitingConnection {
			m.receiver.state = awaitingConfirmation // 直接进入等待确认状态
			m.receiver.fileTree = fileTree.NewFileTree("Received files info:", msg.Nodes)
			cmds = append(cmds, m.listenForAppMessages())
		}
	}

	// 为 spinner 更新这样的通用组件传递消息
	var spinCmd tea.Cmd
	m.receiver.spinner, spinCmd = m.receiver.spinner.Update(msg)
	cmds = append(cmds, spinCmd)

	return m, tea.Batch(cmds...)
}
