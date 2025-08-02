package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/rescp17/lanFileSharer/pkg/ui"
)

func main() {
	f, _ := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		if err := f.Close(); err != nil {
			slog.Warn("failed to close log file", "error", err)
		}
	}()
	log.SetOutput(f)

	var port int
	cmd := &cobra.Command{
		Use:   "lanFileSharer",
		Short: "A file sharing application for local networks",
	}

	cmd.PersistentFlags().IntVar(&port, "port", 8080, "Port to listen on")

	receiveCmd := &cobra.Command{
		Use:   "receive",
		Short: "Start the receiver mode",
		Run: func(cmd *cobra.Command, args []string) {
			mode := ui.Receiver
			model := ui.InitialModel(mode, port)
			p := tea.NewProgram(model)
			if _, err := p.Run(); err != nil {
				fmt.Printf("Alas, there's been an error: %v", err)
				os.Exit(1)
			}
		},
	}

	sendCmd := &cobra.Command{
		Use:   "send",
		Short: "Start the sender mode",
		Run: func(cmd *cobra.Command, args []string) {
			mode := ui.Sender
			model := ui.InitialModel(mode, port)
			p := tea.NewProgram(model)
			if _, err := p.Run(); err != nil {
				fmt.Printf("Alas, there's been an error: %v", err)
				os.Exit(1)
			}
		},
	}

	cmd.AddCommand(receiveCmd)
	cmd.AddCommand(sendCmd)

	if err := fang.Execute(context.Background(), cmd); err != nil {
		os.Exit(1)
	}
}
