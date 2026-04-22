package performance

import (
	"sync"
	"time"
)

// PerformanceMonitor tracks network diagnostic tool performance metrics
type PerformanceMonitor struct {
	mu             sync.RWMutex      // Mutex for thread-safe access to all fields
	pingLatencies  []time.Duration   // Track ping round-trip times
	speedTestResults []float64       // Track download/upload speeds in Mbps
	uiRenderTimes  []time.Duration   // Measure UI rendering performance in milliseconds
}

// NewPerformanceMonitor creates a new PerformanceMonitor instance
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		mu:             sync.RWMutex{},
		pingLatencies:  make([]time.Duration, 0),
		speedTestResults: make([]float64, 0),
		uiRenderTimes:  make([]time.Duration, 0),
	}
}

// RecordPing stores a ping latency measurement
func (pm *PerformanceMonitor) RecordPing(latency time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.pingLatencies = append(pm.pingLatencies, latency)
}

// GetAveragePing returns the average ping latency in milliseconds
// Returns 0 if no pings have been recorded to avoid division by zero
func (pm *PerformanceMonitor) GetAveragePing() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	if len(pm.pingLatencies) == 0 {
		return 0.0
	}
	
	var total time.Duration
	for _, latency := range pm.pingLatencies {
		total += latency
	}
	
	return float64(total.Milliseconds()) / float64(len(pm.pingLatencies))
}

// RecordSpeedTest stores a speed test result
func (pm *PerformanceMonitor) RecordSpeedTest(speed float64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.speedTestResults = append(pm.speedTestResults, speed)
}

// GetAverageSpeed returns the average download/upload speed in Mbps
// Returns 0 if no speed tests have been recorded to avoid division by zero
func (pm *PerformanceMonitor) GetAverageSpeed() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	if len(pm.speedTestResults) == 0 {
		return 0.0
	}
	
	var total float64
	for _, speed := range pm.speedTestResults {
		total += speed
	}
	
	return total / float64(len(pm.speedTestResults))
}

// RecordUIRender stores a UI render time measurement
func (pm *PerformanceMonitor) RecordUIRender(renderTime time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.uiRenderTimes = append(pm.uiRenderTimes, renderTime)
}

// GetAverageRenderTime returns the average UI rendering time in milliseconds
// Returns 0 if no UI renders have been recorded to avoid division by zero
func (pm *PerformanceMonitor) GetAverageRenderTime() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	if len(pm.uiRenderTimes) == 0 {
		return 0.0
	}
	
	var total time.Duration
	for _, renderTime := range pm.uiRenderTimes {
		total += renderTime
	}
	
	return float64(total.Milliseconds()) / float64(len(pm.uiRenderTimes))
}

// Reset clears all tracking data and resets the monitor to initial state
func (pm *PerformanceMonitor) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.pingLatencies = make([]time.Duration, 0)
	pm.speedTestResults = make([]float64, 0)
	pm.uiRenderTimes = make([]time.Duration, 0)
}
