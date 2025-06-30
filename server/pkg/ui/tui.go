package ui

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	dnssdlog "github.com/brutella/dnssd/log"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/rescp17/lanFileSharer/api"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
)

type mode int

type serverErrorMsg struct {
	err error
}

type foundServiceMsg struct {
	Services []discovery.ServiceInfo
}

const (
	None mode = iota
	Sender
	Receiver
)

type model struct {
	mode     mode
	port     int
	server   *http.Server // HTTP server for the TUI
	services []discovery.ServiceInfo
	spinner  spinner.Model
	table    table.Model
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

func InitialModel(m mode, port int) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		mode:     m,
		port:     port,
		services: []discovery.ServiceInfo{},
		spinner:  s,
	}
}

func (m model) Init() tea.Cmd {
	if m.mode == Receiver {
		return m.initReceiverModel()
	} else if m.mode == Sender {
		return m.initSenderModel()
	}
	return nil
}

func (m model) initReceiverModel() tea.Cmd {
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /ask", api.AskHandler) // Register the AskHandler

	m.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", m.port),
		Handler: mux,
	}

	hostname, err := os.Hostname()
	if err != nil {
		return func() tea.Msg {
			return serverErrorMsg{err}
		}
	}

	serviceUUID := uuid.New().String()

	serviceInfo := discovery.ServiceInfo{
		Name:   fmt.Sprintf("%s-%s", hostname, serviceUUID[:8]),
		Type:   discovery.DefaultServerType,
		Domain: discovery.DefaultDomain,
		Addr:   nil, // This will be set by the discovery package
		Port:   m.port,
	}
	// remove dnssd logging
	dnssdlog.Info.SetOutput(io.Discard)
	dnssdlog.Debug.SetOutput(io.Discard)

	Adapter := &discovery.MDNSAdapter{}

	go func() {
		if err := Adapter.Announce(context.TODO(), serviceInfo); err != nil {
			errCh <- err
			return
		}
		if err := m.server.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			if err := <-errCh; err != nil {
				return serverErrorMsg{err}
			}
			return nil
		},
	)
}

var readServices tea.Cmd

func (m model) initSenderModel() tea.Cmd {
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
		m.spinner.Tick,
		readServices,
		func() tea.Msg {
			if err := <-errCh; err != nil {
				return serverErrorMsg{err}
			}
			return nil
		},
	)
}

func (m model) View() string {
	if m.mode == Receiver {
		return m.receiverView()
	} else if m.mode == Sender {
		return m.senderView()
	}
	return ""
}

func (m model) senderView() string {
	s := ""
	length := len(m.services)
	if length < 1 {
		s += fmt.Sprintf("\n%s Finding receiver", m.spinner.View())
	} else {
		s += fmt.Sprintf("\n%s Found %d receiver(s)\n", m.spinner.View(), length)
		s += baseStyle.Render(m.table.View()) + "\n"
	}

	s += "\nPress q to quit.\n"

	return s
}

func (m model) receiverView() string {
	s := fmt.Sprintf("\n\n %s Awaiting sender connection", m.spinner.View())

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	switch msg := msg.(type) {
	case foundServiceMsg:
		m.services = msg.Services

		rows := []table.Row{}
		for index, svc := range m.services {
			rows = append(rows, table.Row{
				strconv.Itoa(index), svc.Name, svc.Type, svc.Addr.String(), strconv.Itoa(svc.Port),
			})
		}

		t := table.New(
			table.WithColumns(columns),
			table.WithRows(rows),
			table.WithFocused(true),
			table.WithHeight(len(m.services)+1),
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
		m.table = t

		return m, readServices

	// Is it a key press?
	case tea.KeyMsg:

		switch msg.String() {
		// These keys should exit the program.
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			// TODO
			return m, nil
		default:
			return m, nil
		}
	}
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}
