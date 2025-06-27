package ui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	dnssdlog "github.com/brutella/dnssd/log"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/api"
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
	mode     mode
	server   *http.Server // HTTP server for the TUI
	services []discovery.ServiceInfo
	port     int           // port to listen on
	spinner  spinner.Model // choices to display in the UI
}

func InitialModel(m mode, port int) model {
	return model{
		mode:     m,
		port:     port,
		services: []discovery.ServiceInfo{},
		spinner:  spinner.New(),
	}
}

func (m model) Init() tea.Cmd {
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

	serviceInfo := discovery.ServiceInfo{
		Name:   hostname,
		Type:   discovery.DefaultServerType,
		Domain: discovery.DefaultDomain,
		Addr:   nil, // This will be set by the discovery package
		Port:   m.port,
	}
	// remove dnssd logging
	dnssdlog.Info.SetOutput(io.Discard)
	dnssdlog.Debug.SetOutput(io.Discard)

	Adapter := &discovery.MDNSAdapter{}

	return func() tea.Msg {
		if err := Adapter.Announce(context.TODO(), serviceInfo); err != nil {
			return serverErrorMsg{err}
		}
		if err := m.server.ListenAndServe(); err != http.ErrServerClosed {
			return serverErrorMsg{err}
		}
		m.spinner.Tick()
		return nil
	}
}

func (m model) View() string {
	s := fmt.Sprintf("\n\n %s Awaiting sender connectiong", m.spinner.View())

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}
