package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/rescp17/lanFileSharer/internal/style"
)

// ChartPoint represents a point in a chart
type ChartPoint struct {
	X     float64
	Y     float64
	Label string
	Time  time.Time
}

// LineChart represents a simple ASCII line chart
type LineChart struct {
	title      string
	width      int
	height     int
	points     []ChartPoint
	maxPoints  int
	minY       float64
	maxY       float64
	autoScale  bool
	showGrid   bool
	showLabels bool
	yAxisLabel string
	xAxisLabel string
}

// NewLineChart creates a new line chart
func NewLineChart(title string, width, height, maxPoints int) *LineChart {
	return &LineChart{
		title:      title,
		width:      width,
		height:     height,
		maxPoints:  maxPoints,
		points:     make([]ChartPoint, 0, maxPoints),
		autoScale:  true,
		showGrid:   true,
		showLabels: true,
	}
}

// SetYAxisRange sets the Y-axis range (disables auto-scaling)
func (lc *LineChart) SetYAxisRange(min, max float64) {
	lc.minY = min
	lc.maxY = max
	lc.autoScale = false
}

// SetLabels sets the axis labels
func (lc *LineChart) SetLabels(xLabel, yLabel string) {
	lc.xAxisLabel = xLabel
	lc.yAxisLabel = yLabel
}

// AddPoint adds a new point to the chart
func (lc *LineChart) AddPoint(x, y float64, label string) {
	point := ChartPoint{
		X:     x,
		Y:     y,
		Label: label,
		Time:  time.Now(),
	}

	lc.points = append(lc.points, point)

	// Keep only the most recent points
	if len(lc.points) > lc.maxPoints {
		lc.points = lc.points[1:]
	}

	// Update auto-scaling
	if lc.autoScale {
		lc.updateScale()
	}
}

// updateScale updates the Y-axis scale based on current points
func (lc *LineChart) updateScale() {
	if len(lc.points) == 0 {
		return
	}

	lc.minY = lc.points[0].Y
	lc.maxY = lc.points[0].Y

	for _, point := range lc.points {
		if point.Y < lc.minY {
			lc.minY = point.Y
		}
		if point.Y > lc.maxY {
			lc.maxY = point.Y
		}
	}

	// Add some padding
	range_ := lc.maxY - lc.minY
	if range_ == 0 {
		range_ = 1
	}
	padding := range_ * 0.1
	lc.minY -= padding
	lc.maxY += padding
}

// Render renders the line chart as ASCII art
func (lc *LineChart) Render() string {
	if len(lc.points) == 0 {
		return lc.renderEmpty()
	}

	var result strings.Builder

	// Title
	if lc.title != "" {
		result.WriteString(style.HeaderStyle.Render(lc.title))
		result.WriteString("\n")
	}

	// Chart area
	chartLines := lc.renderChart()
	for _, line := range chartLines {
		result.WriteString(line)
		result.WriteString("\n")
	}

	// X-axis label
	if lc.xAxisLabel != "" {
		result.WriteString(fmt.Sprintf("%*s", lc.width/2+len(lc.xAxisLabel)/2, lc.xAxisLabel))
		result.WriteString("\n")
	}

	return result.String()
}

// renderEmpty renders an empty chart
func (lc *LineChart) renderEmpty() string {
	var result strings.Builder

	if lc.title != "" {
		result.WriteString(style.HeaderStyle.Render(lc.title))
		result.WriteString("\n")
	}

	// Empty chart frame
	result.WriteString("┌" + strings.Repeat("─", lc.width-2) + "┐\n")
	for i := 0; i < lc.height-2; i++ {
		result.WriteString("│" + strings.Repeat(" ", lc.width-2) + "│\n")
	}
	result.WriteString("└" + strings.Repeat("─", lc.width-2) + "┘\n")

	centerY := lc.height / 2
	centerX := lc.width / 2
	message := "No data available"

	// Overwrite center with message
	lines := strings.Split(result.String(), "\n")
	if centerY < len(lines) {
		line := lines[centerY]
		if len(line) >= centerX+len(message)/2 {
			start := centerX - len(message)/2
			end := start + len(message)
			if start >= 1 && end < len(line)-1 {
				newLine := line[:start] + message + line[end:]
				lines[centerY] = newLine
			}
		}
	}

	return strings.Join(lines, "\n")
}

// renderChart renders the actual chart
func (lc *LineChart) renderChart() []string {
	lines := make([]string, lc.height)

	// Initialize with empty frame
	for i := 0; i < lc.height; i++ {
		if i == 0 {
			lines[i] = "┌" + strings.Repeat("─", lc.width-2) + "┐"
		} else if i == lc.height-1 {
			lines[i] = "└" + strings.Repeat("─", lc.width-2) + "┘"
		} else {
			lines[i] = "│" + strings.Repeat(" ", lc.width-2) + "│"
		}
	}

	// Add Y-axis labels
	if lc.showLabels {
		lc.addYAxisLabels(lines)
	}

	// Plot points
	lc.plotPoints(lines)

	// Add grid if enabled
	if lc.showGrid {
		lc.addGrid(lines)
	}

	return lines
}

// addYAxisLabels adds Y-axis labels
func (lc *LineChart) addYAxisLabels(lines []string) {
	chartHeight := lc.height - 2 // Exclude top and bottom borders

	for i := 0; i < 5; i++ { // Show 5 Y-axis labels
		y := float64(i)/4.0*(lc.maxY-lc.minY) + lc.minY
		lineIndex := chartHeight - int(float64(i)/4.0*float64(chartHeight)) // Invert Y

		if lineIndex >= 1 && lineIndex < lc.height-1 {
			label := lc.formatYValue(y)
			// Place label on the left side
			if len(label) < lc.width-3 {
				line := lines[lineIndex]
				lines[lineIndex] = line[:1] + label + line[1+len(label):]
			}
		}
	}
}

// formatYValue formats a Y-axis value
func (lc *LineChart) formatYValue(value float64) string {
	if lc.yAxisLabel == "rate" {
		return lc.formatRateValue(value)[:6] // Truncate for space
	} else if lc.yAxisLabel == "bytes" {
		return lc.formatBytesValue(int64(value))[:6]
	}
	return fmt.Sprintf("%.1f", value)[:6]
}

// formatRateValue formats a rate value
func (lc *LineChart) formatRateValue(rate float64) string {
	if rate > 1024*1024 {
		return fmt.Sprintf("%.1fM/s", rate/(1024*1024))
	} else if rate > 1024 {
		return fmt.Sprintf("%.1fK/s", rate/1024)
	}
	return fmt.Sprintf("%.0fB/s", rate)
}

// formatBytesValue formats a bytes value
func (lc *LineChart) formatBytesValue(bytes int64) string {
	if bytes > 1024*1024*1024 {
		return fmt.Sprintf("%.1fGB", float64(bytes)/(1024*1024*1024))
	} else if bytes > 1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	} else if bytes > 1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%dB", bytes)
}

// plotPoints plots the data points
func (lc *LineChart) plotPoints(lines []string) {
	if len(lc.points) < 2 {
		return
	}

	chartWidth := lc.width - 2   // Exclude left and right borders
	chartHeight := lc.height - 2 // Exclude top and bottom borders

	for i := 0; i < len(lc.points)-1; i++ {
		p1 := lc.points[i]
		p2 := lc.points[i+1]

		// Convert to chart coordinates
		x1 := int(float64(i) / float64(len(lc.points)-1) * float64(chartWidth-1))
		x2 := int(float64(i+1) / float64(len(lc.points)-1) * float64(chartWidth-1))

		y1 := chartHeight - 1 - int((p1.Y-lc.minY)/(lc.maxY-lc.minY)*float64(chartHeight-1))
		y2 := chartHeight - 1 - int((p2.Y-lc.minY)/(lc.maxY-lc.minY)*float64(chartHeight-1))

		// Draw line between points
		lc.drawLine(lines, x1+1, y1+1, x2+1, y2+1) // +1 for border offset
	}
}

// drawLine draws a line between two points using ASCII characters
func (lc *LineChart) drawLine(lines []string, x1, y1, x2, y2 int) {
	// Simple line drawing using Bresenham-like algorithm
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)

	x, y := x1, y1
	xInc := 1
	if x1 > x2 {
		xInc = -1
	}
	yInc := 1
	if y1 > y2 {
		yInc = -1
	}

	if dx > dy {
		// More horizontal
		err := dx / 2
		for x != x2 {
			if y >= 1 && y < len(lines) && x >= 1 && x < len(lines[y])-1 {
				line := []rune(lines[y])
				line[x] = '─'
				lines[y] = string(line)
			}

			err -= dy
			if err < 0 {
				y += yInc
				err += dx
			}
			x += xInc
		}
	} else {
		// More vertical
		err := dy / 2
		for y != y2 {
			if y >= 1 && y < len(lines) && x >= 1 && x < len(lines[y])-1 {
				line := []rune(lines[y])
				line[x] = '│'
				lines[y] = string(line)
			}

			err -= dx
			if err < 0 {
				x += xInc
				err += dy
			}
			y += yInc
		}
	}

	// Mark endpoints
	if y1 >= 1 && y1 < len(lines) && x1 >= 1 && x1 < len(lines[y1])-1 {
		line := []rune(lines[y1])
		line[x1] = '●'
		lines[y1] = string(line)
	}
	if y2 >= 1 && y2 < len(lines) && x2 >= 1 && x2 < len(lines[y2])-1 {
		line := []rune(lines[y2])
		line[x2] = '●'
		lines[y2] = string(line)
	}
}

// addGrid adds a grid to the chart
func (lc *LineChart) addGrid(lines []string) {
	chartHeight := lc.height - 2

	// Add horizontal grid lines
	for i := 1; i < 4; i++ {
		lineIndex := 1 + int(float64(i)/4.0*float64(chartHeight))
		if lineIndex < len(lines) {
			line := []rune(lines[lineIndex])
			for j := 1; j < len(line)-1; j++ {
				if line[j] == ' ' {
					line[j] = '·'
				}
			}
			lines[lineIndex] = string(line)
		}
	}
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// SparkLine represents a compact sparkline chart
type SparkLine struct {
	values    []float64
	maxValues int
	width     int
}

// NewSparkLine creates a new sparkline
func NewSparkLine(width, maxValues int) *SparkLine {
	return &SparkLine{
		values:    make([]float64, 0, maxValues),
		maxValues: maxValues,
		width:     width,
	}
}

// AddValue adds a new value to the sparkline
func (sl *SparkLine) AddValue(value float64) {
	sl.values = append(sl.values, value)

	if len(sl.values) > sl.maxValues {
		sl.values = sl.values[1:]
	}
}

// Render renders the sparkline
func (sl *SparkLine) Render() string {
	if len(sl.values) == 0 {
		return strings.Repeat("_", sl.width)
	}

	// Find min and max for scaling
	min, max := sl.values[0], sl.values[0]
	for _, v := range sl.values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	if min == max {
		return strings.Repeat("─", sl.width)
	}

	// Sparkline characters (from low to high)
	chars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	var result strings.Builder

	// Sample values to fit width
	step := float64(len(sl.values)) / float64(sl.width)

	for i := 0; i < sl.width; i++ {
		index := int(float64(i) * step)
		if index >= len(sl.values) {
			index = len(sl.values) - 1
		}

		value := sl.values[index]
		normalized := (value - min) / (max - min)
		charIndex := int(normalized * float64(len(chars)-1))

		if charIndex >= len(chars) {
			charIndex = len(chars) - 1
		}
		if charIndex < 0 {
			charIndex = 0
		}

		result.WriteRune(chars[charIndex])
	}

	return result.String()
}

// HistogramChart represents a simple histogram
type HistogramChart struct {
	title   string
	buckets map[string]int
	width   int
	height  int
}

// NewHistogramChart creates a new histogram chart
func NewHistogramChart(title string, width, height int) *HistogramChart {
	return &HistogramChart{
		title:   title,
		buckets: make(map[string]int),
		width:   width,
		height:  height,
	}
}

// AddValue adds a value to the histogram
func (hc *HistogramChart) AddValue(bucket string) {
	hc.buckets[bucket]++
}

// Render renders the histogram
func (hc *HistogramChart) Render() string {
	if len(hc.buckets) == 0 {
		return hc.title + "\nNo data available"
	}

	var result strings.Builder

	if hc.title != "" {
		result.WriteString(style.HeaderStyle.Render(hc.title))
		result.WriteString("\n")
	}

	// Find max value for scaling
	maxValue := 0
	for _, count := range hc.buckets {
		if count > maxValue {
			maxValue = count
		}
	}

	// Render bars
	for bucket, count := range hc.buckets {
		barLength := int(float64(count) / float64(maxValue) * float64(hc.width-20))
		if barLength < 1 && count > 0 {
			barLength = 1
		}

		result.WriteString(fmt.Sprintf("%-15s │%s %d\n",
			bucket,
			strings.Repeat("█", barLength),
			count))
	}

	return result.String()
}
