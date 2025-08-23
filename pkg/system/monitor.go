package system

import (
	"runtime"
	"time"
)

// SystemInfo contains cross-platform system information
type SystemInfo struct {
	OS           string        `json:"os"`
	Arch         string        `json:"arch"`
	NumCPU       int           `json:"num_cpu"`
	GoVersion    string        `json:"go_version"`
	MemoryStats  MemoryStats   `json:"memory_stats"`
	RuntimeStats RuntimeStats  `json:"runtime_stats"`
	Timestamp    time.Time     `json:"timestamp"`
}

// MemoryStats contains memory usage statistics
type MemoryStats struct {
	Alloc        uint64 `json:"alloc"`         // bytes allocated and still in use
	TotalAlloc   uint64 `json:"total_alloc"`   // bytes allocated (even if freed)
	Sys          uint64 `json:"sys"`           // bytes obtained from system
	Lookups      uint64 `json:"lookups"`       // number of pointer lookups
	Mallocs      uint64 `json:"mallocs"`       // number of mallocs
	Frees        uint64 `json:"frees"`         // number of frees
	HeapAlloc    uint64 `json:"heap_alloc"`    // bytes allocated and still in use
	HeapSys      uint64 `json:"heap_sys"`      // bytes obtained from system
	HeapIdle     uint64 `json:"heap_idle"`     // bytes in idle spans
	HeapInuse    uint64 `json:"heap_inuse"`    // bytes in non-idle span
	HeapReleased uint64 `json:"heap_released"` // bytes released to the OS
	HeapObjects  uint64 `json:"heap_objects"`  // total number of allocated objects
	StackInuse   uint64 `json:"stack_inuse"`   // bytes used by stack allocator
	StackSys     uint64 `json:"stack_sys"`     // bytes obtained from system for stack allocator
	MSpanInuse   uint64 `json:"mspan_inuse"`   // bytes used by mspan structures
	MSpanSys     uint64 `json:"mspan_sys"`     // bytes obtained from system for mspan structures
	MCacheInuse  uint64 `json:"mcache_inuse"`  // bytes used by mcache structures
	MCacheSys    uint64 `json:"mcache_sys"`    // bytes obtained from system for mcache structures
	BuckHashSys  uint64 `json:"buck_hash_sys"` // bytes used by the profiling bucket hash table
	GCSys        uint64 `json:"gc_sys"`        // bytes used for garbage collection system metadata
	OtherSys     uint64 `json:"other_sys"`     // bytes used for other system allocations
}

// RuntimeStats contains Go runtime statistics
type RuntimeStats struct {
	NumGoroutine int           `json:"num_goroutine"` // number of goroutines
	NumCgoCall   int64         `json:"num_cgo_call"`  // number of cgo calls made
	GCStats      GCStats       `json:"gc_stats"`      // garbage collection statistics
}

// GCStats contains garbage collection statistics
type GCStats struct {
	NumGC        uint32        `json:"num_gc"`         // number of completed GC cycles
	NumForcedGC  uint32        `json:"num_forced_gc"`  // number of GC cycles that were forced
	GCCPUFraction float64      `json:"gc_cpu_fraction"` // fraction of CPU time used by GC
	TotalPause   time.Duration `json:"total_pause"`    // total pause time
	LastPause    time.Duration `json:"last_pause"`     // last pause time
	PauseHistory []time.Duration `json:"pause_history"` // recent pause times
}

// SystemMonitor provides cross-platform system monitoring
type SystemMonitor struct {
	startTime    time.Time
	lastGCNum    uint32
	pauseHistory []time.Duration
	maxHistory   int
}

// NewSystemMonitor creates a new system monitor
func NewSystemMonitor() *SystemMonitor {
	return &SystemMonitor{
		startTime:    time.Now(),
		pauseHistory: make([]time.Duration, 0, 10),
		maxHistory:   10,
	}
}

// GetSystemInfo returns current system information
func (sm *SystemMonitor) GetSystemInfo() SystemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Update GC pause history
	sm.updateGCHistory(&m)

	return SystemInfo{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		NumCPU:    runtime.NumCPU(),
		GoVersion: runtime.Version(),
		MemoryStats: MemoryStats{
			Alloc:        m.Alloc,
			TotalAlloc:   m.TotalAlloc,
			Sys:          m.Sys,
			Lookups:      m.Lookups,
			Mallocs:      m.Mallocs,
			Frees:        m.Frees,
			HeapAlloc:    m.HeapAlloc,
			HeapSys:      m.HeapSys,
			HeapIdle:     m.HeapIdle,
			HeapInuse:    m.HeapInuse,
			HeapReleased: m.HeapReleased,
			HeapObjects:  m.HeapObjects,
			StackInuse:   m.StackInuse,
			StackSys:     m.StackSys,
			MSpanInuse:   m.MSpanInuse,
			MSpanSys:     m.MSpanSys,
			MCacheInuse:  m.MCacheInuse,
			MCacheSys:    m.MCacheSys,
			BuckHashSys:  m.BuckHashSys,
			GCSys:        m.GCSys,
			OtherSys:     m.OtherSys,
		},
		RuntimeStats: RuntimeStats{
			NumGoroutine: runtime.NumGoroutine(),
			NumCgoCall:   runtime.NumCgoCall(),
			GCStats: GCStats{
				NumGC:         m.NumGC,
				NumForcedGC:   m.NumForcedGC,
				GCCPUFraction: m.GCCPUFraction,
				TotalPause:    time.Duration(m.PauseTotalNs),
				LastPause:     sm.getLastPause(&m),
				PauseHistory:  sm.pauseHistory,
			},
		},
		Timestamp: time.Now(),
	}
}

// updateGCHistory updates the GC pause history
func (sm *SystemMonitor) updateGCHistory(m *runtime.MemStats) {
	if m.NumGC > sm.lastGCNum {
		// New GC cycles have occurred
		for i := sm.lastGCNum; i < m.NumGC; i++ {
			pauseTime := time.Duration(m.PauseNs[(i+255)%256])
			sm.pauseHistory = append(sm.pauseHistory, pauseTime)
			
			// Keep only recent history
			if len(sm.pauseHistory) > sm.maxHistory {
				sm.pauseHistory = sm.pauseHistory[1:]
			}
		}
		sm.lastGCNum = m.NumGC
	}
}

// getLastPause returns the last GC pause time
func (sm *SystemMonitor) getLastPause(m *runtime.MemStats) time.Duration {
	if m.NumGC == 0 {
		return 0
	}
	return time.Duration(m.PauseNs[(m.NumGC+255)%256])
}

// GetMemoryUsagePercent returns memory usage as a percentage
func (sm *SystemMonitor) GetMemoryUsagePercent() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	if m.Sys == 0 {
		return 0
	}
	return float64(m.Alloc) / float64(m.Sys) * 100
}

// GetGoroutineCount returns the current number of goroutines
func (sm *SystemMonitor) GetGoroutineCount() int {
	return runtime.NumGoroutine()
}

// ForceGC forces a garbage collection
func (sm *SystemMonitor) ForceGC() {
	runtime.GC()
}

// GetUptime returns the uptime since monitor creation
func (sm *SystemMonitor) GetUptime() time.Duration {
	return time.Since(sm.startTime)
}

// ResourceUsage contains resource usage information
type ResourceUsage struct {
	MemoryPercent    float64       `json:"memory_percent"`
	GoroutineCount   int           `json:"goroutine_count"`
	GCPauseTime      time.Duration `json:"gc_pause_time"`
	GCFrequency      float64       `json:"gc_frequency"` // GCs per minute
	HeapSize         uint64        `json:"heap_size"`
	AllocRate        float64       `json:"alloc_rate"` // bytes per second
	Uptime           time.Duration `json:"uptime"`
}

// GetResourceUsage returns current resource usage
func (sm *SystemMonitor) GetResourceUsage() ResourceUsage {
	info := sm.GetSystemInfo()
	
	// Calculate GC frequency (GCs per minute)
	uptime := sm.GetUptime()
	var gcFrequency float64
	if uptime > 0 {
		gcFrequency = float64(info.RuntimeStats.GCStats.NumGC) / uptime.Minutes()
	}
	
	// Calculate allocation rate (rough estimate)
	var allocRate float64
	if uptime > 0 {
		allocRate = float64(info.MemoryStats.TotalAlloc) / uptime.Seconds()
	}
	
	return ResourceUsage{
		MemoryPercent:  sm.GetMemoryUsagePercent(),
		GoroutineCount: info.RuntimeStats.NumGoroutine,
		GCPauseTime:    info.RuntimeStats.GCStats.LastPause,
		GCFrequency:    gcFrequency,
		HeapSize:       info.MemoryStats.HeapSys,
		AllocRate:      allocRate,
		Uptime:         uptime,
	}
}

// IsResourceConstrained checks if the system is resource constrained
func (sm *SystemMonitor) IsResourceConstrained() bool {
	usage := sm.GetResourceUsage()
	
	// Consider system constrained if:
	// - Memory usage > 80%
	// - Too many goroutines (> 1000)
	// - High GC pause times (> 10ms)
	// - High GC frequency (> 10 per minute)
	
	return usage.MemoryPercent > 80.0 ||
		usage.GoroutineCount > 1000 ||
		usage.GCPauseTime > 10*time.Millisecond ||
		usage.GCFrequency > 10.0
}

// GetOptimizationRecommendations returns optimization recommendations based on current resource usage
func (sm *SystemMonitor) GetOptimizationRecommendations() []string {
	usage := sm.GetResourceUsage()
	var recommendations []string
	
	if usage.MemoryPercent > 80 {
		recommendations = append(recommendations, "High memory usage detected. Consider reducing buffer sizes or enabling compression.")
	}
	
	if usage.GoroutineCount > 1000 {
		recommendations = append(recommendations, "High goroutine count detected. Consider reducing concurrency levels.")
	}
	
	if usage.GCPauseTime > 10*time.Millisecond {
		recommendations = append(recommendations, "High GC pause times detected. Consider reducing allocation rate or heap size.")
	}
	
	if usage.GCFrequency > 10 {
		recommendations = append(recommendations, "Frequent garbage collection detected. Consider optimizing memory allocation patterns.")
	}
	
	if usage.AllocRate > 100*1024*1024 { // > 100MB/s
		recommendations = append(recommendations, "High allocation rate detected. Consider object pooling or reducing temporary allocations.")
	}
	
	return recommendations
}

// PerformanceProfile represents different performance profiles
type PerformanceProfile int

const (
	ProfileLowResource PerformanceProfile = iota
	ProfileBalanced
	ProfileHighPerformance
	ProfileMaximumThroughput
)

// String returns the string representation of PerformanceProfile
func (pp PerformanceProfile) String() string {
	switch pp {
	case ProfileLowResource:
		return "Low Resource"
	case ProfileBalanced:
		return "Balanced"
	case ProfileHighPerformance:
		return "High Performance"
	case ProfileMaximumThroughput:
		return "Maximum Throughput"
	default:
		return "Unknown"
	}
}

// GetRecommendedProfile returns the recommended performance profile based on system resources
func (sm *SystemMonitor) GetRecommendedProfile() PerformanceProfile {
	info := sm.GetSystemInfo()
	usage := sm.GetResourceUsage()
	
	// Consider system capabilities
	isLowResource := info.NumCPU <= 2 || info.MemoryStats.Sys < 2*1024*1024*1024 // < 2GB
	isConstrained := sm.IsResourceConstrained()
	
	if isLowResource || isConstrained {
		return ProfileLowResource
	} else if info.NumCPU >= 8 && info.MemoryStats.Sys >= 8*1024*1024*1024 { // >= 8 cores, >= 8GB
		if usage.MemoryPercent < 50 && usage.GoroutineCount < 500 {
			return ProfileMaximumThroughput
		}
		return ProfileHighPerformance
	}
	
	return ProfileBalanced
}

// GetProfileSettings returns settings for a given performance profile
func GetProfileSettings(profile PerformanceProfile) map[string]interface{} {
	switch profile {
	case ProfileLowResource:
		return map[string]interface{}{
			"max_goroutines":     50,
			"buffer_size":        16 * 1024,
			"chunk_size":         256 * 1024,
			"concurrent_streams": 1,
			"compression_level":  1,
			"gc_percent":         200, // Less frequent GC
		}
	case ProfileBalanced:
		return map[string]interface{}{
			"max_goroutines":     200,
			"buffer_size":        64 * 1024,
			"chunk_size":         1024 * 1024,
			"concurrent_streams": 4,
			"compression_level":  6,
			"gc_percent":         100, // Default GC
		}
	case ProfileHighPerformance:
		return map[string]interface{}{
			"max_goroutines":     500,
			"buffer_size":        128 * 1024,
			"chunk_size":         2 * 1024 * 1024,
			"concurrent_streams": 8,
			"compression_level":  3, // Lower compression for speed
			"gc_percent":         50,  // More frequent GC
		}
	case ProfileMaximumThroughput:
		return map[string]interface{}{
			"max_goroutines":     1000,
			"buffer_size":        256 * 1024,
			"chunk_size":         4 * 1024 * 1024,
			"concurrent_streams": 16,
			"compression_level":  1, // Minimal compression
			"gc_percent":         25,  // Very frequent GC
		}
	default:
		return GetProfileSettings(ProfileBalanced)
	}
}
