package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/rescp17/lanFileSharer/pkg/ui"
)

// main is the entry point for the TUI example.
func main() {
	// 1. Create some sample FileNode data to display.
	// This simulates a file structure you would receive from the sender.
	sampleNodes := createSampleData()

	// 2. Create a new file tree model with a title and the data.
	fileTreeModel := ui.NewFileTree("Remote Files", sampleNodes)

	// 3. Start the Bubble Tea program.
	p := tea.NewProgram(fileTreeModel)
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running file tree TUI: %v", err)
		os.Exit(1)
	}

	fmt.Println("\nTUI exited.")
}

// createSampleData generates a tree of FileNodes for demonstration.
func createSampleData() []fileInfo.FileNode {
	return []fileInfo.FileNode{
		{
			Name:  "Documents",
			IsDir: true,
			Children: []fileInfo.FileNode{
				{Name: "report.docx", IsDir: false, Size: 12345},
				{Name: "presentation.pptx", IsDir: false, Size: 67890},
			},
		},
		{
			Name:  "Pictures",
			IsDir: true,
			Children: []fileInfo.FileNode{
				{
					Name:  "Vacation",
					IsDir: true,
					Children: []fileInfo.FileNode{
						{Name: "photo1.jpg", IsDir: false, Size: 102400},
						{Name: "photo2.png", IsDir: false, Size: 204800},
					},
				},
				{Name: "profile.jpg", IsDir: false, Size: 51200},
			},
		},
		{
			Name:  "music.mp3",
			IsDir: false,
			Size:  5242880,
		},
		{
			Name:  "archive.zip",
			IsDir: false,
			Size:  10485760,
		},
	}
}
