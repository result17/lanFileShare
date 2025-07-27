package ui

import (
	"context"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	receiverApp "github.com/rescp17/lanFileSharer/pkg/receiver"
	senderApp "github.com/rescp17/lanFileSharer/pkg/sender"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/internal/style"
)

type mode int

type serverErrorMsg struct {
	err error
}

const (
	None mode = iota
	Sender
	Receiver
)

type model struct {
	mode          mode
	appController AppController
	sender        senderModel
	receiver      receiverModel
	ctx           context.Context
	cancel        context.CancelFunc
	err           error
}

func InitialModel(m mode, port int) model {
	var appController AppController
	var sender senderModel
	var receiver receiverModel

	switch m {
	case Sender:
		appController = senderApp.NewApp(&discovery.MDNSAdapter{})
		sender = initSenderModel()
	case Receiver:
		appController = receiverApp.NewApp(port)
		receiver = initReceiverModel(port)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return model{
		mode:          m,
		appController: appController,
		sender:        sender,
		receiver:      receiver,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (m model) Init() tea.Cmd {
	if m.appController == nil {
		return tea.Quit
	}

	var initCmd tea.Cmd
	switch m.mode {
	case Sender:
		initCmd = m.initSender()
	case Receiver:
		initCmd = m.initReceiver()
	}

	runCmd := func() tea.Msg {
		if err := m.appController.Run(m.ctx); err != nil {
			slog.Error("App runtime error", "error", err)
			return serverErrorMsg{err: err}
		}
		return nil
	}

	return tea.Batch(initCmd, runCmd)
}

func (m model) View() string {
	if m.err != nil {
		return "Error: " + m.err.Error() + "\n\nPress ctrl+c to quit."
	}

	var s string

	switch m.mode {
	case Sender:
		s += m.senderView()
	case Receiver:
		s += m.receiverView()
	default:
		return ""
	}
	s += style.ErrorStyle.Render(m.err.Error()) + "\n"
	s += "\nPress ctrl + c to quit"
	return s
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.QuitMsg:
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	case serverErrorMsg:
		m.err = msg.err
		return m, nil
	}

	switch m.mode {
	case Sender:
		return m.updateSender(msg)
	case Receiver:
		return m.updateReceiver(msg)
	}

	return m, nil
}
