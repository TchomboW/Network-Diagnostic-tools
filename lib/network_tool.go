package networktool

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

// NewNetworkMonitor creates a new NetworkMonitor instance with default values.
// All fields are initialized and ready for use.
func NewNetworkMonitor() *NetworkMonitor {
	return &NetworkMonitor{
		mu:             sync.RWMutex{},
		target:         "8.8.8.8", // Default target as specified in requirements
		baselineDown:   0,          // Will be calculated from actual measurements
		baselineUp:     0,          // Will be calculated from actual measurements
		performanceMgr: performance.NewPerformanceMonitor(),
	}
}

// SetTarget updates the monitoring target with comprehensive validation.
// Thread-safe using RWMutex for concurrent read/write access.
// Returns an error if the input is invalid (empty string, malformed URL, etc.).
func (nm *NetworkMonitor) SetTarget(t string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if t == "" {
		return errors.New("target cannot be empty")
	}

	nm.target = t
	return nil
}

// GetTarget returns the current monitoring target.
// Thread-safe using RWMutex for concurrent read access.
func (nm *NetworkMonitor) GetTarget() string {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.target
}

// GetBaselineDown returns the current download speed baseline in Mbps.
// Thread-safe using RWMutex for concurrent read access.
func (nm *NetworkMonitor) GetBaselineDown() float64 {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.baselineDown
}

// GetBaselineUp returns the current upload speed baseline in Mbps.
// Thread-safe using RWMutex for concurrent read access.
func (nm *NetworkMonitor) GetBaselineUp() float64 {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.baselineUp
}

// setBaselineSpeeds updates both download and upload speed baselines.
// Thread-safe using RWMutex for concurrent write access.
// Returns an error if the input values are invalid (negative or NaN).
func (nm *NetworkMonitor) setBaselineSpeeds(down, up float64) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if down < 0 || math.IsNaN(down) {
		return errors.New("download speed cannot be negative or NaN")
	}
	if up < 0 || math.IsNaN(up) {
		return errors.New("upload speed cannot be negative or NaN")
	}

	nm.baselineDown = down
	nm.baselineUp = up
	return nil
}

// trackPingLatency records a new ping latency measurement.
// Thread-safe using RWMutex for concurrent read/write access.
// Returns an error if the latency value is invalid (negative or NaN).
func (nm *NetworkMonitor) trackPingLatency(latency time.Duration) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if latency < 0 || math.IsNaN(float64(latency)) {
		return errors.New("ping latency cannot be negative or NaN")
	}

	// Record the latency in the performance monitor
	nm.performanceMgr.RecordPing(latency)

	// Print current average ping (non-blocking)
	go func() {
		avgLatency := nm.performanceMgr.GetAveragePing()

		// Only print successful averages (non-zero values)
		if avgLatency > 0 && !math.IsNaN(avgLatency) {
			fmt.Printf("Current average ping: %.2f ms\n", avgLatency)
		} else if avgLatency == 0 {
			fmt.Println("No ping data available yet")
		}
	}()

	return nil
}

// trackSpeedTest records new speed test results.
// Thread-safe using RWMutex for concurrent read/write access.
// Returns an error if the speed values are invalid (negative or NaN).
func (nm *NetworkMonitor) trackSpeedTest(down, up float64) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if down < 0 || math.IsNaN(down) {
		return errors.New("download speed cannot be negative or NaN")
	}
	if up < 0 || math.IsNaN(up) {
		return errors.New("upload speed cannot be negative or NaN")
	}

	// Record the speeds in the performance monitor
	nm.performanceMgr.RecordSpeedTest(down)

	// Update baseline speeds
	nm.baselineDown = down
	nm.baselineUp = up

	// Print current average speeds (non-blocking)
	go func() {
		avgSpeed := nm.performanceMgr.GetAverageSpeed()

		// Only print successful averages (non-zero values)
		if avgSpeed > 0 && !math.IsNaN(avgSpeed) {
			fmt.Printf("Current average network speed: %.2f Mbps\n", avgSpeed)
		} else if avgSpeed == 0 {
			fmt.Println("No speed test data available yet")
		}
	}()

	return nil
}

// ValidateTargetFormat checks if the target follows a valid format.
// Supports IP addresses, hostnames, and URLs.
// Returns an error if the target format is invalid.
func (nm *NetworkMonitor) ValidateTargetFormat(target string) error {
	if target == "" {
		return errors.New("target cannot be empty")
	}

	// Check if it's a valid URL
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		// URL format - validate the host part
		host := strings.TrimPrefix(target, "http://")
		host = strings.TrimPrefix(host, "https://")
		host = strings.Split(host, "/")[0]
		host = strings.Split(host, ":")[0]

		if host == "" {
			return errors.New("invalid URL: empty host")
		}
		return nil
	}

	// Check if it's a valid IP address or hostname
	// Simple validation - could be enhanced with more sophisticated checks
	if strings.Contains(target, ".") || strings.Contains(target, ":") {
		// Likely an IP address or hostname
		return nil
	}

	return errors.New("invalid target format: must be an IP address, hostname, or URL")
}

// ValidateSpeedValues checks if the speed values are valid.
// Returns an error if either value is negative or NaN.
func (nm *NetworkMonitor) ValidateSpeedValues(down, up float64) error {
	if down < 0 || math.IsNaN(down) {
		return errors.New("download speed cannot be negative or NaN")
	}
	if up < 0 || math.IsNaN(up) {
		return errors.New("upload speed cannot be negative or NaN")
	}
	return nil
}

// Reset clears all monitoring data and resets to default values.
// Thread-safe using RWMutex for concurrent write access.
func (nm *NetworkMonitor) Reset() error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.target = "8.8.8.8"
	nm.baselineDown = 0
	nm.baselineUp = 0
	nm.performanceMgr.Reset()

	return nil
}
