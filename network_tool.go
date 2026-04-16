// Usage example: How to track ping latency using PerformanceMonitor
func (nm *NetworkMonitor) trackPingLatency(latency time.Duration) {
	// Record ping latency for performance monitoring
	nm.performanceMgr.RecordPing(latency)
	
	// You can also get the average latency over all pings
	avgLatency := nm.performanceMgr.GetAveragePing()
	fmt.Printf("Current average ping: %.2f ms\n", avgLatency)
}

// Usage example: How to track speed test results using PerformanceMonitor
func (nm *NetworkMonitor) trackSpeedTest(down, up float64) {
	// Record download and upload speeds for performance monitoring
	nm.performanceMgr.RecordSpeedTest(down)
	nm.performanceMgr.RecordSpeedTest(up)
	
	// You can also get the average speed over all tests
	avgSpeed := nm.performanceMgr.GetAverageSpeed()
	fmt.Printf("Current average network speed: %.2f Mbps\n", avgSpeed)
}
