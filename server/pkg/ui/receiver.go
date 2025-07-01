package ui

import (
	"net/http"
	"fmt"
	"context"
	"io"
	"os"

	"github.com/google/uuid"
	dnssdlog "github.com/brutella/dnssd/log"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/api"
	tea "github.com/charmbracelet/bubbletea"
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

	mux := http.NewServeMux()
	mux.HandleFunc("POST /ask", api.AskHandler) // Register the AskHandler

	m.receiver.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", m.receiver.port),
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
		Port:   m.receiver.port,
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
		if err := m.receiver.server.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	return tea.Batch(
		m.receiver.spinner.Tick,
		func() tea.Msg {
			if err := <-errCh; err != nil {
				return serverErrorMsg{err}
			}
			return nil
		},
	)
}

func (m model) receiverView() string {
	s := fmt.Sprintf("\n\n %s Awaiting sender connection", m.receiver.spinner.View())

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}

func (m model) updateReceiver(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.receiver.spinner, cmd = m.receiver.spinner.Update(msg)
	return m, cmd
}

