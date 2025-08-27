package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/rescp17/lanFileSharer/internal/style"
)

// StatusLevel represents the severity level of a status
type StatusLevel int

const (
	StatusInfo StatusLevel = iota
	StatusSuccess
	StatusWarning
	StatusError
)

// StatusMessage represents a status message with metadata
type StatusMessage struct {
	Level     StatusLevel
	Message   string
	Timestamp time.Time
	Details   string
	Action    string // Suggested action for the user
}

// StatusIndicator manages and displays status messages
type StatusIndicator struct {
	messages    []StatusMessage
	maxMessages int
	showTime    bool
	compact     bool
}

// NewStatusIndicator creates a new status indicator
func NewStatusIndicator(maxMessages int, showTime bool) *StatusIndicator {
	return &StatusIndicator{
		messages:    make([]StatusMessage, 0),
		maxMessages: maxMessages,
		showTime:    showTime,
		compact:     false,
	}
}

// SetCompact sets whether to use compact display mode
func (si *StatusIndicator) SetCompact(compact bool) {
	si.compact = compact
}

// AddMessage adds a new status message
func (si *StatusIndicator) AddMessage(level StatusLevel, message string) {
	si.AddDetailedMessage(level, message, "", "")
}

// AddDetailedMessage adds a status message with details and suggested action
func (si *StatusIndicator) AddDetailedMessage(level StatusLevel, message, details, action string) {
	msg := StatusMessage{
		Level:     level,
		Message:   message,
		Timestamp: time.Now(),
		Details:   details,
		Action:    action,
	}

	si.messages = append(si.messages, msg)

	// Keep only the most recent messages
	if len(si.messages) > si.maxMessages {
		si.messages = si.messages[len(si.messages)-si.maxMessages:]
	}
}

// Clear clears all status messages
func (si *StatusIndicator) Clear() {
	si.messages = si.messages[:0]
}

// GetLatest returns the most recent status message
func (si *StatusIndicator) GetLatest() *StatusMessage {
	if len(si.messages) == 0 {
		return nil
	}
	return &si.messages[len(si.messages)-1]
}

// Render renders all status messages
func (si *StatusIndicator) Render() string {
	if len(si.messages) == 0 {
		return ""
	}

	if si.compact {
		return si.renderCompact()
	}
	return si.renderFull()
}

// renderFull renders all messages with full details
func (si *StatusIndicator) renderFull() string {
	var result strings.Builder

	for i, msg := range si.messages {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(si.renderMessage(msg, false))
	}

	return result.String()
}

// renderCompact renders only the latest message in compact form
func (si *StatusIndicator) renderCompact() string {
	if len(si.messages) == 0 {
		return ""
	}

	latest := si.messages[len(si.messages)-1]
	return si.renderMessage(latest, true)
}

// renderMessage renders a single status message
func (si *StatusIndicator) renderMessage(msg StatusMessage, compact bool) string {
	var result strings.Builder

	// Get icon and style for the status level
	icon := si.getStatusIcon(msg.Level)
	msgStyle := si.getStatusStyle(msg.Level)

	// Add timestamp if enabled and not compact
	if si.showTime && !compact {
		timeStr := msg.Timestamp.Format("15:04:05")
		result.WriteString(style.FileStyle.Render(fmt.Sprintf("[%s] ", timeStr)))
	}

	// Add icon and message
	result.WriteString(fmt.Sprintf("%s %s", icon, msgStyle.Render(msg.Message)))

	// Add details if available and not compact
	if msg.Details != "" && !compact {
		result.WriteString(fmt.Sprintf("\n   %s", style.FileStyle.Render(msg.Details)))
	}

	// Add suggested action if available and not compact
	if msg.Action != "" && !compact {
		actionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Italic(true)
		result.WriteString(fmt.Sprintf("\n   💡 %s", actionStyle.Render(msg.Action)))
	}

	return result.String()
}

// getStatusIcon returns the appropriate icon for the status level
func (si *StatusIndicator) getStatusIcon(level StatusLevel) string {
	switch level {
	case StatusInfo:
		return "ℹ️"
	case StatusSuccess:
		return "✅"
	case StatusWarning:
		return "⚠️"
	case StatusError:
		return "❌"
	default:
		return "📝"
	}
}

// getStatusStyle returns the appropriate style for the status level
func (si *StatusIndicator) getStatusStyle(level StatusLevel) lipgloss.Style {
	switch level {
	case StatusInfo:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // Blue
	case StatusSuccess:
		return style.SuccessStyle
	case StatusWarning:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	case StatusError:
		return style.ErrorStyle
	default:
		return style.FileStyle
	}
}

// NetworkQualityIndicator shows network connection quality
type NetworkQualityIndicator struct {
	latency    time.Duration
	packetLoss float64
	bandwidth  float64 // bytes per second
	quality    string  // "excellent", "good", "fair", "poor"
}

// NewNetworkQualityIndicator creates a new network quality indicator
func NewNetworkQualityIndicator() *NetworkQualityIndicator {
	return &NetworkQualityIndicator{
		quality: "unknown",
	}
}

// Update updates the network quality metrics
func (nqi *NetworkQualityIndicator) Update(latency time.Duration, packetLoss, bandwidth float64) {
	nqi.latency = latency
	nqi.packetLoss = packetLoss
	nqi.bandwidth = bandwidth
	nqi.quality = nqi.calculateQuality()
}

// calculateQuality calculates overall network quality based on metrics
func (nqi *NetworkQualityIndicator) calculateQuality() string {
	score := 100.0

	// Latency impact (0-40 points deduction)
	if nqi.latency > 200*time.Millisecond {
		score -= 40
	} else if nqi.latency > 100*time.Millisecond {
		score -= 20
	} else if nqi.latency > 50*time.Millisecond {
		score -= 10
	}

	// Packet loss impact (0-40 points deduction)
	if nqi.packetLoss > 5.0 {
		score -= 40
	} else if nqi.packetLoss > 2.0 {
		score -= 20
	} else if nqi.packetLoss > 1.0 {
		score -= 10
	}

	// Bandwidth impact (0-20 points deduction)
	if nqi.bandwidth < 1024*1024 { // < 1 MB/s
		score -= 20
	} else if nqi.bandwidth < 10*1024*1024 { // < 10 MB/s
		score -= 10
	}

	// Determine quality level
	if score >= 80 {
		return "excellent"
	} else if score >= 60 {
		return "good"
	} else if score >= 40 {
		return "fair"
	}
	return "poor"
}

// Render renders the network quality indicator
func (nqi *NetworkQualityIndicator) Render() string {
	if nqi.quality == "unknown" {
		return "📶 Network: Unknown"
	}

	icon := nqi.getQualityIcon()
	qualityStyle := nqi.getQualityStyle()
	
	result := fmt.Sprintf("%s Network: %s", icon, qualityStyle.Render(strings.ToUpper(nqi.quality)))

	// Add detailed metrics
	if nqi.latency > 0 {
		result += fmt.Sprintf(" (Latency: %dms", nqi.latency.Milliseconds())
		
		if nqi.packetLoss > 0 {
			result += fmt.Sprintf(", Loss: %.1f%%", nqi.packetLoss)
		}
		
		if nqi.bandwidth > 0 {
			if nqi.bandwidth > 1024*1024 {
				result += fmt.Sprintf(", BW: %.1f MB/s", nqi.bandwidth/(1024*1024))
			} else {
				result += fmt.Sprintf(", BW: %.1f KB/s", nqi.bandwidth/1024)
			}
		}
		
		result += ")"
	}

	return result
}

// getQualityIcon returns the appropriate icon for network quality
func (nqi *NetworkQualityIndicator) getQualityIcon() string {
	switch nqi.quality {
	case "excellent":
		return "📶"
	case "good":
		return "📶"
	case "fair":
		return "📶"
	case "poor":
		return "📵"
	default:
		return "📶"
	}
}

// getQualityStyle returns the appropriate style for network quality
func (nqi *NetworkQualityIndicator) getQualityStyle() lipgloss.Style {
	switch nqi.quality {
	case "excellent":
		return style.SuccessStyle
	case "good":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // Blue
	case "fair":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	case "poor":
		return style.ErrorStyle
	default:
		return style.FileStyle
	}
}

// TransferStatsPanel shows comprehensive transfer statistics
type TransferStatsPanel struct {
	startTime       time.Time
	totalFiles      int
	completedFiles  int
	failedFiles     int
	totalBytes      int64
	transferredBytes int64
	currentRate     float64
	averageRate     float64
	peakRate        float64
	networkQuality  *NetworkQualityIndicator
	compact         bool
}

// NewTransferStatsPanel creates a new transfer statistics panel
func NewTransferStatsPanel() *TransferStatsPanel {
	return &TransferStatsPanel{
		startTime:      time.Now(),
		networkQuality: NewNetworkQualityIndicator(),
	}
}

// SetCompact sets whether to use compact display mode
func (tsp *TransferStatsPanel) SetCompact(compact bool) {
	tsp.compact = compact
}

// Update updates the transfer statistics
func (tsp *TransferStatsPanel) Update(totalFiles, completedFiles, failedFiles int, 
	totalBytes, transferredBytes int64, currentRate, averageRate, peakRate float64) {
	tsp.totalFiles = totalFiles
	tsp.completedFiles = completedFiles
	tsp.failedFiles = failedFiles
	tsp.totalBytes = totalBytes
	tsp.transferredBytes = transferredBytes
	tsp.currentRate = currentRate
	tsp.averageRate = averageRate
	tsp.peakRate = peakRate
}

// UpdateNetworkQuality updates network quality metrics
func (tsp *TransferStatsPanel) UpdateNetworkQuality(latency time.Duration, packetLoss, bandwidth float64) {
	tsp.networkQuality.Update(latency, packetLoss, bandwidth)
}

// Render renders the transfer statistics panel
func (tsp *TransferStatsPanel) Render() string {
	if tsp.compact {
		return tsp.renderCompact()
	}
	return tsp.renderFull()
}

// renderFull renders the full statistics panel
func (tsp *TransferStatsPanel) renderFull() string {
	var result strings.Builder

	// Header
	result.WriteString(style.HeaderStyle.Render("📊 Transfer Statistics"))
	result.WriteString("\n\n")

	// File statistics
	result.WriteString(fmt.Sprintf("Files: %d total, %d completed", tsp.totalFiles, tsp.completedFiles))
	if tsp.failedFiles > 0 {
		result.WriteString(fmt.Sprintf(", %s failed", style.ErrorStyle.Render(fmt.Sprintf("%d", tsp.failedFiles))))
	}
	result.WriteString("\n")

	// Data statistics
	result.WriteString(fmt.Sprintf("Data: %s\n", formatBytes(tsp.transferredBytes, tsp.totalBytes)))

	// Rate statistics
	result.WriteString(fmt.Sprintf("Current Rate: %s\n", formatRate(tsp.currentRate)))
	result.WriteString(fmt.Sprintf("Average Rate: %s\n", formatRate(tsp.averageRate)))
	if tsp.peakRate > 0 {
		result.WriteString(fmt.Sprintf("Peak Rate: %s\n", formatRate(tsp.peakRate)))
	}

	// Time statistics
	elapsed := time.Since(tsp.startTime)
	result.WriteString(fmt.Sprintf("Elapsed: %s\n", formatDuration(elapsed)))

	// Network quality
	result.WriteString(fmt.Sprintf("\n%s\n", tsp.networkQuality.Render()))

	return result.String()
}

// renderCompact renders a compact version of the statistics
func (tsp *TransferStatsPanel) renderCompact() string {
	elapsed := time.Since(tsp.startTime)
	return fmt.Sprintf("📊 %d/%d files | %s | %s | %s",
		tsp.completedFiles, tsp.totalFiles,
		formatRate(tsp.currentRate),
		formatDuration(elapsed),
		tsp.networkQuality.quality)
}

// Helper functions (reuse from progress.go)
func formatBytes(current, total int64) string {
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

func formatRate(rate float64) string {
	if rate > 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", rate/(1024*1024))
	} else if rate > 1024 {
		return fmt.Sprintf("%.1f KB/s", rate/1024)
	}
	return fmt.Sprintf("%.0f B/s", rate)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-60*d.Minutes())
	}
	return fmt.Sprintf("%.0fh %.0fm", d.Hours(), d.Minutes()-60*d.Hours())
}
