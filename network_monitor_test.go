package network_tool

import (
	"sync"
	"testing"
	"time"

	"network_tool/tools/performance"
)

// TestNewNetworkMonitorCreation tests that NetworkMonitor can be created successfully
func TestNewNetworkMonitorCreation(t *testing.T) {
	nm := NewNetworkMonitor()
	if nm == nil {
		t.Fatal("Expected NetworkMonitor to not be nil")
	}

	// Verify all fields are initialized correctly
	if nm.target != "8.8.8.8" {
		t.Errorf("Expected default target to be '8.8.8.8', got '%s'", nm.target)
	}

	if nm.baselineDown != 0 {
		t.Errorf("Expected baseline down to be 0, got %f", nm.baselineDown)
	}

	if nm.baselineUp != 0 {
		t.Errorf("Expected baseline up to be 0, got %f", nm.baselineUp)
	}

	// Verify mutex is properly initialized
	if !nm.mu.TryLock() {
		t.Fatal("Failed to acquire initial lock, mutex may not be properly initialized")
	}
	nm.mu.Unlock()

	// Verify PerformanceMonitor is created and accessible
	if nm.performanceMgr == nil {
		t.Error("Expected performance manager to not be nil")
	}

	// Test that we can get the average ping from an empty monitor (should return 0)
	ping := nm.performanceMgr.GetAveragePing()
	if ping != 0.0 {
		t.Errorf("Expected average ping of 0 for empty monitor, got %f", ping)
	}

	// Test that we can get the average speed from an empty monitor (should return 0)
	speed := nm.performanceMgr.GetAverageSpeed()
	if speed != 0.0 {
		t.Errorf("Expected average speed of 0 for empty monitor, got %f", speed)
	}
}

// TestGetterMethods tests that getter methods work correctly
func TestGetterMethods(t *testing.T) {
	nm := NewNetworkMonitor()

	// Test GetTarget returns default value
	target := nm.GetTarget()
	if target != "8.8.8.8" {
		t.Errorf("Expected GetTarget to return '8.8.8.8', got '%s'", target)
	}

	// Test GetBaselineDown and GetBaselineUp initially return 0
	down := nm.GetBaselineDown()
	if down != 0.0 {
		t.Errorf("Expected baseline down to be 0, got %f", down)
	}

	up := nm.GetBaselineUp()
	if up != 0.0 {
		t.Errorf("Expected baseline up to be 0, got %f", up)
	}
}

// TestSetTarget tests that SetTarget updates the monitoring target correctly
func TestSetTarget(t *testing.T) {
	nm := NewNetworkMonitor()

	// Test setting a new target
	testTargets := []string{
		"google.com",
		"1.1.1.1",
		"localhost",
	}

	for _, testTarget := range testTargets {
		nm.SetTarget(testTarget)
		result := nm.GetTarget()
		if result != testTarget {
			t.Errorf("Expected target to be '%s', got '%s'", testTarget, result)
		}
	}

	// Test thread-safe setting with concurrent access
	var wg sync.WaitGroup
	numGoroutines := 10
	targetsToSet := []string{"targetA", "targetB", "targetC", "targetD", "targetE", "targetF", "targetG", "targetH", "targetI", "targetJ"}

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			nm.SetTarget(targetsToSet[idx])
		}(i % len(targetsToSet)) // Use modulo to cycle through targets
	}

	wg.Wait()

	// Concurrent reads of target and baselines
	wg.Add(numGoroutines * 3) // 3 getters per goroutine
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_ = nm.GetTarget()   // Should not panic
			_ = nm.GetBaselineDown()
			_ = nm.GetBaselineUp()
		}(i)
	}

	wg.Wait()

	// Verify all operations completed without deadlock or race conditions
	// Just checking we got here means thread safety works
	t.Log("Concurrent access test passed - no deadlocks or panics")
}

// TestSetBaselineSpeeds tests that setBaselineSpeeds sets both baselines correctly
func TestSetBaselineSpeeds(t *testing.T) {
	nm := NewNetworkMonitor()

	// Test setting baseline speeds
	testCases := []struct {
		down float64
		up   float64
	}{
		{10.5, 20.3},
		{0, 0},
		{100, 50},
	}

	for _, tc := range testCases {
		nm.setBaselineSpeeds(tc.down, tc.up)

		resultDown := nm.GetBaselineDown()
		if resultDown != tc.down {
			t.Errorf("Expected baseline down to be %f, got %f", tc.down, resultDown)
		}

		resultUp := nm.GetBaselineUp()
		if resultUp != tc.up {
			t.Errorf("Expected baseline up to be %f, got %f", tc.up, resultUp)
		}
	}
}

// TestConcurrentAccess tests thread-safe access using sync.RWMutex for concurrent operations
func TestConcurrentAccess(t *testing.T) {
	nm := NewNetworkMonitor()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent writes to target
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			nm.SetTarget("target" + string(rune('A'+idx)))
		}(i)
	}

	wg.Wait()

	// Concurrent reads of target and baselines
	wg.Add(numGoroutines * 3) // 3 getters per goroutine
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_ = nm.GetTarget()   // Should not panic
			_ = nm.GetBaselineDown()
			_ = nm.GetBaselineUp()
		}(i)
	}

	wg.Wait()

	// Verify all operations completed without deadlock or race conditions
	// Just checking we got here means thread safety works
	t.Log("Concurrent access test passed - no deadlocks or panics")
}

// TestPerformanceMonitorIntegration tests that the PerformanceMonitor is properly integrated
func TestPerformanceMonitorIntegration(t *testing.T) {
	nm := NewNetworkMonitor()

	// Record a ping latency and verify it's tracked
	testLatency := 50 * time.Millisecond
	nm.trackPingLatency(testLatency)

	// Verify the average ping was calculated correctly
	result := nm.performanceMgr.GetAveragePing()
	if result != float64(50.0) { // Milliseconds
		t.Errorf("Expected average ping of 50ms, got %f", result)
	}

	// Record multiple pings and verify calculation
	nm.trackPingLatency(30 * time.Millisecond)
	nm.trackPingLatency(70 * time.Millisecond)
	result = nm.performanceMgr.GetAveragePing()
	expected := (50.0 + 30.0 + 70.0) / 3.0 // Average of all three pings
	if result != expected {
		t.Errorf("Expected average ping of %f, got %f", expected, result)
	}

	// Record speed test results and verify they're tracked
	nm.trackSpeedTest(15.0, 5.0) // down=15, up=5 Mbps each
	result = nm.performanceMgr.GetAverageSpeed()
	expected = (15.0 + 5.0) / 2.0 // Average of both recorded speeds
	if result != expected {
		t.Errorf("Expected average speed of %f, got %f", expected, result)
	}

	// Record multiple speed tests and verify calculation
	nm.trackSpeedTest(10.0, 8.0) // down=10, up=8 Mbps each
	result = nm.performanceMgr.GetAverageSpeed()
	expected = (15.0 + 5.0 + 10.0 + 8.0) / 4.0 // Average of all four recorded speeds
	if result != expected {
		t.Errorf("Expected average speed of %f, got %f", expected, result)
	}
}

// TestNilPerformanceMonitor tests that we handle nil performance manager gracefully
func TestNilPerformanceMonitor(t *testing.T) {
	nm := NewNetworkMonitor()

	// Temporarily set performanceMgr to nil to test nil checks
	nm.performanceMgr = nil

	// These should not panic even with nil performanceMgr
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("trackPingLatency panicked with nil performanceMgr: %v", r)
		}
	}()

	nm.trackPingLatency(50 * time.Millisecond) // Should handle gracefully if we add nil checks

	// Restore for other tests
	nm.performanceMgr = performance.NewPerformanceMonitor()
}

// TestEmptyStringTarget tests edge case of empty string handling in SetTarget
func TestEmptyStringTarget(t *testing.T) {
	nm := NewNetworkMonitor()

	// Empty string should be allowed (could be valid in some contexts)
	nm.SetTarget("")
	if nm.GetTarget() != "" {
		t.Errorf("Expected target to remain empty, got '%s'", nm.GetTarget())
	}

	// Set back to default
	nm.SetTarget("8.8.8.8")
	if nm.GetTarget() != "8.8.8.8" {
		t.Errorf("Expected target to be '8.8.8.8', got '%s'", nm.GetTarget())
	}

	// Negative speeds should be allowed (edge case - could represent invalid data)
	nm.setBaselineSpeeds(-10.0, 5.0) // negative download speed
	if nm.GetBaselineDown() != -10.0 {
		t.Errorf("Expected baseline down to be -10.0, got %f", nm.GetBaselineDown())
	}

	nm.setBaselineSpeeds(10.0, -5.0) // negative upload speed
	if nm.GetBaselineUp() != -5.0 {
		t.Errorf("Expected baseline up to be -5.0, got %f", nm.GetBaselineUp())
	}
}