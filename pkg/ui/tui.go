package ui

import (
	"context"
	"errors"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	appevents "github.com/rescp17/lanFileSharer/internal/app_events"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	receiverApp "github.com/rescp17/lanFileSharer/pkg/receiver"
	senderApp "github.com/rescp17/lanFileSharer/pkg/sender"
)

type Mode int

const (
	None Mode = iota
	Sender
	Receiver
)

type model struct {
	mode          Mode
	appController AppController
	sender        senderModel
	receiver      receiverModel
	ctx           context.Context
	cancel        context.CancelFunc
	err           error
}

func InitialModel(m Mode, port int, outputPath string) model {
	var appController AppController
	var sender senderModel
	var receiver receiverModel

	switch m {
	case Sender:
		appController = senderApp.NewApp(&discovery.MDNSAdapter{})
		sender = initSenderModel()
	case Receiver:
		appController = receiverApp.NewApp(port, outputPath)
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

			if errors.Is(err, context.Canceled) {
				return appevents.AppFinishedMsg{}
			}
			return appevents.Error{Err: err}
		}
		return appevents.AppFinishedMsg{}
	}

	return tea.Batch(initCmd, runCmd)
}

func (m model) View() string {
	if m.err != nil {
		return style.ErrorStyle.Render(m.err.Error()) + "\n\nPress ctrl+c to quit."
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

	s += "\nPress ctrl + c to quit"
	return s
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// case tea.KeyMsg:
	// 	switch msg.Type {
	// 	case tea.KeyCtrlC:
	// 		if m.cancel != nil {
	// 			m.cancel()
	// 		}
	// 		return m, tea.Quit
	// 	}
	case tea.QuitMsg:
		// This is sent on Ctrl+C by default.
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	case appevents.Error:
		m.err = msg.Err
		return m, tea.Quit
	case appevents.AppFinishedMsg:
		return m, tea.Quit
	}

	switch m.mode {
	case Sender:
		return m.updateSender(msg)
	case Receiver:
		return m.updateReceiver(msg)
	}

	return m, nil
}
