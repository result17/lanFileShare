package ui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/pkg/fileTree"
	"github.com/rescp17/lanFileSharer/pkg/receiver"
)

// receiverState defines the different states of the receiver UI
type receiverState int

const (
	awaitingReceiver receiverState = iota
	showingFileNodes
	receivingFiles
	receiveComplete
	receiveFailed
)

type receiverModel struct {
	state    receiverState
	app      *receiver.App
	spinner  spinner.Model
	port     int
	fileTree fileTree.Model
}

func initReceiverModel(port int) model {
	s := style.NewSpinner()

	return model{
		mode: Receiver,
		receiver: receiverModel{
			spinner: s,
			port:    port,
			app:     receiver.NewApp(),
		},
	}
}

func (m *model) listenForReceiverAppMessages() tea.Cmd {
	return func() tea.Msg {
		return <-m.receiver.app.UIMessages()
	}
}

func (m model) initReceiver() tea.Cmd {
	go m.receiver.app.Run(context.Background(), m.receiver.port)

	return tea.Batch(
		m.receiver.spinner.Tick,
		m.listenForReceiverAppMessages(),
	)
}

func (m model) receiverView() string {
	switch m.receiver.state {
	case awaitingReceiver:
		return fmt.Sprintf("\n\n %s Awaiting sender connection", m.receiver.spinner.View())
	case showingFileNodes:
		return ""
	default:
		return "Internal error: unknown receiver state"
	}
}

func (m model) updateReceiver(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.receiver.spinner, cmd = m.receiver.spinner.Update(msg)
	return m, cmd
}
