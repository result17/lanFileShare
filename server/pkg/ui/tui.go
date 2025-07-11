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
		return initSenderModel()
	case Receiver:
		return initReceiverModel(port)
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
	var s string
	switch m.mode {
	case Sender:
		s += m.senderView()
	case Receiver:
		s += m.receiverView()
	default:
		return ""
	}
	s += "\n press ctrl + c to quit"
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
