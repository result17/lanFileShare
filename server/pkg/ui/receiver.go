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
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/rescp17/lanFileSharer/api"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
)

type receiverModel struct {
	spinner spinner.Model
	port    int
	server  *http.Server // HTTP server for the TUI
}

func initReceiverModel(m mode, port int) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		mode: m,
		receiver: receiverModel{
			spinner: s,
			port:    port,
		},
	}
}

func (m model) initReceiver() tea.Cmd {
	errCh := make(chan error, 1)

	// 1. Create a configured API instance.
	// All routing, middleware, and logic are handled within the API.
	apiHandler := api.NewAPI()

	// 2. Pass the API instance directly as the Handler to the server.
	m.receiver.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", m.receiver.port),
		Handler: apiHandler, // Use the API instance directly.
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
		Addr:   nil, // This will be set by the discovery package.
		Port:   m.receiver.port,
	}
	// remove dnssd logging
	dnssdlog.Info.SetOutput(io.Discard)
	dnssdlog.Debug.SetOutput(io.Discard)

	Adapter := &discovery.MDNSAdapter{}

	go func() {
		// Start service discovery and the HTTP server in the same goroutine
		// to better manage their lifecycle.
		go func() {
			if err := m.receiver.server.ListenAndServe(); err != http.ErrServerClosed {
				errCh <- err
			}
		}()

		if err := Adapter.Announce(context.TODO(), serviceInfo); err != nil {
			errCh <- err
			return
		}
	}()

	return tea.Batch(
		m.receiver.spinner.Tick,
		func() tea.Msg {
			// Listen for errors on the channel.
			if err := <-errCh; err != nil {
				return serverErrorMsg{err}
			}
			return nil
		},
	)
}

func (m model) receiverView() string {
	s := fmt.Sprintf("\n\n %s Awaiting sender connection", m.receiver.spinner.View())

	// Send the UI for rendering
	return s
}

func (m model) updateReceiver(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.receiver.spinner, cmd = m.receiver.spinner.Update(msg)
	return m, cmd
}
