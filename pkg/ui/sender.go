package ui

import (
	"fmt"
	"log"
	"log/slog"
	"strconv"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	senderEvent "github.com/rescp17/lanFileSharer/internal/app_events/sender"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/multiFilePicker"
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

func initSenderModel() senderModel {
	s := style.NewSpinner()

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(0),
	)

	t.SetStyles(style.NewTableStyles())

	return senderModel{
		spinner: s,
		fp:      multiFilePicker.InitialModel(),
		state:   findingReceivers,
		table:   t,
	}
}

// listenForAppMessages is a command that listens for messages from the app controller.
func (m *model) listenForAppMessages() tea.Cmd {
	return func() tea.Msg {
		return <-m.appController.UIMessages()
	}
}

func (m *model) initSender() tea.Cmd {
	return tea.Batch(m.sender.spinner.Tick, m.listenForAppMessages())
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
			m.appController.AppEvents() <- senderEvent.QuitAppMsg{}
			return m, tea.Quit
		}
	case senderEvent.FoundServicesMsg:
		log.Printf("Discovery Update: Found %d services.", len(msg.Services))
		for _, s := range msg.Services {
			log.Printf("  - Service: %s, Addr: %s, Port: %d", s.Name, s.Addr, s.Port)
		}

		if len(msg.Services) > 0 && m.sender.state == findingReceivers {
			m.sender.state = selectingReceiver
		}
		// If the list of services becomes empty, go back to the finding state.
		if len(msg.Services) == 0 && m.sender.state == selectingReceiver {
			m.sender.state = findingReceivers
		}

		m.updateReceiverTable(msg.Services)
		return m, m.listenForAppMessages() // Continue listening
	case senderEvent.TransferStartedMsg:
		m.sender.state = waitingForReceiverConfirmation
		return m, m.listenForAppMessages()
	case senderEvent.StatusUpdateMsg:
		// This could be used to update a status line in the UI
		slog.Info("Status Update", "message", msg.Message) // For now, just log
		return m, m.listenForAppMessages()
	case senderEvent.TransferCompleteMsg:
		m.sender.state = transferComplete
		return m, m.listenForAppMessages()
	case senderEvent.ErrorMsg:
		m.sender.state = transferFailed
		m.sender.lastError = msg.Err
		return m, m.listenForAppMessages()
	}

	// Handle UI events
	switch m.sender.state {
	case selectingReceiver:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "enter" {
				if len(m.sender.table.SelectedRow()) > 0 {
					selectedIndex, err := strconv.Atoi(m.sender.table.SelectedRow()[0])
					if err != nil {
						slog.Error("Failed to parse selected index", "error", err)
						return m, nil
					}
					if selectedIndex < 0 || selectedIndex >= len(m.sender.services) {
						err := fmt.Errorf("selected index %d out of range (0-%d)", selectedIndex, len(m.sender.services)-1)
						slog.Error("Selected index out of range", "error", err)
						return m, nil
					}
					m.sender.selectedService = m.sender.services[selectedIndex]
					m.sender.state = selectingFiles
					return m, nil

				}

			}
		}
		var cmd tea.Cmd
		m.sender.table, cmd = m.sender.table.Update(msg)
		cmds = append(cmds, cmd)

	case selectingFiles:
		switch msg := msg.(type) {
		case multiFilePicker.SelectedFileNodeMsg:
			// The app will now send messages about the transfer progress
			m.appController.AppEvents() <- senderEvent.SendFilesMsg{
				Receiver: m.sender.selectedService,
				Files:    msg.Files,
			}
		}
		newFpModel, cmd := m.sender.fp.Update(msg)
		m.sender.fp = newFpModel.(multiFilePicker.Model)
		cmds = append(cmds, cmd)

	case transferComplete, transferFailed:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
			m.sender = initSenderModel()
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
		s += style.BaseStyle.Render(m.sender.table.View()) + "\n"
		s += "Use arrow keys to navigate, Enter to select."
		return s
	case selectingFiles:
		return fmt.Sprintf("Receiver: %s\n%s\n", style.HighlightFontStyle.Render(m.sender.selectedService.Name), m.sender.fp.View())
	case waitingForReceiverConfirmation:
		return fmt.Sprintf("\n%s Waiting for %s to confirm...", m.sender.spinner.View(), style.HighlightFontStyle.Render(m.sender.selectedService.Name))
	case sendingFiles:
		return fmt.Sprintf("\n%s Sending files to %s...", m.sender.spinner.View(), style.HighlightFontStyle.Render(m.sender.selectedService.Name))
	case transferComplete:
		return "\nTransfer complete! 🎉\n\nPress Enter to send more files."
	case transferFailed:
		return fmt.Sprintf("\nTransfer failed: %v\n\nPress Enter to try again.", m.sender.lastError)
	default:
		return "Internal error: unknown sender state"
	}
}
