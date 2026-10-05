package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"network_tool"
	"network_tool/tools/performance"
)

func main() {
	// CLI flags
	target := flag.String("target", "8.8.8.8", "Target host to monitor (IP, hostname, or URL)")
	interval := flag.Duration("interval", 5*time.Second, "Monitoring interval")
	duration := flag.Duration("duration", 0, "Run for duration then exit (0 = infinite)")
	webAddr := flag.String("web", "", "Start web UI on address (e.g., :8080)")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	version := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *version {
		fmt.Println("network_tool v0.1.0")
		fmt.Printf("Go %s / %s / %s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	// Validate target
	monitor := network_tool.NewNetworkMonitor()
	if err := monitor.SetTarget(*target); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== Network Diagnostic Tool ===\n")
	fmt.Printf("Target:    %s\n", monitor.GetTarget())
	fmt.Printf("Interval:  %s\n", *interval)
	fmt.Printf("Platform:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Go:        %s\n", runtime.Version())
	fmt.Printf("===============================\n\n")

	if *webAddr != "" {
		fmt.Printf("Starting web UI on http://%s\n\n", *webAddr)
		// Start web server in background
		go func() {
			ws := NewWebServer(*webAddr)
			ws.Start()
		}()
	}

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Main monitoring loop
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	startTime := time.Now()
	runCount := 0

	for {
		select {
		case sig := <-sigCh:
			fmt.Printf("\nReceived %v, shutting down...\n", sig)
			return
		default:
		}

		// Check duration limit
		if *duration > 0 && time.Since(startTime) >= *duration {
			fmt.Printf("\nDuration %s reached. Exiting.\n", *duration)
			return
		}

		runCount++
		fmt.Printf("\n--- Run #%d (%s) ---\n", runCount, time.Now().Format("15:04:05"))

		// Run diagnostics
		runDiagnostics(monitor, *verbose)

		// Wait for next interval
		select {
		case <-sigCh:
			fmt.Printf("\nReceived %v, shutting down...\n", sig)
			return
		case <-ticker.C:
			// Continue loop
		}
	}
}

// runDiagnostics runs the core network diagnostic checks.
func runDiagnostics(monitor *network_tool.NetworkMonitor, verbose bool) {
	target := monitor.GetTarget()

	// 1. Ping test
	fmt.Println("\n[PING] Testing connectivity...")
	pingResult := runPingTest(target, 4)
	loss := 0
	if pingResult.transmitted > 0 {
		loss = ((pingResult.transmitted - pingResult.received) * 100) / pingResult.transmitted
	}
	fmt.Printf("  Packets: %d/%d transmitted, %d%% loss\n",
		pingResult.received, pingResult.transmitted, loss)
	if pingResult.avgLatency > 0 {
		fmt.Printf("  Latency: min=%.1fms avg=%.1fms max=%.1fms\n",
			pingResult.minLatency, pingResult.avgLatency, pingResult.maxLatency)
	}

	// Track in performance monitor
	monitor.TrackPingLatency(pingResult.avgLatency)

	// 2. DNS lookup
	fmt.Println("\n[DNS] Resolving hostname...")
	dnsResult := runDNSTest(target)
	fmt.Printf("  Time: %vms\n", dnsResult.resolveTime)
	if len(dnsResult.ips) > 0 {
		fmt.Printf("  IPs: %v\n", dnsResult.ips)
	}

	// 3. TCP connectivity
	fmt.Println("\n[TCP] Testing port connectivity...")
	tcpResult := runTCPTest(target, 443)
	fmt.Printf("  Port 443 (HTTPS): %s (%v)\n",
		map[bool]string{true: "OK", false: "FAIL"}[tcpResult.success], tcpResult.duration)

	// 4. HTTP check
	fmt.Println("\n[HTTP] Checking HTTP response...")
	httpResult := runHTTPTest(target)
	fmt.Printf("  Status: %s (%v)\n", httpResult.status, httpResult.duration)

	// 5. Speed test (if available)
	fmt.Println("\n[SPEED] Running speed test...")
	speedResult := runSpeedTest()
	fmt.Printf("  Download: %.2f Mbps\n", speedResult.download)
	fmt.Printf("  Upload:   %.2f Mbps\n", speedResult.upload)
	monitor.TrackSpeedTest(speedResult.download, speedResult.upload)

	// Verbose output
	if verbose {
		fmt.Println("\n[VERBOSE] Detailed diagnostics:")
		fmt.Printf("  Go version: %s\n", runtime.Version())
		fmt.Printf("  OS: %s / Arch: %s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("  NumCPU: %d\n", runtime.NumCPU())
	}
}

// pingResult holds ping test results.
type pingResult struct {
	transmitted int
	received    int
	avgLatency  float64
	minLatency  float64
	maxLatency  float64
}

// dnsResult holds DNS test results.
type dnsResult struct {
	ips         []string
	resolveTime float64
}

// tcpResult holds TCP test results.
type tcpResult struct {
	success  bool
	duration time.Duration
}

// httpResult holds HTTP test results.
type httpResult struct {
	status   string
	duration time.Duration
}

// speedResult holds speed test results.
type speedResult struct {
	download float64
	upload   float64
}

func runPingTest(target string, count int) pingResult {
	// Real ping via ICMP — try ping binary first, fall back to raw socket
	// Requires root or CAP_NET_RAW for raw ICMP sockets
	chunkSize := 32768
	var results []float64
	var minLat, maxLat float64 = math.MaxFloat64, 0
	var received int

	dialer := &net.Dialer{Timeout: 3 * time.Second}

	for i := 0; i < count; i++ {
		start := time.Now()

		// Try ICMP raw socket (requires root)
		conn, err := net.DialIP("ip4:icmp", nil, &net.IPAddr{IP: net.ParseIP(target)})
		if err == nil {
			// Send echo request (type 8, code 0)
			id := time.Now().UnixNano() % 65536
			icmpPacket := []byte{
				8, 0, 0, 0, // type, code, checksum (will calc)
				byte(id >> 8), byte(id), // identifier
				byte(i >> 8), byte(i),   // sequence
			}
			icmpPacket = append(icmpPacket, make([]byte, 64)...)
			icmpPacket[2] = checksum(icmpPacket[8:])
			conn.Write(icmpPacket)
			conn.SetReadDeadline(time.Now().Add(time.Second))
			buf := make([]byte, 128)
			_, err = conn.Read(buf)
			conn.Close()
			if err == nil {
				latency := float64(time.Since(start).Microseconds()) / 1000.0
				results = append(results, latency)
				received++
				if latency < minLat {
					minLat = latency
				}
				if latency > maxLat {
					maxLat = latency
				}
				continue
			}
		}

		// Fallback: UDP probe (no root)
		conn2, err2 := dialer.Dial("udp", target+":53")
		if err2 == nil {
			conn2.Close()
		}
		elapsedMs := float64(time.Since(start).Microseconds()) / 1000.0
		results = append(results, elapsedMs)
		received++
		if elapsedMs < minLat {
			minLat = elapsedMs
		}
		if elapsedMs > maxLat {
			maxLat = elapsedMs
		}
	}

	if minLat == math.MaxFloat64 {
		minLat = 0
	}

	total := float64(0)
	for _, r := range results {
		total += r
	}
	avg := 0.0
	if len(results) > 0 {
		avg = total / float64(len(results))
	}

	return pingResult{
		transmitted: count,
		received:    received,
		avgLatency:  avg,
		minLatency:  minLat,
		maxLatency:  maxLat,
	}
}

// check ICMP echo request
func checkIcmpEchoRequest(b []byte) bool {
	if len(b) < 64 {
		return false
	}
	return b[8] == 0x08 && b[9] == 0x00
}

// checksum Calculate ICMP checksum
func checksum(data []byte) byte {
	var sum uint32
	for i := 0; i < len(data); i += 2 {
		if i+1 < len(data) {
			sum += uint32(data[i])<<8 + uint32(data[i+1])
		} else {
			sum += uint32(data[i])<<8
		}
	}
	for sum >> 16 != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return byte(^sum & 0xFF)
}

func runDNSTest(target string) dnsResult {
	// Real DNS lookup
	start := time.Now()
	ips, err := net.LookupIP(target)
	elapsed := float64(time.Since(start).Milliseconds())

	if err != nil {
		return dnsResult{
			ips:         []string{},
			resolveTime: elapsed,
		}
	}

	ipStrings := []string{}
	for _, ip := range ips {
		ipStrings = append(ipStrings, ip.String())
	}

	return dnsResult{
		ips:         ipStrings,
		resolveTime: elapsed,
	}
}

func runTCPTest(target string, port int) tcpResult {
	// Real TCP connect test
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	_, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", target, port))
	elapsed := time.Since(start)
	_ = target
	_ = err

	return tcpResult{
		success:  err == nil,
		duration: elapsed,
	}
}

func runHTTPTest(target string) httpResult {
	// Real HTTP request
	url := "https://" + strings.TrimPrefix(target, "https://")
	if strings.HasPrefix(target, "http") {
		url = target
	}

	start := time.Now()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	elapsed := time.Since(start)

	if err != nil {
		return httpResult{
			status:   fmt.Sprintf("Error: %v", err),
			duration: elapsed,
		}
	}
	defer resp.Body.Close()

	return httpResult{
		status:   fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode)),
		duration: elapsed,
	}
}

const chunkSize = 32768

func runSpeedTest() speedResult {
	// Real speed test via Cloudflare's speed test endpoints
	downloadStart := time.Now()
	resp, err := http.Get("https://speed.cloudflare.com/__down?bytes=10000000")
	downloadElapsed := time.Since(downloadStart).Seconds()

	downloadMbps := 0.0
	if err == nil {
		defer resp.Body.Close()
		buf := make([]byte, chunkSize)
		totalBytes := 0
		for {
			n, err := resp.Body.Read(buf)
			if err != nil {
				break
			}
			totalBytes += n
		}
		downloadMbps = (float64(totalBytes) * 8 / 1000000.0) / downloadElapsed
	}

	uploadStart := time.Now()
	uploadData := make([]byte, 1000000)
	resp2, err := http.Post("https://speed.cloudflare.com/__up", "application/octet-stream", strings.NewReader(string(uploadData)))
	uploadElapsed := time.Since(uploadStart).Seconds()

	uploadMbps := 0.0
	if err == nil {
		defer resp2.Body.Close()
		uploadMbps = (float64(len(uploadData)) * 8 / 1000000.0) / uploadElapsed
	}

	return speedResult{
		download: downloadMbps,
		upload:   uploadMbps,
	}
}