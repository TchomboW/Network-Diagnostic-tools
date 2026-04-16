package performance

import (
	"testing"
	"time"
)

// BenchmarkPingPerformance measures ping performance over N iterations
func BenchmarkPingPerformance(b *testing.B) {
	pm := NewPerformanceMonitor()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testLatency := time.Duration(5+i%10) * time.Millisecond
		pm.RecordPing(testLatency)
	}
	b.StopTimer()
	
	// Verify we can get results without errors
	_ = pm.GetAveragePing()
}

// BenchmarkSpeedTestPerformance measures speed test performance
func BenchmarkSpeedTestPerformance(b *testing.B) {
	pm := NewPerformanceMonitor()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testSpeed := float64(10 + i%20) // Speed between 10-30 Mbps
		pm.RecordSpeedTest(testSpeed)
	}
	b.StopTimer()
	
	// Verify we can get results without errors
	_ = pm.GetAverageSpeed()
}

// BenchmarkUIRenderingPerformance measures UI rendering overhead
func BenchmarkUIRenderingPerformance(b *testing.B) {
	pm := NewPerformanceMonitor()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testRenderTime := time.Duration(10+i%20) * time.Millisecond
		pm.RecordUIRender(testRenderTime)
	}
	b.StopTimer()
	
	// Verify we can get results without errors
	_ = pm.GetAverageRenderTime()
}
