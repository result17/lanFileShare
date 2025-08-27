package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/rescp17/lanFileSharer/internal/style"
)

// ProgressBarConfig defines the configuration for a progress bar
type ProgressBarConfig struct {
	Width           int
	ShowPercentage  bool
	ShowBytes       bool
	ShowRate        bool
	ShowETA         bool
	Animated        bool
	CompactMode     bool
}

// DefaultProgressConfig returns a default progress bar configuration
func DefaultProgressConfig() ProgressBarConfig {
	return ProgressBarConfig{
		Width:          50,
		ShowPercentage: true,
		ShowBytes:      true,
		ShowRate:       true,
		ShowETA:        true,
		Animated:       true,
		CompactMode:    false,
	}
}

// CompactProgressConfig returns a compact progress bar configuration
func CompactProgressConfig() ProgressBarConfig {
	return ProgressBarConfig{
		Width:          30,
		ShowPercentage: true,
		ShowBytes:      false,
		ShowRate:       false,
		ShowETA:        false,
		Animated:       false,
		CompactMode:    true,
	}
}

// ProgressData contains all the data needed to render a progress bar
type ProgressData struct {
	Current     int64
	Total       int64
	Rate        float64 // bytes per second
	ETA         time.Duration
	Label       string
	Status      string // "active", "paused", "complete", "error"
	StartTime   time.Time
	CurrentFile string
}

// ProgressBar represents an enhanced progress bar component
type ProgressBar struct {
	config ProgressBarConfig
	data   ProgressData
}

// NewProgressBar creates a new progress bar with the given configuration
func NewProgressBar(config ProgressBarConfig) *ProgressBar {
	return &ProgressBar{
		config: config,
		data:   ProgressData{StartTime: time.Now()},
	}
}

// Update updates the progress bar data
func (pb *ProgressBar) Update(data ProgressData) {
	pb.data = data
	if pb.data.StartTime.IsZero() {
		pb.data.StartTime = time.Now()
	}
}

// Render renders the progress bar as a string
func (pb *ProgressBar) Render() string {
	if pb.config.CompactMode {
		return pb.renderCompact()
	}
	return pb.renderFull()
}

// renderFull renders the full progress bar with all details
func (pb *ProgressBar) renderFull() string {
	var result strings.Builder

	// Calculate progress percentage
	var percentage float64
	if pb.data.Total > 0 {
		percentage = float64(pb.data.Current) / float64(pb.data.Total) * 100
	}

	// Add label if provided
	if pb.data.Label != "" {
		result.WriteString(fmt.Sprintf("%s\n", style.HeaderStyle.Render(pb.data.Label)))
	}

	// Render the progress bar
	progressBar := pb.renderBar(percentage)
	result.WriteString(progressBar)

	// Add percentage if enabled
	if pb.config.ShowPercentage {
		result.WriteString(fmt.Sprintf(" %.1f%%", percentage))
	}

	result.WriteString("\n")

	// Add detailed information
	var details []string

	// Bytes information
	if pb.config.ShowBytes {
		details = append(details, pb.formatBytes(pb.data.Current, pb.data.Total))
	}

	// Transfer rate
	if pb.config.ShowRate && pb.data.Rate > 0 {
		details = append(details, pb.formatRate(pb.data.Rate))
	}

	// ETA
	if pb.config.ShowETA && pb.data.ETA > 0 {
		details = append(details, fmt.Sprintf("ETA: %s", pb.formatDuration(pb.data.ETA)))
	}

	// Elapsed time
	elapsed := time.Since(pb.data.StartTime)
	if elapsed > time.Second {
		details = append(details, fmt.Sprintf("Elapsed: %s", pb.formatDuration(elapsed)))
	}

	if len(details) > 0 {
		result.WriteString(style.FileStyle.Render(strings.Join(details, " | ")))
		result.WriteString("\n")
	}

	// Current file information
	if pb.data.CurrentFile != "" {
		result.WriteString(fmt.Sprintf("Current: %s\n", style.FileStyle.Render(pb.data.CurrentFile)))
	}

	// Status information
	if pb.data.Status != "" && pb.data.Status != "active" {
		statusStyle := pb.getStatusStyle(pb.data.Status)
		result.WriteString(fmt.Sprintf("Status: %s\n", statusStyle.Render(strings.ToUpper(pb.data.Status))))
	}

	return result.String()
}

// renderCompact renders a compact version of the progress bar
func (pb *ProgressBar) renderCompact() string {
	var percentage float64
	if pb.data.Total > 0 {
		percentage = float64(pb.data.Current) / float64(pb.data.Total) * 100
	}

	progressBar := pb.renderBar(percentage)
	
	result := progressBar
	if pb.config.ShowPercentage {
		result += fmt.Sprintf(" %.1f%%", percentage)
	}

	return result
}

// renderBar renders the actual progress bar visual
func (pb *ProgressBar) renderBar(percentage float64) string {
	filledWidth := int(float64(pb.config.Width) * percentage / 100.0)
	emptyWidth := pb.config.Width - filledWidth

	// Choose characters based on status and animation
	var filledChar, emptyChar string
	
	switch pb.data.Status {
	case "paused":
		filledChar = "▓"
		emptyChar = "░"
	case "error":
		filledChar = "▓"
		emptyChar = "░"
	case "complete":
		filledChar = "█"
		emptyChar = "░"
	default: // active
		if pb.config.Animated {
			// Use different characters for animation effect
			filledChar = "█"
			emptyChar = "░"
		} else {
			filledChar = "█"
			emptyChar = "░"
		}
	}

	// Apply styling based on status
	statusStyle := pb.getStatusStyle(pb.data.Status)
	
	filled := statusStyle.Render(strings.Repeat(filledChar, filledWidth))
	empty := style.FileStyle.Render(strings.Repeat(emptyChar, emptyWidth))

	return fmt.Sprintf("[%s%s]", filled, empty)
}

// getStatusStyle returns the appropriate style for the given status
func (pb *ProgressBar) getStatusStyle(status string) lipgloss.Style {
	switch status {
	case "paused":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	case "error":
		return style.ErrorStyle
	case "complete":
		return style.SuccessStyle
	default: // active
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // Blue
	}
}

// formatBytes formats byte counts in a human-readable format
func (pb *ProgressBar) formatBytes(current, total int64) string {
	if total > 1024*1024*1024 {
		return fmt.Sprintf("%.1f/%.1f GB",
			float64(current)/(1024*1024*1024),
			float64(total)/(1024*1024*1024))
	} else if total > 1024*1024 {
		return fmt.Sprintf("%.1f/%.1f MB",
			float64(current)/(1024*1024),
			float64(total)/(1024*1024))
	} else if total > 1024 {
		return fmt.Sprintf("%.1f/%.1f KB",
			float64(current)/1024,
			float64(total)/1024)
	}
	return fmt.Sprintf("%d/%d B", current, total)
}

// formatRate formats transfer rate in a human-readable format
func (pb *ProgressBar) formatRate(rate float64) string {
	if rate > 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", rate/(1024*1024))
	} else if rate > 1024 {
		return fmt.Sprintf("%.1f KB/s", rate/1024)
	}
	return fmt.Sprintf("%.0f B/s", rate)
}

// formatDuration formats duration in a human-readable format
func (pb *ProgressBar) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-60*d.Minutes())
	}
	return fmt.Sprintf("%.0fh %.0fm", d.Hours(), d.Minutes()-60*d.Hours())
}

// MultiFileProgress represents progress for multiple files
type MultiFileProgress struct {
	Files           []FileProgress
	OverallProgress ProgressData
	config          ProgressBarConfig
}

// FileProgress represents progress for a single file
type FileProgress struct {
	Name     string
	Progress ProgressData
	Status   string // "pending", "active", "complete", "error"
}

// NewMultiFileProgress creates a new multi-file progress tracker
func NewMultiFileProgress(config ProgressBarConfig) *MultiFileProgress {
	return &MultiFileProgress{
		config: config,
	}
}

// UpdateOverall updates the overall progress
func (mfp *MultiFileProgress) UpdateOverall(data ProgressData) {
	mfp.OverallProgress = data
}

// UpdateFile updates progress for a specific file
func (mfp *MultiFileProgress) UpdateFile(filename string, data ProgressData, status string) {
	for i, file := range mfp.Files {
		if file.Name == filename {
			mfp.Files[i].Progress = data
			mfp.Files[i].Status = status
			return
		}
	}
	// File not found, add it
	mfp.Files = append(mfp.Files, FileProgress{
		Name:     filename,
		Progress: data,
		Status:   status,
	})
}

// Render renders the multi-file progress display
func (mfp *MultiFileProgress) Render() string {
	var result strings.Builder

	// Overall progress
	overallBar := NewProgressBar(mfp.config)
	overallBar.Update(mfp.OverallProgress)
	result.WriteString("Overall Progress:\n")
	result.WriteString(overallBar.Render())
	result.WriteString("\n")

	// Individual file progress (show only active and recent files)
	activeFiles := 0
	for _, file := range mfp.Files {
		if file.Status == "active" || file.Status == "complete" {
			if activeFiles < 3 { // Limit to 3 files to avoid clutter
				compactConfig := CompactProgressConfig()
				fileBar := NewProgressBar(compactConfig)
				fileBar.Update(file.Progress)
				
				statusIcon := mfp.getFileStatusIcon(file.Status)
				result.WriteString(fmt.Sprintf("%s %s %s\n", 
					statusIcon, 
					style.FileStyle.Render(file.Name), 
					fileBar.Render()))
				activeFiles++
			}
		}
	}

	return result.String()
}

// getFileStatusIcon returns an icon for the file status
func (mfp *MultiFileProgress) getFileStatusIcon(status string) string {
	switch status {
	case "pending":
		return "⏳"
	case "active":
		return "🔄"
	case "complete":
		return "✅"
	case "error":
		return "❌"
	default:
		return "📄"
	}
}
