package ui

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/rescp17/lanFileSharer/pkg/multiFilePicker"
)

type foundServiceMsg struct {
	Services []discovery.ServiceInfo
}

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

func (s senderState) String() string {
	return [...]string{
		"Finding Receivers",
		"Selecting Receiver",
		"Selecting Files",
		"Waiting for Receiver's Confirmation",
		"Sending Files",
		"Transfer Complete",
		"Transfer Failed",
	}[s]
}

type senderModel struct {
	port            int
	state           senderState
	spinner         spinner.Model
	table           table.Model
	services        []discovery.ServiceInfo
	selectedService table.Row
	fp              multiFilePicker.Model
	fileInfos       []*fileInfo.FileNode
}

var columns = []table.Column{
	{Title: "Index", Width: 10},
	{Title: "Name", Width: 16},
	{Title: "Type", Width: 24},
	{Title: "Address", Width: 16},
	{Title: "Port", Width: 10},
}

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

var highLightFontStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

func initSenderModel(m mode, port int) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		mode: m,
		sender: senderModel{
			spinner: s,
			port:    port,
			fp:      multiFilePicker.InitialModel(),
			state:   findingReceivers,
		},
	}
}

var readServices tea.Cmd

func (m model) initSender() tea.Cmd {
	errCh := make(chan error, 1)
	Adapter := &discovery.MDNSAdapter{}
	serviceChan, err := Adapter.Discover(context.TODO(), fmt.Sprintf("%s.%s.", discovery.DefaultServerType, discovery.DefaultDomain))

	readServices = func() tea.Msg {
		if err != nil {
			return serverErrorMsg{err}
		}
		services, ok := <-serviceChan
		log.Printf("Found services: %#v", services)
		if !ok {
			return nil
		}
		return foundServiceMsg{Services: services}
	}
	return tea.Batch(
		m.sender.spinner.Tick,
		readServices,
		func() tea.Msg {
			if err := <-errCh; err != nil {
				return serverErrorMsg{err}
			}
			return nil
		},
	)
}

func createReceiverTable(m model, services []discovery.ServiceInfo) (tea.Model, tea.Cmd) {
	m.sender.services = services

	rows := []table.Row{}
	for index, svc := range m.sender.services {
		rows = append(rows, table.Row{
			strconv.Itoa(index), svc.Name, svc.Type, svc.Addr.String(), strconv.Itoa(svc.Port),
		})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(len(m.sender.services)+1),
	)
	style := table.DefaultStyles()
	style.Header = style.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	style.Selected = style.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(style)
	m.sender.table = t

	return m, readServices
}

func (m model) updateSender(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch m.sender.state {
	case findingReceivers:
		switch msg := msg.(type) {
		case foundServiceMsg:
			if len(msg.Services) > 0 {
				m.sender.state = selectingReceiver
				return createReceiverTable(m, msg.Services)
			}
			return m, readServices // continue waiting for services
		}

	case selectingReceiver:
		switch msg := msg.(type) {
		case foundServiceMsg:
			// Refresh the list of services
			return createReceiverTable(m, msg.Services)
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				m.sender.selectedService = m.sender.table.SelectedRow()
				m.sender.state = selectingFiles
				return m, nil
			case "esc":
				m.sender.table.Blur()
				return m, nil
			}
		}
		// Update table
		m.sender.table, cmd = m.sender.table.Update(msg)
		cmds = append(cmds, cmd)

	case selectingFiles:
		switch msg := msg.(type) {
		case multiFilePicker.SelectedFileNodeMsg:
			log.Printf("selected %v", msg.Infos)
			// Here you would initiate the connection to the receiver
			// and wait for confirmation.
			m.sender.fileInfos = msg.Infos
			m.sender.state = waitingForReceiverConfirmation
			// For now, we'll just log it and show a placeholder state.
			// return m, waitForConfirmationCmd()
			return m, nil
		case tea.KeyMsg:
			// Handle key presses for the file picker
			newFpModel, cmd := m.sender.fp.Update(msg)
			m.sender.fp = newFpModel.(multiFilePicker.Model)
			return m, cmd
		}

	case waitingForReceiverConfirmation:
		// Here you would handle messages like `receiverAcceptedMsg` or `receiverRejectedMsg`
		// For now, it's a placeholder.
		// On Enter, we can simulate moving to the next state for demonstration
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "enter" {
				m.sender.state = sendingFiles
			}
		}

	case sendingFiles:
		// Here you would handle file transfer progress messages.
		// On Enter, we can simulate completion for demonstration
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "enter" {
				m.sender.state = transferComplete
			}
		}

	case transferComplete, transferFailed:
		// User can press enter to go back to the beginning
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "enter" {
				return initSenderModel(m.mode, m.sender.port), m.initSender()
			}
		}
	}

	m.sender.spinner, cmd = m.sender.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) senderView() string {
	switch m.sender.state {
	case findingReceivers:
		return fmt.Sprintf("\n%s Finding receiver...", m.sender.spinner.View())
	case selectingReceiver:
		s := fmt.Sprintf("\n%s Found %d receiver(s)\n", "✔", len(m.sender.services))
		s += baseStyle.Render(m.sender.table.View()) + "\n"
		s += "Press Enter to select a receiver."
		return s
	case selectingFiles:
		return fmt.Sprintf("Receiver: %s\n%s\n", highLightFontStyle.Render(m.sender.selectedService[1]), m.sender.fp.View())
	case waitingForReceiverConfirmation:
		var sb strings.Builder
		sb.WriteString("Select files and directory:\n")
		for index, info := range m.sender.fileInfos {
			if info.IsDir {
				sb.WriteString(DirStyle.Render(fmt.Sprintf("%d (directory). %s\n", index, info.Path)))
			} else {
				sb.WriteString(fmt.Sprintf("%d (file). %s\n", index, info.Path))
			}
		}

		return fmt.Sprintf("%s\nWaiting for %s to confirm the transfer...\n\n(Press Enter to simulate confirmation)", sb.String(), highLightFontStyle.Render(m.sender.selectedService[1]))
	case sendingFiles:
		// In a real implementation, you'd have a progress bar here.
		return fmt.Sprintf("\n%s Sending files to %s...\n\n(Press Enter to simulate completion)", m.sender.spinner.View(), highLightFontStyle.Render(m.sender.selectedService[1]))
	case transferComplete:
		return "\nTransfer complete! 🎉\n\nPress Enter to send more files."
	case transferFailed:
		return "\nTransfer failed. 😞\n\nPress Enter to try again."
	default:
		return "Internal error: unknown sender state"
	}
}
