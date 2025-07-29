package ui

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/internal/app_events"
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
	if cmd, processed := m.handleSenderAppEvent(msg); processed {
		return m, cmd
	}
	var cmd tea.Cmd
	// Handle UI events
	switch m.sender.state {
	case selectingReceiver:
		cmd = m.updateSelectingReceiverState(msg)
	case selectingFiles:
		cmd = m.updateSelectingFilesState(msg)
	case transferComplete, transferFailed:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyEnter {
			m.sender.reset()
			m.sender.state = findingReceivers // Explicitly set state
			return m, m.initSender()
		}
	}

	var spinCmd tea.Cmd
	m.sender.spinner, spinCmd = m.sender.spinner.Update(msg)

	return m, tea.Batch(cmd, spinCmd)
}

func (m *model) handleSenderAppEvent(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case senderEvent.FoundServicesMsg:
		slog.Info("Discovery update", "service_count", len(msg.Services))
		for _, s := range msg.Services {
			slog.Debug("Found service", "name", s.Name, "addr", s.Addr, "port", s.Port)
		}

		if len(msg.Services) > 0 && m.sender.state == findingReceivers {
			m.sender.state = selectingReceiver
		}
		// If the list of services becomes empty, go back to the finding state.
		if len(msg.Services) == 0 && m.sender.state == selectingReceiver {
			m.sender.state = findingReceivers
		}

		m.updateReceiverTable(msg.Services)
		return m.listenForAppMessages(), true // Continue listening
	case senderEvent.TransferStartedMsg:
		m.sender.state = waitingForReceiverConfirmation
		return m.listenForAppMessages(), true
	case senderEvent.ReceiverAcceptedMsg:
		m.sender.state = sendingFiles
		return m.listenForAppMessages(), true
	case senderEvent.StatusUpdateMsg:
		// This could be used to update a status line in the UI
		slog.Info("Status Update", "message", msg.Message) // For now, just log
		return m.listenForAppMessages(), true
	case senderEvent.TransferCompleteMsg:
		m.sender.state = transferComplete
		return m.listenForAppMessages(), true
	case appevents.AppErrorMsg:
		m.sender.state = transferFailed
		return m.listenForAppMessages(), true
	}
	return nil, false
}

// updateSelectingReceiverState handles UI events for the selectingReceiver state.
func (m *model) updateSelectingReceiverState(msg tea.Msg) tea.Cmd {
	// ... logic for key presses and table updates
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if len(m.sender.services) > 0 {
				selectedIndex := m.sender.table.Cursor()
				if selectedIndex >= 0 && selectedIndex < len(m.sender.services) {
					m.err = nil // Reset any previous error
					m.sender.selectedService = m.sender.services[selectedIndex]
					m.sender.state = selectingFiles
				} else {
					// This case should ideally not be hit, but good to have for safety
					err := fmt.Errorf("internal error: cursor %d is out of sync with services list (len %d)", selectedIndex, len(m.sender.services))
					slog.Error("Cursor out of sync", "error", err)
					m.err = err
				}
				_, cmd := m.sender.table.Update(msg)
				return cmd
			}

		}
	}
	return nil
}

func (m *model) updateSelectingFilesState(msg tea.Msg) tea.Cmd {
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
	return cmd
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
		return "\nTransfer failed: \nPress Enter to try again."
	default:
		return "Internal error: unknown sender state"
	}
}

func (m *senderModel) reset() {
	*m = initSenderModel()
}
