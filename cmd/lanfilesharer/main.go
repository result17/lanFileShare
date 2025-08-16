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

func runWithUIMode(mode ui.Mode, cmd *cobra.Command) {
	port, _ := cmd.Flags().GetInt("port")
	outputDir, _ := cmd.Flags().GetString("output")

	model := ui.InitialModel(mode, port, outputDir)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func main() {
	f, _ := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		if err := f.Close(); err != nil {
			slog.Warn("failed to close log file", "error", err)
		}
	}()
	log.SetOutput(f)

	cmd := &cobra.Command{
		Use:   "lanFileSharer",
		Short: "A file sharing application for local networks",
	}

	cmd.PersistentFlags().IntP("port", "p", 8080, "Port to listen on")
	
	cmd.PersistentFlags().StringP("output", "o", ".", "Output directory for received files")

	receiveCmd := &cobra.Command{
		Use:   "receive",
		Short: "Start the receiver mode",
		Run: func(cmd *cobra.Command, args []string) {
			runWithUIMode(ui.Receiver, cmd)
		},
	}

	sendCmd := &cobra.Command{
		Use:   "send",
		Short: "Start the sender mode",
		Run: func(cmd *cobra.Command, args []string) {
			runWithUIMode(ui.Sender, cmd)
		},
	}

	cmd.AddCommand(receiveCmd)
	cmd.AddCommand(sendCmd)

	if err := fang.Execute(context.Background(), cmd); err != nil {
		os.Exit(1)
	}
}
