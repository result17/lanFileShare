package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/rescp17/lanFileSharer/pkg/ui"
)

func main() {
	var port int
	cmd := &cobra.Command{
		Use:   "lanFileSharer",
		Short: "A file sharing application for local networks",
	}

	cmd.Flags().IntVar(&port, "port", 8080, "Port to listen on")

	cmd.AddCommand(
		&cobra.Command{
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
		})

	if err := fang.Execute(context.Background(), cmd); err != nil {
		os.Exit(1)
	}
}
