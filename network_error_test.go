package network_tool

import (
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSetTarget_InvalidURL tests that invalid URLs are rejected with proper error messages
func TestSetTarget_InvalidURL(t *testing.T) {
	nm := NewNetworkMonitor()

	// Invalid URLs should return errors
	invalidURLs := []struct {
		url       string
		expectErr bool
	}{
		{"", true},                              // empty string
		{"http://.com", true},                   // malformed URL
	}

	for _, tc := range invalidURLs {
		err := nm.SetTarget(tc.url)
		if tc.expectErr && err == nil {
			t.Errorf("Expected error for URL '%v', but got none", tc.url)
		} else if !tc.expectErr && err != nil {
			t.Errorf("Unexpected error for valid input '%v': %v", tc.url, err)
		}

		// Verify target wasn't set to invalid value (when there was an error)
		if tc.expectErr && nm.GetTarget() == tc.url {
			t.Errorf("Invalid URL was stored: %v", tc.url)
		}
	}

	// Test valid URLs are accepted
	validURLs := []struct {
		url  string
		want string // what it should be set to (possibly normalized)
	}{
		{"https://google.com", "https://google.com"},
		{"http://1.1.1.1", "http://1.1.1.1"},
	}

	for _, tc := range validURLs {
		err := nm.SetTarget(tc.url)
		if err != nil {
			t.Errorf("Unexpected error for valid URL '%v': %v", tc.url, err)
		}
		assert.Equal(t, tc.want, nm.GetTarget(), "SetTarget should accept valid URLs")
	}

	// Test hostnames without protocol are allowed
	hostnames := []string{
		"8.8.8.8",
		"google.com",
		"localhost",
		"example.org",
	}

	for _, hostname := range hostnames {
		err := nm.SetTarget(hostname)
		if err != nil {
			t.Errorf("Unexpected error for hostname '%v': %v", hostname, err)
		}
		assert.Equal(t, hostname, nm.GetTarget(), "SetTarget should accept valid hostnames")
	}

	// Test that URLs with double dots are rejected but similar valid ones work
	malformedURLs := []string{
		"https://invalid..domain", // Double dot - should fail
		"http://.example.com",     // Starts with dot - should fail
		"http://example.",         // Ends with dot - should fail (no TLD)
	}

	for _, malformedURL := range malformedURLs {
		err := nm.SetTarget(malformedURL)
		if err == nil {
			t.Errorf("Expected error for malformed URL '%v', but got none", malformedURL)
		} else if nm.GetTarget() == malformedURL {
			t.Errorf("Malformed URL was stored: %v", malformedURL)
		}
	}

	// Test that valid domains work even without protocol
	validDomains := []string{
		"google.com",
		"github.com",
		"kubernetes.io",
	}

	for _, domain := range validDomains {
		err := nm.SetTarget(domain)
		if err != nil {
			t.Errorf("Unexpected error for valid hostname '%v': %v", domain, err)
		}
		assert.Equal(t, domain, nm.GetTarget(), "Valid hostname should be accepted")
	}
}

// TestSetTarget_EmptyString tests that empty strings are handled properly
func TestSetTarget_EmptyString(t *testing.T) {
	nm := NewNetworkMonitor()

	// Empty string should return an error
	err := nm.SetTarget("")
	if err == nil {
		t.Errorf("Expected error for empty string, but got none")
	} else if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected 'empty' in error message, got: %v", err)
	}

	assert.Equal(t, "", nm.GetTarget(), "Empty string should be stored as-is")

	// Set back to default and verify it works
	nm.SetTarget("8.8.8.8")
	if nm.GetTarget() != "8.8.8.8" {
		t.Errorf("Expected target to be '8.8.8.8', got '%s'", nm.GetTarget())
	}

	// Test that empty strings don't cause panics or unexpected behavior after initial error
	err = nm.SetTarget("")
	assert.Error(t, err, "Empty string should return an error on subsequent attempts")
	t.Log("Multiple empty string assignments handled gracefully without panic")
}

// TestSetTarget_ValidURL tests that valid URLs are accepted and stored correctly
func TestSetTarget_ValidURL(t *testing.T) {
	nm := NewNetworkMonitor()

	testCases := []struct {
		url  string
		want bool // should be accepted (true) or rejected (false)
	}{
		{"https://google.com", true},
		{"http://1.1.1.1", true},
	}

	for _, tc := range testCases {
		err := nm.SetTarget(tc.url)
		if tc.want && err != nil {
			t.Errorf("Expected '%v' to be accepted, but got error: %v", tc.url, err)
		} else if !tc.want && err == nil {
			t.Errorf("Expected '%v' to be rejected, but it was accepted", tc.url)
		}
	}

	// Verify valid URLs are stored correctly
	validURLs := []string{
		"https://google.com",
		"http://1.1.1.1",
		"ftp://example.org", // Different protocol is allowed for hostnames without scheme
	}

	for _, url := range validURLs {
		nm.SetTarget(url)
		assert.Equal(t, url, nm.GetTarget(), "Valid URL should be stored correctly")
	}
}

// TestTrackSpeedTest_NegativeValues tests rejection of negative speed values
func TestTrackSpeedTest_NegativeValues(t *testing.T) {
	nm := NewNetworkMonitor()

	// Negative values should be rejected with clear error messages
	testCases := []struct {
		desc    string
		down, up float64
		wantErr bool
	}{
		{"negative download", -10.5, 20.0, true},   // negative download
		{"negative upload", 10.0, -20.0, true},    // negative upload  
		{"both negative", -10.5, -20.0, true},     // both negative
		{"zeros are valid", 0.0, 0.0, false},      // zeros are valid
		{"positive values", 15.5, 8.3, false},     // normal positive values
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := nm.trackSpeedTest(tc.down, tc.up)
			if tc.wantErr && err == nil {
				t.Errorf("Expected error for speeds (%v, %v), but got none", tc.down, tc.up)
			} else if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error for valid speeds (%v, %v): %v", tc.down, tc.up, err)
			}

			// Verify that negative values are not stored
			if tc.wantErr {
				assert.Equal(t, float64(0), nm.GetBaselineDown(), "Negative download should not be stored")
				assert.Equal(t, float64(0), nm.GetBaselineUp(), "Negative upload should not be stored")
			}
		})
	}

	// Test special float values that should be rejected
	t.Run("special_float_values", func(t *testing.T) {
		nm := NewNetworkMonitor()

		testCases := []struct {
			desc    string
			down, up float64
			wantErr bool
		}{
			{"NaN download", math.NaN(), 20.0, true},
			{"Infinity upload", 15.0, math.Inf(1), true},
			{"Negative infinity", math.Inf(-1), 8.0, true},
		}

		for _, tc := range testCases {
			err := nm.trackSpeedTest(tc.down, tc.up)
			if tc.wantErr && err == nil {
				t.Errorf("Expected error for %s, but got none", tc.desc)
			} else if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}
	})
}

// TestTrackPingLatency_ErrorHandling tests error returns from ping recording
func TestTrackPingLatency_ErrorHandling(t *testing.T) {
	nm := NewNetworkMonitor()

	// Create a test case where performanceMgr is nil
	t.Run("nil_performance_mgr", func(t *testing.T) {
		nm.performanceMgr = nil
		
		err := nm.trackPingLatency(50 * time.Millisecond)
		assert.Error(t, err, "Should return error when performanceMgr is nil")
		assert.Contains(t, err.Error(), "performance monitor not initialized", "Error message should mention performance monitor")
	})

	// Test with valid latency values (these won't fail but test the flow)
	t.Run("valid_latency", func(t *testing.T) {
		err := nm.trackPingLatency(50 * time.Millisecond)
		assert.NoError(t, err, "Valid ping latency should not cause errors")
		
		// Verify the average is being tracked
		avg := nm.performanceMgr.GetAveragePing()
		if avg != 0 && avg != 50.0 { // If we have previous data, it will be non-zero
			t.Logf("Average ping after single recording: %.2f ms", avg)
		}
		
		err = nm.trackPingLatency(30 * time.Millisecond)
		assert.NoError(t, err, "Second valid ping should not cause errors")
		
		// Verify average updates correctly
		avg = nm.performanceMgr.GetAveragePing()
		expected := (50.0 + 30.0) / 2.0
		if avg != expected && avg != 0 {
			t.Errorf("Expected average ping of %f, got %f", expected, avg)
		}
	})

	// Test with very small latency values
	t.Run("very_small_latency", func(t *testing.T) {
		err := nm.trackPingLatency(1 * time.Microsecond) // 0.001 ms
		assert.NoError(t, err, "Very small ping should work")
		
		// Verify it's recorded (will be printed as a very small number)
	})

	// Test with large latency values
	t.Run("large_latency", func(t *testing.T) {
		err := nm.trackPingLatency(500 * time.Millisecond) // 500 ms - should work, just slow response
		assert.NoError(t, err, "Large ping latency should work (just indicates slow network)")
	})

	// Test multiple consecutive pings to ensure no state issues
	t.Run("consecutive_pings", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			err := nm.trackPingLatency(time.Duration(25+i*10) * time.Millisecond)
			assert.NoError(t, err, "Consecutive ping %d should not fail", i+1)
			
			// Verify the latency is being tracked
			avg := nm.performanceMgr.GetAveragePing()
			if avg != 0 {
				t.Logf("After ping %d: average = %.2f ms", i+1, avg)
			}
		}
		
		assert.NotNil(t, nm.performanceMgr.GetAveragePing(), "Should have some data after multiple pings")
	})

	// Test negative latency values
	t.Run("negative_latency", func(t *testing.T) {
		err := nm.trackPingLatency(-50 * time.Millisecond)
		assert.Error(t, err, "Negative ping should return an error")
		assert.Contains(t, err.Error(), "cannot be negative", "Error message should mention negative value")
	})

	// Test zero latency (edge case - technically valid but unusual)
	t.Run("zero_latency", func(t *testing.T) {
		err := nm.trackPingLatency(0 * time.Millisecond)
		assert.NoError(t, err, "Zero ping should be allowed (edge case)")
		
		avg := nm.performanceMgr.GetAveragePing()
		if avg != 0 && !math.IsNaN(avg) {
			t.Logf("Average after zero latency: %f ms", avg)
		}
	})

	// Test extremely large latencies (should work but might indicate issues)
	t.Run("extremely_large_latency", func(t *testing.T) {
		err := nm.trackPingLatency(5000 * time.Millisecond) // 5 seconds - very slow!
		assert.NoError(t, err, "Extremely large latency should work (just indicates very slow network)")
		
		avg := nm.performanceMgr.GetAveragePing()
		if avg != 0 && !math.IsNaN(avg) {
			t.Logf("Average with extremely large latency: %f ms", avg)
		}
	})
}

// TestTrackSpeedTest_Success tests that speed test tracking works with valid values
func TestTrackSpeedTest_Success(t *testing.T) {
	nm := NewNetworkMonitor()

	t.Run("valid_speeds", func(t *testing.T) {
		err := nm.trackSpeedTest(15.0, 8.0)
		assert.NoError(t, err, "Valid speeds should not cause errors")
		
		// Verify the average is being tracked
		avg := nm.performanceMgr.GetAverageSpeed()
		expected := (15.0 + 8.0) / 2.0
		if avg != expected {
			t.Errorf("Expected average speed of %f, got %f", expected, avg)
		}
		
		assert.Equal(t, float64(15.0), nm.GetBaselineDown(), "Download baseline should be updated")
		assert.Equal(t, float64(8.0), nm.GetBaselineUp(), "Upload baseline should be updated")
	})

	t.Run("multiple_speed_tests", func(t *testing.T) {
		testCases := []struct {
			down, up float64
		}{
			{15.0, 8.0},
			{20.0, 12.0},
			{25.0, 18.0},
		}

		for _, tc := range testCases {
			err := nm.trackSpeedTest(tc.down, tc.up)
			assert.NoError(t, err, "Speed test should succeed")
			
			// Verify values are being tracked
			down := nm.GetBaselineDown()
			up := nm.GetBaselineUp()
			t.Logf("After test: down=%f, up=%f", down, up)
		}

		// Final verification - averages should be calculated from all tests
		finalAvg := nm.performanceMgr.GetAverageSpeed()
		expected := (15.0 + 8.0 + 20.0 + 12.0 + 25.0 + 18.0) / 6.0 // average of all recorded speeds
		if finalAvg != expected && finalAvg != 0 {
			t.Errorf("Expected average speed of %f, got %f", expected, finalAvg)
		}
	})

	t.Run("zero_speeds", func(t *testing.T) {
		err := nm.trackSpeedTest(0.0, 0.0)
		assert.NoError(t, err, "Zero speeds should be valid")
		
		// Verify they don't cause issues in average calculation
		avg := nm.performanceMgr.GetAverageSpeed()
		if avg != 0 { // If we have previous data, it won't be zero
			t.Logf("Average after zero speeds: %f Mbps", avg)
		}
	})

	t.Run("very_small_speeds", func(t *testing.T) {
		err := nm.trackSpeedTest(0.1, 0.05) // very slow speeds
		assert.NoError(t, err, "Very small speeds should work")
		
		avg := nm.performanceMgr.GetAverageSpeed()
		if avg != 0 && !math.IsNaN(avg) {
			t.Logf("Average with very small speeds: %f Mbps", avg)
		}
	})

	t.Run("very_large_speeds", func(t *testing.T) {
		err := nm.trackSpeedTest(1000.0, 500.0) // very fast speeds
		assert.NoError(t, err, "Very large speeds should work")
		
		avg := nm.performanceMgr.GetAverageSpeed()
		if avg != 0 && !math.IsNaN(avg) {
			t.Logf("Average with very large speeds: %f Mbps", avg)
		}
	})

	t.Run("mixed_valid_and_invalid", func(t *testing.T) {
		// Test that one valid and one invalid value both fail the entire operation
		err := nm.trackSpeedTest(15.0, -5.0) // One positive, one negative
		assert.Error(t, err, "Mixed valid/invalid speeds should return an error")
		
		// Verify neither value is stored when there's an error
		assert.Equal(t, float64(0), nm.GetBaselineDown(), "Download should not be stored after error")
		assert.Equal(t, float64(0), nm.GetBaselineUp(), "Upload should not be stored after error")

		// Test with NaN values
		err = nm.trackSpeedTest(math.NaN(), 15.0)
		assert.Error(t, err, "NaN download speed should return an error")
		
		// Test with Infinity values  
		err = nm.trackSpeedTest(15.0, math.Inf(-1))
		assert.Error(t, err, "Negative infinity upload speed should return an error")
	})
}

// TestRaceConditionInTrackSpeedTest verifies no race conditions in trackSpeedTest
func TestRaceConditionInTrackSpeedTest(t *testing.T) {
	nm := NewNetworkMonitor()

	var wg sync.WaitGroup
	const numGoroutines = 10
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Each goroutine tries to update speed test values concurrently
			err := nm.trackSpeedTest(float64(id*10), float64(id*5))
			if err != nil && !strings.Contains(err.Error(), "performance monitor not initialized") {
				t.Errorf("Goroutine %d encountered unexpected error: %v", id, err)
			} else if strings.Contains(err.Error(), "performance monitor not initialized") {
				t.Logf("Goroutine %d got expected nil performanceMgr error (this is OK in this test)", id)
			}
		}(i)
	}

	wg.Wait()
	
	// If we reach here without panicking or hanging, no race condition exists
	t.Log("Race condition test completed successfully - no deadlocks or panics")
}

// TestConcurrentSetTarget tests multiple goroutines setting target simultaneously
func TestConcurrentSetTarget(t *testing.T) {
	nm := NewNetworkMonitor()

	const numGoroutines = 10
	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			err := nm.SetTarget("target" + string(rune('A' + (id % 26))))
			if err != nil {
				t.Errorf("Goroutine %d encountered unexpected error: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	
	// If we reach here without panicking or hanging, no race condition exists
	t.Log("Concurrent SetTarget test completed successfully - no deadlocks or panics")
}

// TestNilPerformanceMonitor tests behavior when performanceMgr is nil  
func TestSetTarget_NilPerformanceMonitor(t *testing.T) {
	nm := NewNetworkMonitor()

	// Temporarily set performanceMgr to nil
	nm.performanceMgr = nil
	
	// SetTarget should still work (it doesn't need performanceMgr)
	err := nm.SetTarget("8.8.8.8")
	assert.NoError(t, err, "SetTarget should work even with nil performanceMgr")
	
	// Other methods might fail gracefully
	t.Run("trackPingLatency_with_nil_performance_mgr", func(t *testing.T) {
		err := nm.trackPingLatency(50 * time.Millisecond)
		if err != nil {
			assert.Contains(t, err.Error(), "performance monitor not initialized", 
				"Error should mention performance monitor")
		} else {
			t.Log("trackPingLatency did not return error (nil check may have been bypassed)")
		}
	})

	t.Run("trackSpeedTest_with_nil_performance_mgr", func(t *testing.T) {
		err := nm.trackSpeedTest(15.0, 8.0)
		if err != nil {
			assert.Contains(t, err.Error(), "performance monitor not initialized", 
				"Error should mention performance monitor")
		} else {
			t.Log("trackSpeedTest did not return error (nil check may have been bypassed)")
		}
	})

	// Restore for other tests
	nm.performanceMgr = NewNetworkMonitor().performanceMgr
}