// +build !short

package webrtc

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// CITestConfig holds configuration for CI-specific test adjustments
type CITestConfig struct {
	IsCI                bool
	TimeoutMultiplier   float64
	MaxRetries          int
	SkipRaceTests       bool
	SkipSlowTests       bool
	ReducedConcurrency  bool
}

// GetCITestConfig returns test configuration based on the environment
func GetCITestConfig() *CITestConfig {
	config := &CITestConfig{
		IsCI:                false,
		TimeoutMultiplier:   1.0,
		MaxRetries:          1,
		SkipRaceTests:       false,
		SkipSlowTests:       false,
		ReducedConcurrency:  false,
	}

	// Detect CI environment
	if os.Getenv("CI") == "true" || 
	   os.Getenv("GITHUB_ACTIONS") == "true" ||
	   os.Getenv("CONTINUOUS_INTEGRATION") == "true" {
		config.IsCI = true
		config.TimeoutMultiplier = 2.0
		config.MaxRetries = 3
		config.ReducedConcurrency = true
		
		// Adjust based on OS
		if runtime.GOOS == "windows" {
			config.TimeoutMultiplier = 3.0
			config.SkipRaceTests = true // Race detector can be flaky on Windows CI
		}
		
		// Adjust based on available resources
		if runtime.NumCPU() <= 2 {
			config.ReducedConcurrency = true
			config.TimeoutMultiplier *= 1.5
		}
	}

	return config
}

// AdjustTimeout adjusts timeout based on CI configuration
func (c *CITestConfig) AdjustTimeout(baseTimeout time.Duration) time.Duration {
	return time.Duration(float64(baseTimeout) * c.TimeoutMultiplier)
}

// ShouldSkipTest determines if a test should be skipped based on CI config
func (c *CITestConfig) ShouldSkipTest(t *testing.T, testType string) bool {
	switch testType {
	case "race":
		if c.SkipRaceTests {
			t.Skip("Skipping race test in CI environment")
			return true
		}
	case "slow":
		if c.SkipSlowTests {
			t.Skip("Skipping slow test in CI environment")
			return true
		}
	case "network":
		// Skip network tests in CI if network is unreliable
		if c.IsCI && os.Getenv("SKIP_NETWORK_TESTS") == "true" {
			t.Skip("Skipping network test in CI environment")
			return true
		}
	}
	return false
}

// RetryOnFailure retries a test function on failure (useful for flaky tests)
func (c *CITestConfig) RetryOnFailure(t *testing.T, testName string, testFunc func() error) {
	var lastErr error
	
	for attempt := 1; attempt <= c.MaxRetries; attempt++ {
		if attempt > 1 {
			t.Logf("Retrying %s (attempt %d/%d)", testName, attempt, c.MaxRetries)
			// Add a small delay between retries
			time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
		}
		
		lastErr = testFunc()
		if lastErr == nil {
			if attempt > 1 {
				t.Logf("Test %s succeeded on attempt %d", testName, attempt)
			}
			return
		}
		
		t.Logf("Test %s failed on attempt %d: %v", testName, attempt, lastErr)
	}
	
	// All retries failed
	t.Fatalf("Test %s failed after %d attempts. Last error: %v", testName, c.MaxRetries, lastErr)
}

// SetupCIEnvironment sets up the test environment for CI
func SetupCIEnvironment(t *testing.T) *CITestConfig {
	config := GetCITestConfig()
	
	if config.IsCI {
		t.Logf("Running in CI environment with config: %+v", config)
		
		// Set environment variables for better stability
		os.Setenv("GOMAXPROCS", "2")
		
		// Reduce parallelism in CI
		if config.ReducedConcurrency {
			// This affects tests that use t.Parallel()
			// The actual parallelism is controlled by GOMAXPROCS and test flags
		}
	}
	
	return config
}

// CleanupCIEnvironment performs cleanup after CI tests
func CleanupCIEnvironment(t *testing.T, config *CITestConfig) {
	if config.IsCI {
		// Force garbage collection to free up memory
		runtime.GC()
		
		// Log memory usage
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		t.Logf("Memory usage after test: Alloc=%d KB, Sys=%d KB", 
			m.Alloc/1024, m.Sys/1024)
	}
}

// WaitWithTimeout waits for a condition with CI-adjusted timeout
func (c *CITestConfig) WaitWithTimeout(t *testing.T, condition func() bool, baseTimeout time.Duration, checkInterval time.Duration) bool {
	timeout := c.AdjustTimeout(baseTimeout)
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(checkInterval)
	}
	
	t.Logf("Condition not met within timeout %v (adjusted from %v)", timeout, baseTimeout)
	return false
}

// LogCIEnvironment logs information about the CI environment
func LogCIEnvironment(t *testing.T) {
	t.Logf("CI Environment Information:")
	t.Logf("  GOOS: %s", runtime.GOOS)
	t.Logf("  GOARCH: %s", runtime.GOARCH)
	t.Logf("  NumCPU: %d", runtime.NumCPU())
	t.Logf("  CI: %s", os.Getenv("CI"))
	t.Logf("  GITHUB_ACTIONS: %s", os.Getenv("GITHUB_ACTIONS"))
	t.Logf("  GOMAXPROCS: %s", os.Getenv("GOMAXPROCS"))
	
	// Log memory info
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("  Memory: Alloc=%d KB, Sys=%d KB", m.Alloc/1024, m.Sys/1024)
}
