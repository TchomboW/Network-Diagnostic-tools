package performance

import (
	"sync"
	"testing"
	"time"
)

// Mock PerformanceMonitor for testing - same structure as the real one we'll implement
type MockPerformanceMonitor struct {
	mu             sync.RWMutex
	pingLatencies  []time.Duration
	speedTestResults []float64
	uiRenderTimes  []time.Duration
}

func NewMockPerformanceMonitor() *MockPerformanceMonitor {
	return &MockPerformanceMonitor{
		mu:             sync.RWMutex{},
		pingLatencies:  make([]time.Duration, 0),
		speedTestResults: make([]float64, 0),
		uiRenderTimes:  make([]time.Duration, 0),
	}
}

func (pm *MockPerformanceMonitor) RecordPing(latency time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.pingLatencies = append(pm.pingLatencies, latency)
}

func (pm *MockPerformanceMonitor) GetAveragePing() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if len(pm.pingLatencies) == 0 { return 0.0 }
	var total time.Duration
	for _, latency := range pm.pingLatencies { total += latency }
	return float64(total.Milliseconds()) / float64(len(pm.pingLatencies))
}

func (pm *MockPerformanceMonitor) RecordSpeedTest(speed float64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.speedTestResults = append(pm.speedTestResults, speed)
}

func (pm *MockPerformanceMonitor) GetAverageSpeed() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if len(pm.speedTestResults) == 0 { return 0.0 }
	var total float64
	for _, speed := range pm.speedTestResults { total += speed }
	return total / float64(len(pm.speedTestResults))
}

func (pm *MockPerformanceMonitor) RecordUIRender(renderTime time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.uiRenderTimes = append(pm.uiRenderTimes, renderTime)
}

func (pm *MockPerformanceMonitor) GetAverageRenderTime() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if len(pm.uiRenderTimes) == 0 { return 0.0 }
	var total time.Duration
	for _, renderTime := range pm.uiRenderTimes { total += renderTime }
	return float64(total.Milliseconds()) / float64(len(pm.uiRenderTimes))
}

func (pm *MockPerformanceMonitor) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.pingLatencies = make([]time.Duration, 0)
	pm.speedTestResults = make([]float64, 0)
	pm.uiRenderTimes = make([]time.Duration, 0)
}

// TestPerformanceMonitorCreation tests that the PerformanceMonitor can be created successfully
func TestPerformanceMonitorCreation(t *testing.T) {
	pm := NewMockPerformanceMonitor()
	if pm == nil {
		t.Fatal("Expected PerformanceMonitor to not be nil")
	}
	
	// Verify all fields are initialized
	if !pm.mu.TryLock() {
		t.Fatal("Failed to acquire initial lock, mutex may not be properly initialized")
	}
	pm.mu.Unlock()
}

// TestRecordPing tests that RecordPing stores latency correctly
func TestRecordPing(t *testing.T) {
	pm := NewMockPerformanceMonitor()
	testLatency := 50 * time.Millisecond
	
	pm.RecordPing(testLatency)
	
	if len(pm.pingLatencies) != 1 {
		t.Fatalf("Expected 1 ping latency, got %d", len(pm.pingLatencies))
	}
	
	if pm.pingLatencies[0] != testLatency {
		t.Errorf("Expected latency to be %v, got %v", testLatency, pm.pingLatencies[0])
	}
}

// TestGetAveragePing tests that GetAveragePing returns correct average
func TestGetAveragePing(t *testing.T) {
	pm := NewMockPerformanceMonitor()
	
	// Test with empty slice - should return 0, not panic
	result := pm.GetAveragePing()
	if result != 0.0 {
		t.Errorf("Expected average ping of 0 for empty slice, got %f", result)
	}
	
	// Add some latencies and test calculation
	testCases := []struct {
		name     string
		latencies []time.Duration
		expected float64
	}{
		{"Single latency", []time.Duration{50 * time.Millisecond}, 50.0},
		{"Multiple equal latencies", []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}, 10.0},
		{"Mixed latencies", []time.Duration{20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond}, 30.0},
	}
	
	for _, tc := range testCases {
		pm = NewMockPerformanceMonitor()
		for _, latency := range tc.latencies {
			pm.RecordPing(latency)
		}
		
		result := pm.GetAveragePing()
		if result != tc.expected {
			t.Errorf("%s: Expected average ping of %f, got %f", tc.name, tc.expected, result)
		}
	}
}

// TestRecordSpeedTest tests that RecordSpeedTest stores speed results correctly
func TestRecordSpeedTest(t *testing.T) {
	pm := NewMockPerformanceMonitor()
	testSpeed := 50.5 // Mbps
	
	pm.RecordSpeedTest(testSpeed)
	
	if len(pm.speedTestResults) != 1 {
		t.Fatalf("Expected 1 speed test result, got %d", len(pm.speedTestResults))
	}
	
	if pm.speedTestResults[0] != testSpeed {
		t.Errorf("Expected speed to be %f, got %f", testSpeed, pm.speedTestResults[0])
	}
}

// TestGetAverageSpeed tests that GetAverageSpeed returns correct average
func TestGetAverageSpeed(t *testing.T) {
	pm := NewMockPerformanceMonitor()
	
	// Test with empty slice - should return 0, not panic
	result := pm.GetAverageSpeed()
	if result != 0.0 {
		t.Errorf("Expected average speed of 0 for empty slice, got %f", result)
	}
	
	// Add some speeds and test calculation
	testCases := []struct {
		name     string
		speeds   []float64
		expected float64
	}{
		{"Single speed", []float64{10.5}, 10.5},
		{"Multiple equal speeds", []float64{20.0, 20.0, 20.0}, 20.0},
		{"Mixed speeds", []float64{15.5, 25.5, 35.5}, 25.5}, // (15.5 + 25.5 + 35.5) / 3 = 76.5/3 = 25.5
	}
	
	for _, tc := range testCases {
		pm = NewMockPerformanceMonitor()
		for _, speed := range tc.speeds {
			pm.RecordSpeedTest(speed)
		}
		
		result := pm.GetAverageSpeed()
		if result != tc.expected {
			t.Errorf("%s: Expected average speed of %f, got %f", tc.name, tc.expected, result)
		}
	}
}

// TestRecordUIRender tests that RecordUIRender stores render times correctly
func TestRecordUIRender(t *testing.T) {
	pm := NewMockPerformanceMonitor()
	testRenderTime := 100 * time.Millisecond
	
	pm.RecordUIRender(testRenderTime)
	
	if len(pm.uiRenderTimes) != 1 {
		t.Fatalf("Expected 1 UI render time, got %d", len(pm.uiRenderTimes))
	}
	
	if pm.uiRenderTimes[0] != testRenderTime {
		t.Errorf("Expected render time to be %v, got %v", testRenderTime, pm.uiRenderTimes[0])
	}
}

// TestGetAverageRenderTime tests that GetAverageRenderTime returns correct average
func TestGetAverageRenderTime(t *testing.T) {
	pm := NewMockPerformanceMonitor()
	
	// Test with empty slice - should return 0, not panic
	result := pm.GetAverageRenderTime()
	if result != 0.0 {
		t.Errorf("Expected average render time of 0 for empty slice, got %f", result)
	}
	
	// Add some render times and test calculation
	testCases := []struct {
		name     string
		renderTimes []time.Duration
		expected float64
	}{
		{"Single render time", []time.Duration{50 * time.Millisecond}, 50.0},
		{"Multiple equal render times", []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}, 10.0},
		{"Mixed render times", []time.Duration{20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond}, 30.0},
	}
	
	for _, tc := range testCases {
		pm = NewMockPerformanceMonitor()
		for _, renderTime := range tc.renderTimes {
			pm.RecordUIRender(renderTime)
		}
		
		result := pm.GetAverageRenderTime()
		if result != tc.expected {
			t.Errorf("%s: Expected average render time of %f, got %f", tc.name, tc.expected, result)
		}
	}
}

// TestReset tests that Reset clears all tracking data
func TestReset(t *testing.T) {
	pm := NewMockPerformanceMonitor()
	
	// Add some data first
	testLatency := 50 * time.Millisecond
	testSpeed := 25.0 // Mbps
	testRenderTime := 100 * time.Millisecond
	
	pm.RecordPing(testLatency)
	pm.RecordSpeedTest(testSpeed)
	pm.RecordUIRender(testRenderTime)
	
	// Verify data exists before reset
	if len(pm.pingLatencies) != 1 || len(pm.speedTestResults) != 1 || len(pm.uiRenderTimes) != 1 {
		t.Fatal("Expected all tracking data to exist before reset")
	}
	
	pm.Reset()
	
	// Verify all data is cleared after reset
	if len(pm.pingLatencies) != 0 {
		t.Errorf("Expected ping latencies slice to be empty after reset, got length %d", len(pm.pingLatencies))
	}
	if len(pm.speedTestResults) != 0 {
		t.Errorf("Expected speed test results slice to be empty after reset, got length %d", len(pm.speedTestResults))
	}
	if len(pm.uiRenderTimes) != 0 {
		t.Errorf("Expected UI render times slice to be empty after reset, got length %d", len(pm.uiRenderTimes))
	}
	
	// Verify averages return 0 after reset
	if pm.GetAveragePing() != 0.0 || pm.GetAverageSpeed() != 0.0 || pm.GetAverageRenderTime() != 0.0 {
		t.Error("Expected all averages to be 0 after reset")
	}
}

// TestConcurrentAccess tests thread-safe access using sync.RWMutex for concurrent operations
func TestConcurrentAccess(t *testing.T) {
	pm := NewMockPerformanceMonitor()
	
	var wg sync.WaitGroup
	numGoroutines := 10
	
	// Concurrent writes to ping latencies
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			testLatency := time.Duration(10+idx) * time.Millisecond
			pm.RecordPing(testLatency)
		}(i)
	}
	
	wg.Wait()
	
	// Concurrent reads of averages
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_ = pm.GetAveragePing() // Should not panic
		}(i)
	}
	
	wg.Wait()
	
	// Verify no data loss occurred
	if len(pm.pingLatencies) != numGoroutines {
		t.Errorf("Expected %d ping latencies after concurrent writes, got %d", numGoroutines, len(pm.pingLatencies))
	}
}
