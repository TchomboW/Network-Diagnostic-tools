package network_tool

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"network_tool/tools/performance"
)

// NetworkMonitor provides thread-safe monitoring of network diagnostic tool performance.
// All methods return errors to allow proper error handling and validation.
type NetworkMonitor struct {
	mu             sync.RWMutex                    // Mutex for thread-safe access to all fields
	target         string                          // The monitoring target (e.g., "8.8.8.8" or valid URL)
	baselineDown   float64                         // Download speed baseline in Mbps
	baselineUp     float64                         // Upload speed baseline in Mbps
	performanceMgr *performance.PerformanceMonitor // Performance tracking manager
}

// NewNetworkMonitor creates a new NetworkMonitor instance with default values.\n// All fields are initialized and ready for use.\nfunc NewNetworkMonitor() *NetworkMonitor {\n\treturn &NetworkMonitor{\n\t\tmu:             sync.RWMutex{},\n\t\ttarget:         \"8.8.8.8\", // Default target as specified in requirements\n\t\tbaselineDown:   0,          // Will be calculated from actual measurements\n\t\tbaselineUp:     0,          // Will be calculated from actual measurements\n\t\tperformanceMgr: performance.NewPerformanceMonitor(),\n\t}\n}

// SetTarget updates the monitoring target with comprehensive validation.
// Thread-safe using RWMutex for concurrent read/write access.
// Returns an error if the input is invalid (empty string, malformed URL, etc.).
func (nm *NetworkMonitor) SetTarget(t string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Validate non-empty
	t = strings.TrimSpace(t)
	if t == "" {
		return fmt.Errorf("target cannot be empty")
	}

	// Check if it's a valid URL or hostname
	validURL := isValidURL(t) || isValidHostname(t) || isValidIPAddress(t)

	if !validURL && !(t == "localhost") {
		return fmt.Errorf("invalid target format: %v (must be a valid URL, hostname, or IP address)", t)
	}

	nm.target = t
	return nil
}

// GetTarget returns the current monitoring target.
// Thread-safe using RLock for concurrent read access.
func (nm *NetworkMonitor) GetTarget() string {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.target
}

// GetBaselineDown returns the download speed baseline in Mbps.
// Thread-safe using RLock for concurrent read access.
func (nm *NetworkMonitor) GetBaselineDown() float64 {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.baselineDown
}

// GetBaselineUp returns the upload speed baseline in Mbps.
// Thread-safe using RLock for concurrent read access.
func (nm *NetworkMonitor) GetBaselineUp() float64 {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.baselineUp
}

// setBaselineSpeeds sets both download and upload baselines with validation.
// Private method with lowercase to indicate it's internal-only.
func (nm *NetworkMonitor) setBaselineSpeeds(down, up float64) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Validate non-negative values
	if down < 0 {
		return fmt.Errorf("download speed cannot be negative: %f", down)
	}

	if up < 0 {
		return fmt.Errorf("upload speed cannot be negative: %f", up)
	}

	nm.baselineDown = down
	nm.baselineUp = up
	return nil
}

// trackPingLatency records ping latency and updates performance metrics.
// Thread-safe using RWMutex for concurrent access to performanceMgr.
// Returns an error if the performance manager is not initialized or if recording fails.
func (nm *NetworkMonitor) trackPingLatency(latency time.Duration) error {
	// Validate input - very small latencies might indicate issues, but we allow them
	if latency < 0 {
		return fmt.Errorf("ping latency cannot be negative: %v", latency)
	}

	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Check if performance manager is initialized
	if nm.performanceMgr == nil {
		return errors.New("performance monitor not initialized")
	}

	// Record ping latency for performance monitoring (no return value in this version)
	nm.performanceMgr.RecordPing(latency)

	// Get the average latency over all pings (returns float64, no error)
	avgLatency := nm.performanceMgr.GetAveragePing()

	// Only print successful averages (non-zero values)
	if avgLatency > 0 && !math.IsNaN(avgLatency) {
		fmt.Printf("Current average ping: %.2f ms\n", avgLatency)
	} else if avgLatency == 0 {
		fmt.Println("No ping data available yet")
	}

	return nil
}

// trackSpeedTest records download and upload speed test results.
// Thread-safe using RWMutex for concurrent access to performanceMgr.
// Returns an error if the input is invalid or if recording fails.
func (nm *NetworkMonitor) trackSpeedTest(down, up float64) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Check if performance manager is initialized first
	if nm.performanceMgr == nil {
		return errors.New("performance monitor not initialized")
	}

	// Validate speed values before recording - must be non-negative and valid floats
	if !isValidFloat(down) || down < 0 {
		return fmt.Errorf("invalid download speed value: %v (must be a non-negative number)", down)
	}

	if !isValidFloat(up) || up < 0 {
		return fmt.Errorf("invalid upload speed value: %v (must be a non-negative number)", up)
	}

	// Validate float values are not NaN or Inf
	if math.IsNaN(down) || math.IsInf(down, 0) {
		return fmt.Errorf("download speed cannot be NaN or Infinity")
	}

	if math.IsNaN(up) || math.IsInf(up, 0) {
		return fmt.Errorf("upload speed cannot be NaN or Infinity")
	}

	// Record download and upload speeds for performance monitoring (no return value in this version)
	nm.performanceMgr.RecordSpeedTest(down)
	nm.performanceMgr.RecordSpeedTest(up)

	// Get the average speed over all tests (returns float64, no error)
	avgSpeed := nm.performanceMgr.GetAverageSpeed()

	// Only print successful averages (non-zero values)
	if avgSpeed > 0 && !math.IsNaN(avgSpeed) {
		fmt.Printf("Current average network speed: %.2f Mbps\n", avgSpeed)
	} else if avgSpeed == 0 {
		fmt.Println("No speed test data available yet")
	}

	return nil
}

// ValidateTargetFormat checks if a target string is valid before setting it.
// This allows for pre-validation without actually updating the monitor state.
func (nm *NetworkMonitor) ValidateTargetFormat(target string) error {
	target = strings.TrimSpace(target)

	if target == "" {
		return fmt.Errorf("target cannot be empty")
	}

	validURL := isValidURL(target) || isValidHostname(target) || isValidIPAddress(target)

	if !validURL && target != "localhost" {
		return fmt.Errorf("invalid target format: %v (must be a valid URL, hostname, or IP address)", target)
	}

	return nil
}

// ValidateSpeedValues checks if speed values are valid before recording them.
func (nm *NetworkMonitor) ValidateSpeedValues(down, up float64) error {
	if !isValidFloat(down) || down < 0 {
		return fmt.Errorf("invalid download speed value: %v (must be a non-negative number)", down)
	}

	if !isValidFloat(up) || up < 0 {
		return fmt.Errorf("invalid upload speed value: %v (must be a non-negative number)", up)
	}

	if math.IsNaN(down) || math.IsInf(down, 0) {
		return fmt.Errorf("download speed cannot be NaN or Infinity")
	}

	if math.IsNaN(up) || math.IsInf(up, 0) {
		return fmt.Errorf("upload speed cannot be NaN or Infinity")
	}

	return nil
}

// Reset clears all tracking data and resets the monitor to initial state.
func (nm *NetworkMonitor) Reset() error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if nm.performanceMgr != nil {
		nm.performanceMgr.Reset()
	}

	nm.target = "8.8.8.8" // Default target
	nm.baselineDown = 0   // Will be calculated from actual measurements
	nm.baselineUp = 0     // Will be calculated from actual measurements

	return nil
}
