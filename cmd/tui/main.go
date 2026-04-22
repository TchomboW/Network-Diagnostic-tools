//go:build tui
// +build tui

// Package main provides the CLI/TUI entry point for the Network Diagnostic Tool.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	networktool "network_tool/lib"
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
	monitor := networktool.NewNetworkMonitor()
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
		// Web server requires -tags web to build
		// go build -tags tui,web -o network_tool ./cmd/tui
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
			fmt.Printf("\nReceived signal, shutting down...\n")
			return
		case <-ticker.C:
			// Continue loop
		}
	}
}

// runDiagnostics runs the core network diagnostic checks.
func runDiagnostics(monitor *networktool.NetworkMonitor, verbose bool) {
	target := monitor.GetTarget()

	// 1. Ping test (ICMP)
	fmt.Println("\n[PING] Testing connectivity...")
	pingResult := runPingTest(target, 4)
	loss := 0
	if pingResult.transmitted > 0 {
		loss = ((pingResult.transmitted - pingResult.received) * 100) / pingResult.transmitted
	}
	fmt.Printf("  Packets: %d/%d transmitted, %d%% loss\n",
		pingResult.transmitted, pingResult.received, loss)
	if pingResult.avgLatency > 0 {
		fmt.Printf("  Latency: min=%.1fms avg=%.1fms max=%.1fms\n",
			pingResult.minLatency, pingResult.avgLatency, pingResult.maxLatency)
	}

	// 2. DNS lookup
	fmt.Println("\n[DNS] Resolving hostname...")
	dnsResult := runDNSTest(target)
	fmt.Printf("  Time: %v\n", dnsResult.resolveTime)
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

	// 5. Speed test
	fmt.Println("\n[SPEED] Running speed test...")
	speedResult := runSpeedTest()
	fmt.Printf("  Download: %.2f Mbps\n", speedResult.download)
	fmt.Printf("  Upload:   %.2f Mbps\n", speedResult.upload)

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
	duration  time.Duration
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dialer := &net.Dialer{Timeout: 5 * time.Second}

	var minLatency, maxLatency, totalLatency float64
	received := 0

	for i := 0; i < count; i++ {
		start := time.Now()
		conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(target, "7"))
		if err == nil {
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			buf := make([]byte, 1)
			conn.Write([]byte{0})
			_, err = conn.Read(buf)
			if err == nil {
				latency := float64(time.Since(start).Microseconds()) / 1000.0
				received++
				totalLatency += latency
				if received == 1 || latency < minLatency {
					minLatency = latency
				}
				if received == 1 || latency > maxLatency {
					maxLatency = latency
				}
			}
			conn.Close()
		}
	}

	avgLatency := 0.0
	if received > 0 {
		avgLatency = totalLatency / float64(received)
	}

	return pingResult{
		transmitted: count,
		received:    received,
		avgLatency:  avgLatency,
		minLatency:  minLatency,
		maxLatency:  maxLatency,
	}
}

func runDNSTest(target string) dnsResult {
	start := time.Now()

	// Extract hostname if URL
	host := target
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "http://")
		host = strings.TrimPrefix(host, "https://")
		host = strings.Split(host, "/")[0]
		host = strings.Split(host, ":")[0]
	}

	ips, err := net.LookupIP(host)
	elapsed := time.Since(start)

	if err != nil {
		return dnsResult{
			ips:         nil,
			resolveTime: float64(elapsed.Milliseconds()),
		}
	}

	ipStrs := make([]string, len(ips))
	for i, ip := range ips {
		ipStrs[i] = ip.String()
	}

	return dnsResult{
		ips:         ipStrs,
		resolveTime: float64(elapsed.Milliseconds()),
	}
}

func runTCPTest(target string, port int) tcpResult {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(target, fmt.Sprintf("%d", port)))
	elapsed := time.Since(start)

	if err != nil {
		return tcpResult{
			success:  false,
			duration: elapsed,
		}
	}
	conn.Close()

	return tcpResult{
		success:  true,
		duration: elapsed,
	}
}

func runHTTPTest(target string) httpResult {
	// Build URL
	url := target
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	start := time.Now()
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
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

func runSpeedTest() speedResult {
	// Simple speed test using Cloudflare
	downloadStart := time.Now()
	resp, err := http.Get("https://speed.cloudflare.com/__down?bytes=1000000")
	downloadElapsed := time.Since(downloadStart)

	downloadMbps := 0.0
	if err == nil {
		buf := make([]byte, 1024)
		totalBytes := 0
		for {
			n, err := resp.Body.Read(buf)
			totalBytes += n
			if err != nil {
				break
			}
		}
		resp.Body.Close()
		downloadMbps = float64(totalBytes) * 8 / 1000000.0 / downloadElapsed.Seconds()
	}

	uploadStart := time.Now()
	data := make([]byte, 1000000)
	resp2, err := http.Post("https://speed.cloudflare.com/__up", "application/octet-stream", strings.NewReader(string(data)))
	uploadElapsed := time.Since(uploadStart)

	uploadMbps := 0.0
	if err == nil {
		resp2.Body.Close()
		uploadMbps = float64(len(data)) * 8 / 1000000.0 / uploadElapsed.Seconds()
	}

	return speedResult{
		download: downloadMbps,
		upload:   uploadMbps,
	}
}
