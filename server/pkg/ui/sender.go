package ui

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/multiFilePicker"
	"github.com/rescp17/lanFileSharer/pkg/sender"
)

// senderState defines the different states of the sender UI.
type senderState int

const (
	findingReceivers senderState = iota
	selectingReceiver
	selectingFiles
	waitingForReceiverConfirmation
	sendingFiles
	transferComplete
	transferFailed
)

type senderModel struct {
	state           senderState
	app             *sender.App // The application logic layer
	spinner         spinner.Model
	table           table.Model
	fp              multiFilePicker.Model
	services        []discovery.ServiceInfo
	selectedService discovery.ServiceInfo
	lastError       error
}

var columns = []table.Column{
	{Title: "Index", Width: 10},
	{Title: "Name", Width: 20},
	{Title: "Address", Width: 20},
	{Title: "Port", Width: 10},
}

func initSenderModel() model {
	s := NewSpinner()

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(0),
	)

	t.SetStyles(NewTableStyles())

	return model{
		mode: Sender,
		sender: senderModel{
			spinner: s,
			fp:      multiFilePicker.InitialModel(),
			state:   findingReceivers,
			app:     sender.NewApp(),
			table:   t,
		},
	}
}

// listenForSenderAppMessages is a command that listens for messages from the sender app.
func (m *model) listenForSenderAppMessages() tea.Cmd {
	return func() tea.Msg {
		return <-m.sender.app.UIMessages()
	}
}

func (m *model) initSender() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	go m.sender.app.Run(ctx, cancel)
	return tea.Batch(m.sender.spinner.Tick, m.listenForSenderAppMessages())
}

func (m *model) updateReceiverTable(services []discovery.ServiceInfo) {
	m.sender.services = services
	rows := []table.Row{}
	for index, svc := range services {
		rows = append(rows, table.Row{
			strconv.Itoa(index), svc.Name, svc.Addr.String(), strconv.Itoa(svc.Port),
		})
	}
	m.sender.table.SetRows(rows)
	m.sender.table.SetHeight(len(rows) + 1)
}

func (m *model) updateSender(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle messages from the app logic layer first
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.sender.app.AppEvents() <- sender.QuitAppMsg{}
			return m, tea.Quit
		}
	case sender.FoundServicesMsg:
		if len(msg.Services) > 0 && m.sender.state == findingReceivers {
			m.sender.state = selectingReceiver
		}
		m.updateReceiverTable(msg.Services)
		return m, m.listenForSenderAppMessages() // Continue listening
	case sender.TransferStartedMsg:
		m.sender.state = waitingForReceiverConfirmation
		return m, m.listenForSenderAppMessages()
	case sender.StatusUpdateMsg:
		// This could be used to update a status line in the UI
		log.Println("Status Update:", msg.Message) // For now, just log
		return m, m.listenForSenderAppMessages()
	case sender.TransferCompleteMsg:
		m.sender.state = transferComplete
		return m, m.listenForSenderAppMessages()
	case sender.ErrorMsg:
		m.sender.state = transferFailed
		m.sender.lastError = msg.Err
		return m, m.listenForSenderAppMessages()
	}

	// Handle UI events
	switch m.sender.state {
	case selectingReceiver:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "enter" {
				if len(m.sender.table.SelectedRow()) > 0 {
					selectedIndex, _ := strconv.Atoi(m.sender.table.SelectedRow()[0])
					m.sender.selectedService = m.sender.services[selectedIndex]
					m.sender.state = selectingFiles
				}
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.sender.table, cmd = m.sender.table.Update(msg)
		cmds = append(cmds, cmd)

	case selectingFiles:
		switch msg := msg.(type) {
		case multiFilePicker.SelectedFileNodeMsg:
			// The app will now send messages about the transfer progress
			m.sender.app.AppEvents() <- sender.SendFilesMsg{
				Receiver: m.sender.selectedService,
				Files:    msg.Files,
			}
		}
		newFpModel, cmd := m.sender.fp.Update(msg)
		m.sender.fp = newFpModel.(multiFilePicker.Model)
		cmds = append(cmds, cmd)

	case transferComplete, transferFailed:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
			*m = initSenderModel()
			return m, m.initSender()
		}
	}

	var spinCmd tea.Cmd
	m.sender.spinner, spinCmd = m.sender.spinner.Update(msg)
	cmds = append(cmds, spinCmd)

	return m, tea.Batch(cmds...)
}

func (m *model) senderView() string {
	switch m.sender.state {
	case findingReceivers:
		return fmt.Sprintf("\n%s Finding receivers...", m.sender.spinner.View())
	case selectingReceiver:
		s := fmt.Sprintf("\n✔  Found %d receiver(s)\n", len(m.sender.services))
		s += BaseStyle.Render(m.sender.table.View()) + "\n"
		s += "Use arrow keys to navigate, Enter to select."
		return s
	case selectingFiles:
		return fmt.Sprintf("Receiver: %s\n%s\n", HighlightFontStyle.Render(m.sender.selectedService.Name), m.sender.fp.View())
	case waitingForReceiverConfirmation:
		return fmt.Sprintf("\n%s Waiting for %s to confirm...", m.sender.spinner.View(), HighlightFontStyle.Render(m.sender.selectedService.Name))
	case sendingFiles:
		return fmt.Sprintf("\n%s Sending files to %s...", m.sender.spinner.View(), HighlightFontStyle.Render(m.sender.selectedService.Name))
	case transferComplete:
		return "\nTransfer complete! 🎉\n\nPress Enter to send more files."
	case transferFailed:
		return fmt.Sprintf("\nTransfer failed: %v\n\nPress Enter to try again.", m.sender.lastError)
	default:
		return "Internal error: unknown sender state"
	}
}
