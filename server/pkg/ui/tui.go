package ui

import (
	tea "github.com/charmbracelet/bubbletea"
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
	mode     mode
	sender   senderModel
	receiver receiverModel
}

func InitialModel(m mode, port int) model {
	switch m {
	case Sender:
		return initSenderModel(m, port)
	case Receiver:
		return initReceiverModel(m, port)
	default:
		return model{
			mode: m,
		}
	}
}

func (m model) Init() tea.Cmd {
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
	switch m.mode {
	case Sender:
		return m.senderView()
	case Receiver:
		return m.receiverView()
	default:
		return ""
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	switch m.mode {
	case Sender:
		return m.updateSender(msg)
	case Receiver:
		return m.updateReceiver(msg)
	}

	return m, nil
}
