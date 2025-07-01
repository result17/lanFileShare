package ui

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/charmbracelet/bubbles/table"
)

type foundServiceMsg struct {
	Services []discovery.ServiceInfo
}

type senderModel struct {
	port     int
	spinner  spinner.Model
	table    table.Model
	services []discovery.ServiceInfo
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

func initSenderModel(m mode, port int) model{
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		mode: m,
		sender: senderModel{
			spinner: s,
			port:    port,
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

func (m model) updateSender(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case foundServiceMsg:
		m.sender.services = msg.Services

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

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.sender.table.Focused() {
				m.sender.table.Blur()
			} else {
				m.sender.table.Focus()
			}
		case "enter":
			// TODO
			return m, nil
		}
	}

	// Only update the table if it has been initialized
	if m.sender.table.Columns() != nil {
		m.sender.table, cmd = m.sender.table.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.sender.spinner, cmd = m.sender.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) senderView() string {
	s := ""
	length := len(m.sender.services)
	if length < 1 {
		s += fmt.Sprintf("\n%s Finding receiver", m.sender.spinner.View())
	} else {
		s += fmt.Sprintf("\n%s Found %d receiver(s)\n", m.sender.spinner.View(), length)
		s += baseStyle.Render(m.sender.table.View()) + "\n"
	}

	s += "\nPress q to quit.\n"

	return s
}