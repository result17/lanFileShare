package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	receiverApp "github.com/rescp17/lanFileSharer/pkg/receiver"
	senderApp "github.com/rescp17/lanFileSharer/pkg/sender"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
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

	return model{
		mode:          m,
		appController: appController,
		sender:        sender,
		receiver:      receiver,
	}
}

func (m model) Init() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	go m.appController.Run(ctx, cancel)

	switch m.mode {
	case Sender:
		return m.initSender()
	case Receiver:
		return m.initReceiver()
	default:
		return nil
	}
}

func (m model) View() string {
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
	switch m.mode {
	case Sender:
		return m.updateSender(msg)
	case Receiver:
		return m.updateReceiver(msg)
	}

	return m, nil
}
