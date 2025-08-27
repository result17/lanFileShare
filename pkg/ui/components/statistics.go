package components

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/rescp17/lanFileSharer/internal/style"
)

// TransferMetrics contains detailed transfer metrics
type TransferMetrics struct {
	StartTime          time.Time
	LastUpdateTime     time.Time
	TotalBytes         int64
	TransferredBytes   int64
	CurrentRate        float64 // bytes per second
	AverageRate        float64
	PeakRate           float64
	MinRate            float64
	RateHistory        []RatePoint
	TotalFiles         int
	CompletedFiles     int
	FailedFiles        int
	ActiveConnections  int
	NetworkLatency     time.Duration
	PacketLoss         float64
	Jitter             time.Duration
	RetransmissionRate float64
}

// RatePoint represents a point in time with transfer rate
type RatePoint struct {
	Timestamp time.Time
	Rate      float64
	Bytes     int64
}

// FileMetrics contains metrics for individual files
type FileMetrics struct {
	Name             string
	Size             int64
	TransferredBytes int64
	StartTime        time.Time
	EndTime          *time.Time
	Rate             float64
	Status           string // "pending", "active", "complete", "failed", "paused"
	ErrorCount       int
	RetryCount       int
}

// AdvancedStatsCollector collects and analyzes transfer statistics
type AdvancedStatsCollector struct {
	metrics        TransferMetrics
	fileMetrics    map[string]*FileMetrics
	maxHistory     int
	updateInterval time.Duration
	lastUpdate     time.Time
}

// NewAdvancedStatsCollector creates a new advanced statistics collector
func NewAdvancedStatsCollector(maxHistory int, updateInterval time.Duration) *AdvancedStatsCollector {
	return &AdvancedStatsCollector{
		metrics: TransferMetrics{
			StartTime:   time.Now(),
			RateHistory: make([]RatePoint, 0, maxHistory),
			MinRate:     math.MaxFloat64,
		},
		fileMetrics:    make(map[string]*FileMetrics),
		maxHistory:     maxHistory,
		updateInterval: updateInterval,
		lastUpdate:     time.Now(),
	}
}

// UpdateTransferMetrics updates the overall transfer metrics
func (asc *AdvancedStatsCollector) UpdateTransferMetrics(totalBytes, transferredBytes int64, currentRate float64) {
	now := time.Now()

	asc.metrics.LastUpdateTime = now
	asc.metrics.TotalBytes = totalBytes
	asc.metrics.TransferredBytes = transferredBytes
	asc.metrics.CurrentRate = currentRate

	// Update rate statistics
	if currentRate > asc.metrics.PeakRate {
		asc.metrics.PeakRate = currentRate
	}
	if currentRate < asc.metrics.MinRate && currentRate > 0 {
		asc.metrics.MinRate = currentRate
	}

	// Calculate average rate
	elapsed := now.Sub(asc.metrics.StartTime).Seconds()
	if elapsed > 0 {
		asc.metrics.AverageRate = float64(transferredBytes) / elapsed
	}

	// Add to rate history
	if now.Sub(asc.lastUpdate) >= asc.updateInterval {
		asc.addRatePoint(now, currentRate, transferredBytes)
		asc.lastUpdate = now
	}
}

// UpdateFileMetrics updates metrics for a specific file
func (asc *AdvancedStatsCollector) UpdateFileMetrics(filename string, size, transferredBytes int64, status string) {
	if _, exists := asc.fileMetrics[filename]; !exists {
		asc.fileMetrics[filename] = &FileMetrics{
			Name:      filename,
			Size:      size,
			StartTime: time.Now(),
			Status:    "pending",
		}
	}

	file := asc.fileMetrics[filename]
	file.TransferredBytes = transferredBytes
	file.Status = status

	// Calculate file transfer rate
	if file.Status == "active" {
		elapsed := time.Since(file.StartTime).Seconds()
		if elapsed > 0 {
			file.Rate = float64(transferredBytes) / elapsed
		}
	}

	// Mark completion time
	if status == "complete" && file.EndTime == nil {
		now := time.Now()
		file.EndTime = &now
		asc.metrics.CompletedFiles++
	} else if status == "failed" {
		file.ErrorCount++
		asc.metrics.FailedFiles++
	}
}

// UpdateNetworkMetrics updates network quality metrics
func (asc *AdvancedStatsCollector) UpdateNetworkMetrics(latency time.Duration, packetLoss float64, jitter time.Duration, retransmissionRate float64) {
	asc.metrics.NetworkLatency = latency
	asc.metrics.PacketLoss = packetLoss
	asc.metrics.Jitter = jitter
	asc.metrics.RetransmissionRate = retransmissionRate
}

// addRatePoint adds a new rate point to the history
func (asc *AdvancedStatsCollector) addRatePoint(timestamp time.Time, rate float64, bytes int64) {
	point := RatePoint{
		Timestamp: timestamp,
		Rate:      rate,
		Bytes:     bytes,
	}

	asc.metrics.RateHistory = append(asc.metrics.RateHistory, point)

	// Keep only the most recent points
	if len(asc.metrics.RateHistory) > asc.maxHistory {
		asc.metrics.RateHistory = asc.metrics.RateHistory[1:]
	}
}

// GetMetrics returns the current transfer metrics
func (asc *AdvancedStatsCollector) GetMetrics() TransferMetrics {
	return asc.metrics
}

// GetFileMetrics returns metrics for all files
func (asc *AdvancedStatsCollector) GetFileMetrics() map[string]*FileMetrics {
	return asc.fileMetrics
}

// GetTopFiles returns the top N files by various criteria
func (asc *AdvancedStatsCollector) GetTopFiles(n int, criteria string) []*FileMetrics {
	files := make([]*FileMetrics, 0, len(asc.fileMetrics))
	for _, file := range asc.fileMetrics {
		files = append(files, file)
	}

	// Sort by criteria
	switch criteria {
	case "size":
		sort.Slice(files, func(i, j int) bool {
			return files[i].Size > files[j].Size
		})
	case "rate":
		sort.Slice(files, func(i, j int) bool {
			return files[i].Rate > files[j].Rate
		})
	case "errors":
		sort.Slice(files, func(i, j int) bool {
			return files[i].ErrorCount > files[j].ErrorCount
		})
	default: // "progress"
		sort.Slice(files, func(i, j int) bool {
			progressI := float64(files[i].TransferredBytes) / float64(files[i].Size)
			progressJ := float64(files[j].TransferredBytes) / float64(files[j].Size)
			return progressI > progressJ
		})
	}

	if n > len(files) {
		n = len(files)
	}
	return files[:n]
}

// CalculateETA calculates estimated time of arrival
func (asc *AdvancedStatsCollector) CalculateETA() time.Duration {
	remaining := asc.metrics.TotalBytes - asc.metrics.TransferredBytes
	if remaining <= 0 || asc.metrics.CurrentRate <= 0 {
		return 0
	}

	// Use recent average rate for more accurate ETA
	recentRate := asc.getRecentAverageRate(time.Minute * 2)
	if recentRate > 0 {
		return time.Duration(float64(remaining)/recentRate) * time.Second
	}

	return time.Duration(float64(remaining)/asc.metrics.CurrentRate) * time.Second
}

// getRecentAverageRate calculates average rate over recent period
func (asc *AdvancedStatsCollector) getRecentAverageRate(period time.Duration) float64 {
	if len(asc.metrics.RateHistory) < 2 {
		return asc.metrics.CurrentRate
	}

	cutoff := time.Now().Add(-period)
	var totalRate float64
	var count int

	for _, point := range asc.metrics.RateHistory {
		if point.Timestamp.After(cutoff) {
			totalRate += point.Rate
			count++
		}
	}

	if count > 0 {
		return totalRate / float64(count)
	}
	return asc.metrics.CurrentRate
}

// GetEfficiency calculates transfer efficiency metrics
func (asc *AdvancedStatsCollector) GetEfficiency() map[string]float64 {
	efficiency := make(map[string]float64)

	// Rate consistency (lower is better)
	if len(asc.metrics.RateHistory) > 1 {
		var variance float64
		mean := asc.metrics.AverageRate
		for _, point := range asc.metrics.RateHistory {
			diff := point.Rate - mean
			variance += diff * diff
		}
		variance /= float64(len(asc.metrics.RateHistory))
		efficiency["rate_consistency"] = math.Sqrt(variance) / mean * 100 // CV%
	}

	// Network utilization
	theoreticalMax := 100 * 1024 * 1024 // Assume 100 Mbps theoretical max
	efficiency["network_utilization"] = asc.metrics.AverageRate / float64(theoreticalMax) * 100

	// Success rate
	totalFiles := asc.metrics.CompletedFiles + asc.metrics.FailedFiles
	if totalFiles > 0 {
		efficiency["success_rate"] = float64(asc.metrics.CompletedFiles) / float64(totalFiles) * 100
	}

	// Time efficiency (actual vs theoretical minimum time)
	if asc.metrics.PeakRate > 0 {
		actualTime := time.Since(asc.metrics.StartTime).Seconds()
		theoreticalTime := float64(asc.metrics.TransferredBytes) / asc.metrics.PeakRate
		efficiency["time_efficiency"] = theoreticalTime / actualTime * 100
	}

	return efficiency
}

// RealTimeStatsPanel displays comprehensive real-time statistics
type RealTimeStatsPanel struct {
	collector   *AdvancedStatsCollector
	displayMode string // "overview", "detailed", "files", "network", "efficiency"
	refreshRate time.Duration
	lastRefresh time.Time
}

// NewRealTimeStatsPanel creates a new real-time statistics panel
func NewRealTimeStatsPanel(collector *AdvancedStatsCollector, refreshRate time.Duration) *RealTimeStatsPanel {
	return &RealTimeStatsPanel{
		collector:   collector,
		displayMode: "overview",
		refreshRate: refreshRate,
		lastRefresh: time.Now(),
	}
}

// SetDisplayMode sets the display mode
func (rtsp *RealTimeStatsPanel) SetDisplayMode(mode string) {
	rtsp.displayMode = mode
}

// ShouldRefresh returns whether the panel should refresh
func (rtsp *RealTimeStatsPanel) ShouldRefresh() bool {
	return time.Since(rtsp.lastRefresh) >= rtsp.refreshRate
}

// Render renders the statistics panel
func (rtsp *RealTimeStatsPanel) Render() string {
	if rtsp.ShouldRefresh() {
		rtsp.lastRefresh = time.Now()
	}

	switch rtsp.displayMode {
	case "detailed":
		return rtsp.renderDetailed()
	case "files":
		return rtsp.renderFiles()
	case "network":
		return rtsp.renderNetwork()
	case "efficiency":
		return rtsp.renderEfficiency()
	default:
		return rtsp.renderOverview()
	}
}

// renderOverview renders the overview statistics
func (rtsp *RealTimeStatsPanel) renderOverview() string {
	metrics := rtsp.collector.GetMetrics()
	var result strings.Builder

	result.WriteString(style.HeaderStyle.Render("📊 Transfer Statistics"))
	result.WriteString("\n\n")

	// Basic stats
	elapsed := time.Since(metrics.StartTime)
	eta := rtsp.collector.CalculateETA()

	result.WriteString(fmt.Sprintf("⏱️  Duration: %s", formatDuration(elapsed)))
	if eta > 0 {
		result.WriteString(fmt.Sprintf(" | ETA: %s", formatDuration(eta)))
	}
	result.WriteString("\n")

	result.WriteString(fmt.Sprintf("📁 Files: %d completed, %d failed of %d total\n",
		metrics.CompletedFiles, metrics.FailedFiles, metrics.TotalFiles))

	result.WriteString(fmt.Sprintf("💾 Data: %s of %s (%.1f%%)\n",
		formatBytesSimple(metrics.TransferredBytes),
		formatBytesSimple(metrics.TotalBytes),
		float64(metrics.TransferredBytes)/float64(metrics.TotalBytes)*100))

	result.WriteString(fmt.Sprintf("🚀 Speed: %s (avg: %s, peak: %s)\n",
		formatRate(metrics.CurrentRate),
		formatRate(metrics.AverageRate),
		formatRate(metrics.PeakRate)))

	// Network quality indicator
	if metrics.NetworkLatency > 0 {
		quality := "Good"
		if metrics.NetworkLatency > 100*time.Millisecond {
			quality = "Fair"
		}
		if metrics.NetworkLatency > 200*time.Millisecond {
			quality = "Poor"
		}
		result.WriteString(fmt.Sprintf("📶 Network: %s (%dms latency)\n", quality, metrics.NetworkLatency.Milliseconds()))
	}

	return result.String()
}

// renderDetailed renders detailed statistics
func (rtsp *RealTimeStatsPanel) renderDetailed() string {
	// Implementation for detailed view
	return rtsp.renderOverview() + "\n\n" + "Press 'D' for detailed view (coming soon)"
}

// renderFiles renders file-specific statistics
func (rtsp *RealTimeStatsPanel) renderFiles() string {
	// Implementation for files view
	return rtsp.renderOverview() + "\n\n" + "Press 'F' for files view (coming soon)"
}

// renderNetwork renders network statistics
func (rtsp *RealTimeStatsPanel) renderNetwork() string {
	// Implementation for network view
	return rtsp.renderOverview() + "\n\n" + "Press 'N' for network view (coming soon)"
}

// renderEfficiency renders efficiency metrics
func (rtsp *RealTimeStatsPanel) renderEfficiency() string {
	// Implementation for efficiency view
	return rtsp.renderOverview() + "\n\n" + "Press 'E' for efficiency view (coming soon)"
}

// formatBytesSimple formats bytes in a human-readable format
func formatBytesSimple(bytes int64) string {
	if bytes > 1024*1024*1024 {
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	} else if bytes > 1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	} else if bytes > 1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%d B", bytes)
}
