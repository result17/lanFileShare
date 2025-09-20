package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	appevents "github.com/rescp17/lanFileSharer/internal/app_events"
	receiverEvent "github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/pkg/fileTree"
	"github.com/rescp17/lanFileSharer/pkg/ui/components"
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
	appController AppController
	state         receiverState
	port          int
	fileTree      fileTree.Model
	lastError     error
	senderCard    components.SenderCardModel
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

func initReceiverModel(port int, appController AppController) receiverModel {

	return receiverModel{
		port:    port,
		state:   awaitingConnection,
	}
}


func (m model) receiverView() string {
	var result strings.Builder
	var mainContent string
	switch m.receiver.state {
	case awaitingConnection:
		mainContent = fmt.Sprintf("\n\n %s %s %s...", style.RenderWithSafeReset(style.HighlightFontStyle, m.spinner.View()), style.RenderWithSafeReset(style.HighlightFontStyle, "Awaiting sender connection on port"), style.RenderWithSafeReset(style.ThemeStyle, strconv.Itoa(m.receiver.port)))
	case awaitingConfirmation:
		help := fmt.Sprintf("  %s/%s  %s/%s \n",
			DefaultKeyMap.Accept.Help().Key, DefaultKeyMap.Accept.Help().Desc,
			DefaultKeyMap.Reject.Help().Key, DefaultKeyMap.Reject.Help().Desc,
		)
		mainContent = fmt.Sprintf("%s\n%s\n%s", m.receiver.senderCard.View(), m.receiver.fileTree.View(), style.HelpStyle.Render(help))
	case receivingFiles:
		mainContent = fmt.Sprintf("\n\n %s Receiving files...", m.spinner.View())
	case receiveComplete: // Add this new case
		mainContent = "File transfer complete!\n\nPress Enter to exit."
	case receiveFailed:
		mainContent = fmt.Sprintf("An error occurred: %v\n\nPress Enter to restart.", style.ErrorStyle.Render(m.receiver.lastError.Error()))
	default:
		mainContent = "Internal error: unknown receiver state"
	}

	result.WriteString(m.responsiveLayout.AdaptiveContainer(mainContent, "Receiver"))

	return result.String()
}

func (m *model) resetReceiver() (tea.Model, tea.Cmd) {
	m.receiver = initReceiverModel(m.receiver.port, m.receiver.appController)
	return m, m.Init()
}

func (m *model) updateReceiver(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.listenForAppMessages()
	switch msg := msg.(type) {
	// Handle global events first
	case appevents.Error:
		m.receiver.lastError = msg.Err
		m.receiver.state = receiveFailed
		return m, cmd
	case receiverEvent.TransferFinishedMsg:
		m.receiver.state = receiveComplete
		return m, cmd
	}

	switch m.receiver.state {
	case awaitingConnection:
		return m.updateAwaitingConnection(msg, cmd)
	case awaitingConfirmation:
		return m.updateAwaitingConfirmation(msg, cmd)
	case receivingFiles:
		return m.updateReceivingFiles(msg, cmd)
	case receiveComplete, receiveFailed:
		return m.updateReceiveFinishedOrFailed(msg, cmd)
	}

	return m, cmd
}

func (m *model) updateReceivingFiles(msg tea.Msg, defaultCmd tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case receiverEvent.FileNodeUpdateMsg:
		m.receiver.fileTree = fileTree.NewFileTree("Received files info:", msg.Nodes)
		return m, defaultCmd
	}
	return m, defaultCmd
}

// Example of a new state-specific update function
func (m *model) updateAwaitingConnection(msg tea.Msg, defaultCmd tea.Cmd) (tea.Model, tea.Cmd)  {
	switch msg := msg.(type) {
	case receiverEvent.SenderUpdateMsg:
		cardModel, _ := m.receiver.senderCard.Update(msg)
		m.receiver.senderCard = cardModel.(components.SenderCardModel)
		return m, defaultCmd
	case receiverEvent.FileNodeUpdateMsg:
		m.receiver.state = awaitingConfirmation
		m.receiver.fileTree = fileTree.NewFileTree("Received files info:", msg.Nodes)
		return m, defaultCmd
	}
	return m, defaultCmd
}

func (m *model) updateAwaitingConfirmation(msg tea.Msg, defaultCmd tea.Cmd) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, DefaultKeyMap.Accept):
			m.appController.AppEvents() <- receiverEvent.FileRequestAccepted{}
			m.receiver.state = receivingFiles
			return m, defaultCmd
		case key.Matches(keyMsg, DefaultKeyMap.Reject):
			m.appController.AppEvents() <- receiverEvent.FileRequestRejected{}
			return m.resetReceiver()
		default:
			newFileTree, cmd := m.receiver.fileTree.Update(msg)
			m.receiver.fileTree = newFileTree.(fileTree.Model)
			return m, cmd
		}
	}
	return m, defaultCmd
}

func (m *model) updateReceiveFinishedOrFailed(msg tea.Msg, defaultCmd tea.Cmd) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch m.receiver.state {
		case receiveComplete:
			if keyMsg.Type == tea.KeyEnter {
				return m, tea.Quit
			}
		case receiveFailed:
			if keyMsg.Type == tea.KeyEnter {
				return m.resetReceiver()
			}
		}
	}
	// Ignore all other messages in final states.
	return m, defaultCmd
}
