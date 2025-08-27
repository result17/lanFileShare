package components

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/rescp17/lanFileSharer/internal/style"
)

// PerformanceMetrics contains system performance metrics
type PerformanceMetrics struct {
	CPUUsage        float64       `json:"cpu_usage"`
	MemoryUsage     uint64        `json:"memory_usage"`
	MemoryTotal     uint64        `json:"memory_total"`
	GoroutineCount  int           `json:"goroutine_count"`
	GCPauseTime     time.Duration `json:"gc_pause_time"`
	NetworkLatency  time.Duration `json:"network_latency"`
	DiskIORead      uint64        `json:"disk_io_read"`
	DiskIOWrite     uint64        `json:"disk_io_write"`
	TransferRate    float64       `json:"transfer_rate"`
	ConnectionCount int           `json:"connection_count"`
	ErrorRate       float64       `json:"error_rate"`
	Timestamp       time.Time     `json:"timestamp"`
}

// PerformanceOptimizer manages performance optimization
type PerformanceOptimizer struct {
	mu                sync.RWMutex
	metrics           []PerformanceMetrics
	maxMetrics        int
	optimizationLevel OptimizationLevel
	autoOptimize      bool

	// Optimization settings
	maxGoroutines     int
	bufferSize        int
	compressionLevel  int
	chunkSize         int64
	concurrentStreams int

	// Thresholds for auto-optimization
	cpuThreshold     float64
	memoryThreshold  float64
	latencyThreshold time.Duration

	// Performance history
	avgCPU           float64
	avgMemory        float64
	avgLatency       time.Duration
	peakTransferRate float64

	// Optimization recommendations
	recommendations []OptimizationRecommendation
}

// OptimizationLevel represents different levels of optimization
type OptimizationLevel int

const (
	OptimizationConservative OptimizationLevel = iota
	OptimizationBalanced
	OptimizationAggressive
	OptimizationMaximum
)

// String returns the string representation of OptimizationLevel
func (ol OptimizationLevel) String() string {
	switch ol {
	case OptimizationConservative:
		return "Conservative"
	case OptimizationBalanced:
		return "Balanced"
	case OptimizationAggressive:
		return "Aggressive"
	case OptimizationMaximum:
		return "Maximum"
	default:
		return "Unknown"
	}
}

// OptimizationRecommendation represents a performance optimization recommendation
type OptimizationRecommendation struct {
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Impact      string    `json:"impact"` // "low", "medium", "high"
	Action      string    `json:"action"`
	Applied     bool      `json:"applied"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer() *PerformanceOptimizer {
	return &PerformanceOptimizer{
		metrics:           make([]PerformanceMetrics, 0),
		maxMetrics:        100,
		optimizationLevel: OptimizationBalanced,
		autoOptimize:      true,
		maxGoroutines:     1000,
		bufferSize:        64 * 1024,
		compressionLevel:  6,
		chunkSize:         1024 * 1024,
		concurrentStreams: 4,
		cpuThreshold:      80.0,
		memoryThreshold:   80.0,
		latencyThreshold:  200 * time.Millisecond,
		recommendations:   make([]OptimizationRecommendation, 0),
	}
}

// CollectMetrics collects current performance metrics
func (po *PerformanceOptimizer) CollectMetrics() PerformanceMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := PerformanceMetrics{
		MemoryUsage:    m.Alloc,
		MemoryTotal:    m.Sys,
		GoroutineCount: runtime.NumGoroutine(),
		GCPauseTime:    time.Duration(m.PauseNs[(m.NumGC+255)%256]),
		Timestamp:      time.Now(),
	}

	po.mu.Lock()
	defer po.mu.Unlock()

	// Add to metrics history
	po.metrics = append(po.metrics, metrics)
	if len(po.metrics) > po.maxMetrics {
		po.metrics = po.metrics[1:]
	}

	// Update averages
	po.updateAverages()

	// Generate recommendations if auto-optimize is enabled
	if po.autoOptimize {
		po.generateRecommendations(metrics)
	}

	return metrics
}

// updateAverages updates running averages
func (po *PerformanceOptimizer) updateAverages() {
	if len(po.metrics) == 0 {
		return
	}

	var totalCPU, totalMemory float64
	var totalLatency time.Duration
	var maxTransferRate float64

	for _, m := range po.metrics {
		totalCPU += m.CPUUsage
		totalMemory += float64(m.MemoryUsage)
		totalLatency += m.NetworkLatency
		if m.TransferRate > maxTransferRate {
			maxTransferRate = m.TransferRate
		}
	}

	count := float64(len(po.metrics))
	po.avgCPU = totalCPU / count
	po.avgMemory = totalMemory / count
	po.avgLatency = time.Duration(int64(totalLatency) / int64(count))
	po.peakTransferRate = maxTransferRate
}

// generateRecommendations generates optimization recommendations based on metrics
func (po *PerformanceOptimizer) generateRecommendations(metrics PerformanceMetrics) {
	now := time.Now()

	// High memory usage recommendation
	memoryPercent := float64(metrics.MemoryUsage) / float64(metrics.MemoryTotal) * 100
	if memoryPercent > po.memoryThreshold {
		po.addRecommendation(OptimizationRecommendation{
			Type:        "memory",
			Title:       "High Memory Usage Detected",
			Description: fmt.Sprintf("Memory usage is at %.1f%%, consider reducing buffer sizes", memoryPercent),
			Impact:      "medium",
			Action:      "reduce_buffer_size",
			Timestamp:   now,
		})
	}

	// High goroutine count recommendation
	if metrics.GoroutineCount > po.maxGoroutines {
		po.addRecommendation(OptimizationRecommendation{
			Type:        "goroutines",
			Title:       "High Goroutine Count",
			Description: fmt.Sprintf("Running %d goroutines, consider reducing concurrency", metrics.GoroutineCount),
			Impact:      "high",
			Action:      "reduce_concurrency",
			Timestamp:   now,
		})
	}

	// High network latency recommendation
	if metrics.NetworkLatency > po.latencyThreshold {
		po.addRecommendation(OptimizationRecommendation{
			Type:        "network",
			Title:       "High Network Latency",
			Description: fmt.Sprintf("Network latency is %v, consider adjusting chunk size", metrics.NetworkLatency),
			Impact:      "medium",
			Action:      "adjust_chunk_size",
			Timestamp:   now,
		})
	}

	// Low transfer rate recommendation
	if po.peakTransferRate > 0 && metrics.TransferRate < po.peakTransferRate*0.5 {
		po.addRecommendation(OptimizationRecommendation{
			Type:        "transfer",
			Title:       "Low Transfer Rate",
			Description: "Transfer rate is significantly below peak, consider optimization",
			Impact:      "high",
			Action:      "optimize_transfer",
			Timestamp:   now,
		})
	}
}

// addRecommendation adds a new recommendation if it doesn't already exist
func (po *PerformanceOptimizer) addRecommendation(rec OptimizationRecommendation) {
	// Check if similar recommendation already exists
	for _, existing := range po.recommendations {
		if existing.Type == rec.Type && existing.Action == rec.Action && !existing.Applied {
			return // Don't add duplicate
		}
	}

	po.recommendations = append(po.recommendations, rec)

	// Keep only recent recommendations
	if len(po.recommendations) > 20 {
		po.recommendations = po.recommendations[1:]
	}
}

// ApplyOptimization applies an optimization recommendation
func (po *PerformanceOptimizer) ApplyOptimization(index int) error {
	po.mu.Lock()
	defer po.mu.Unlock()

	if index < 0 || index >= len(po.recommendations) {
		return fmt.Errorf("invalid recommendation index")
	}

	rec := &po.recommendations[index]
	if rec.Applied {
		return fmt.Errorf("recommendation already applied")
	}

	// Apply the optimization based on action
	switch rec.Action {
	case "reduce_buffer_size":
		po.bufferSize = po.bufferSize / 2
		if po.bufferSize < 4096 {
			po.bufferSize = 4096
		}
	case "reduce_concurrency":
		po.concurrentStreams = po.concurrentStreams / 2
		if po.concurrentStreams < 1 {
			po.concurrentStreams = 1
		}
	case "adjust_chunk_size":
		if po.avgLatency > 100*time.Millisecond {
			po.chunkSize = po.chunkSize * 2 // Larger chunks for high latency
		} else {
			po.chunkSize = po.chunkSize / 2 // Smaller chunks for low latency
		}
		if po.chunkSize < 64*1024 {
			po.chunkSize = 64 * 1024
		}
		if po.chunkSize > 10*1024*1024 {
			po.chunkSize = 10 * 1024 * 1024
		}
	case "optimize_transfer":
		po.optimizationLevel = OptimizationAggressive
	}

	rec.Applied = true
	return nil
}

// SetOptimizationLevel sets the optimization level
func (po *PerformanceOptimizer) SetOptimizationLevel(level OptimizationLevel) {
	po.mu.Lock()
	defer po.mu.Unlock()

	po.optimizationLevel = level

	// Adjust settings based on level
	switch level {
	case OptimizationConservative:
		po.maxGoroutines = 100
		po.bufferSize = 32 * 1024
		po.concurrentStreams = 2
		po.compressionLevel = 3
	case OptimizationBalanced:
		po.maxGoroutines = 500
		po.bufferSize = 64 * 1024
		po.concurrentStreams = 4
		po.compressionLevel = 6
	case OptimizationAggressive:
		po.maxGoroutines = 1000
		po.bufferSize = 128 * 1024
		po.concurrentStreams = 8
		po.compressionLevel = 9
	case OptimizationMaximum:
		po.maxGoroutines = 2000
		po.bufferSize = 256 * 1024
		po.concurrentStreams = 16
		po.compressionLevel = 9
	}
}

// GetCurrentSettings returns current optimization settings
func (po *PerformanceOptimizer) GetCurrentSettings() map[string]interface{} {
	po.mu.RLock()
	defer po.mu.RUnlock()

	return map[string]interface{}{
		"optimization_level": po.optimizationLevel.String(),
		"max_goroutines":     po.maxGoroutines,
		"buffer_size":        po.bufferSize,
		"compression_level":  po.compressionLevel,
		"chunk_size":         po.chunkSize,
		"concurrent_streams": po.concurrentStreams,
		"auto_optimize":      po.autoOptimize,
	}
}

// GetRecommendations returns current optimization recommendations
func (po *PerformanceOptimizer) GetRecommendations() []OptimizationRecommendation {
	po.mu.RLock()
	defer po.mu.RUnlock()

	// Return only unapplied recommendations
	var active []OptimizationRecommendation
	for _, rec := range po.recommendations {
		if !rec.Applied {
			active = append(active, rec)
		}
	}
	return active
}

// GetMetricsHistory returns performance metrics history
func (po *PerformanceOptimizer) GetMetricsHistory() []PerformanceMetrics {
	po.mu.RLock()
	defer po.mu.RUnlock()

	result := make([]PerformanceMetrics, len(po.metrics))
	copy(result, po.metrics)
	return result
}

// PerformancePanel displays performance metrics and optimization controls
type PerformancePanel struct {
	optimizer   *PerformanceOptimizer
	visible     bool
	selectedTab int
	tabs        []string
	selectedRec int
	refreshRate time.Duration
	lastRefresh time.Time
}

// NewPerformancePanel creates a new performance panel
func NewPerformancePanel(optimizer *PerformanceOptimizer) *PerformancePanel {
	return &PerformancePanel{
		optimizer:   optimizer,
		visible:     false,
		selectedTab: 0,
		tabs:        []string{"Metrics", "Recommendations", "Settings"},
		refreshRate: 2 * time.Second,
	}
}

// Show shows the performance panel
func (pp *PerformancePanel) Show() {
	pp.visible = true
}

// Hide hides the performance panel
func (pp *PerformancePanel) Hide() {
	pp.visible = false
}

// IsVisible returns whether the panel is visible
func (pp *PerformancePanel) IsVisible() bool {
	return pp.visible
}

// Navigate handles navigation within the performance panel
func (pp *PerformancePanel) Navigate(action KeyAction) bool {
	if !pp.visible {
		return false
	}

	switch action {
	case KeyActionNavigateLeft:
		if pp.selectedTab > 0 {
			pp.selectedTab--
			return true
		}
	case KeyActionNavigateRight:
		if pp.selectedTab < len(pp.tabs)-1 {
			pp.selectedTab++
			return true
		}
	case KeyActionNavigateUp:
		if pp.selectedTab == 1 && pp.selectedRec > 0 { // Recommendations tab
			pp.selectedRec--
			return true
		}
	case KeyActionNavigateDown:
		if pp.selectedTab == 1 { // Recommendations tab
			recommendations := pp.optimizer.GetRecommendations()
			if pp.selectedRec < len(recommendations)-1 {
				pp.selectedRec++
				return true
			}
		}
	case KeyActionSelect:
		if pp.selectedTab == 1 { // Apply recommendation
			pp.optimizer.ApplyOptimization(pp.selectedRec)
			return true
		}
	case KeyActionCancel:
		pp.Hide()
		return true
	}

	return false
}

// ShouldRefresh returns whether the panel should refresh
func (pp *PerformancePanel) ShouldRefresh() bool {
	return time.Since(pp.lastRefresh) >= pp.refreshRate
}

// Render renders the performance panel
func (pp *PerformancePanel) Render() string {
	if !pp.visible {
		return ""
	}

	if pp.ShouldRefresh() {
		pp.optimizer.CollectMetrics()
		pp.lastRefresh = time.Now()
	}

	var result strings.Builder

	// Title
	result.WriteString(style.HeaderStyle.Render("⚡ Performance Monitor"))
	result.WriteString("\n\n")

	// Tab bar
	result.WriteString(pp.renderTabBar())
	result.WriteString("\n\n")

	// Tab content
	switch pp.selectedTab {
	case 0:
		result.WriteString(pp.renderMetricsTab())
	case 1:
		result.WriteString(pp.renderRecommendationsTab())
	case 2:
		result.WriteString(pp.renderSettingsTab())
	}

	// Instructions
	result.WriteString("\n\n")
	result.WriteString(style.FileStyle.Render("←/→ Switch tabs | ↑/↓ Navigate | Enter Apply | Esc Close"))

	// Wrap in border
	content := result.String()
	bordered := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1).
		Render(content)

	return bordered
}

// renderTabBar renders the tab bar
func (pp *PerformancePanel) renderTabBar() string {
	var tabs []string
	for i, tab := range pp.tabs {
		if i == pp.selectedTab {
			tabs = append(tabs, style.HighlightFontStyle.Render(fmt.Sprintf("[%s]", tab)))
		} else {
			tabs = append(tabs, style.FileStyle.Render(tab))
		}
	}
	return strings.Join(tabs, "  ")
}

// renderMetricsTab renders the metrics tab
func (pp *PerformancePanel) renderMetricsTab() string {
	var result strings.Builder

	metrics := pp.optimizer.GetMetricsHistory()
	if len(metrics) == 0 {
		result.WriteString("No metrics available")
		return result.String()
	}

	latest := metrics[len(metrics)-1]

	result.WriteString("📊 Current Performance Metrics\n\n")

	// Memory usage
	memPercent := float64(latest.MemoryUsage) / float64(latest.MemoryTotal) * 100
	result.WriteString(fmt.Sprintf("Memory: %s / %s (%.1f%%)\n",
		formatBytesSimple(int64(latest.MemoryUsage)),
		formatBytesSimple(int64(latest.MemoryTotal)),
		memPercent))

	// Goroutines
	result.WriteString(fmt.Sprintf("Goroutines: %d\n", latest.GoroutineCount))

	// GC pause time
	if latest.GCPauseTime > 0 {
		result.WriteString(fmt.Sprintf("GC Pause: %v\n", latest.GCPauseTime))
	}

	// Network metrics
	if latest.NetworkLatency > 0 {
		result.WriteString(fmt.Sprintf("Network Latency: %v\n", latest.NetworkLatency))
	}

	if latest.TransferRate > 0 {
		result.WriteString(fmt.Sprintf("Transfer Rate: %s\n", formatRateSimple(latest.TransferRate)))
	}

	// Connection count
	if latest.ConnectionCount > 0 {
		result.WriteString(fmt.Sprintf("Connections: %d\n", latest.ConnectionCount))
	}

	return result.String()
}

// renderRecommendationsTab renders the recommendations tab
func (pp *PerformancePanel) renderRecommendationsTab() string {
	var result strings.Builder

	recommendations := pp.optimizer.GetRecommendations()
	result.WriteString("💡 Optimization Recommendations\n\n")

	if len(recommendations) == 0 {
		result.WriteString("No recommendations available")
		return result.String()
	}

	for i, rec := range recommendations {
		prefix := "  "
		if i == pp.selectedRec {
			prefix = "▶ "
		}

		impactIcon := pp.getImpactIcon(rec.Impact)
		result.WriteString(fmt.Sprintf("%s%s %s\n", prefix, impactIcon, rec.Title))

		if i == pp.selectedRec {
			result.WriteString(fmt.Sprintf("    %s\n", rec.Description))
			result.WriteString(fmt.Sprintf("    Impact: %s\n", strings.ToUpper(rec.Impact)))
		}
	}

	return result.String()
}

// renderSettingsTab renders the settings tab
func (pp *PerformancePanel) renderSettingsTab() string {
	var result strings.Builder

	settings := pp.optimizer.GetCurrentSettings()
	result.WriteString("⚙️ Performance Settings\n\n")

	result.WriteString(fmt.Sprintf("Optimization Level: %s\n", settings["optimization_level"]))
	result.WriteString(fmt.Sprintf("Max Goroutines: %d\n", settings["max_goroutines"]))
	result.WriteString(fmt.Sprintf("Buffer Size: %s\n", formatBytesSimple(int64(settings["buffer_size"].(int)))))
	result.WriteString(fmt.Sprintf("Chunk Size: %s\n", formatBytesSimple(settings["chunk_size"].(int64))))
	result.WriteString(fmt.Sprintf("Concurrent Streams: %d\n", settings["concurrent_streams"]))
	result.WriteString(fmt.Sprintf("Compression Level: %d\n", settings["compression_level"]))
	result.WriteString(fmt.Sprintf("Auto Optimize: %v\n", settings["auto_optimize"]))

	return result.String()
}

// getImpactIcon returns an icon for the impact level
func (pp *PerformancePanel) getImpactIcon(impact string) string {
	switch impact {
	case "high":
		return "🔴"
	case "medium":
		return "🟡"
	case "low":
		return "🟢"
	default:
		return "⚪"
	}
}

// formatRateSimple formats transfer rate in a human-readable format
func formatRateSimple(rate float64) string {
	if rate > 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", rate/(1024*1024))
	} else if rate > 1024 {
		return fmt.Sprintf("%.1f KB/s", rate/1024)
	}
	return fmt.Sprintf("%.0f B/s", rate)
}
