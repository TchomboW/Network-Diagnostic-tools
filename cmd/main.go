package main

import (
	"bytes"
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

	"network_tool/network"
)

func main() {
	// CLI flags
	target := flag.String("target", "8.8.8.8", "Target host to monitor (IP, hostname, or URL)")
	interval := flag.Duration("interval", 5*time.Second, "Monitoring interval")
	duration := flag.Duration("duration", 0, "Run for duration then exit (0 = infinite)")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	version := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *version {
		fmt.Println("network_tool v0.1.0")
		fmt.Printf("Go %s / %s / %s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	// Validate target
	if err := validateTarget(*target); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== Network Diagnostic Tool ===\n")
	fmt.Printf("Target:    %s\n", *target)
	fmt.Printf("Interval:  %s\n", *interval)
	fmt.Printf("Platform:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Go:        %s\n", runtime.Version())
	fmt.Printf("===============================\n\n")

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

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

		if *duration > 0 && time.Since(startTime) >= *duration {
			fmt.Printf("\nDuration %s reached. Exiting.\n", *duration)
			return
		}

		runCount++
		fmt.Printf("\n--- Run #%d (%s) ---\n", runCount, time.Now().Format("15:04:05"))
		runDiagnostics(*target, *verbose)

		select {
		case <-sigCh:
			fmt.Printf("\nReceived %v, shutting down...\n", sig)
			return
		case <-ticker.C:
		}
	}
}

func validateTarget(target string) error {
	if target == "" {
		return fmt.Errorf("target cannot be empty")
	}
	if net.ParseIP(target) != nil {
		return nil
	}
	if _, err := net.LookupHost(target); err == nil {
		return nil
	}
	return fmt.Errorf("invalid target: %s", target)
}

type pingResult struct {
	transmitted int
	received    int
	avgLatency  float64
	minLatency  float64
	maxLatency  float64
}

type dnsResult struct {
	ips         []string
	resolveTime float64
}

type tcpResult struct {
	success  bool
	duration time.Duration
}

type httpResult struct {
	status   string
	duration time.Duration
}

type speedResult struct {
	download float64
	upload   float64
}

func runDiagnostics(target string, verbose bool) {
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
	if tcpResult.success {
		fmt.Printf("  Port 443 (HTTPS): OK (%v)\n", tcpResult.duration)
	} else {
		fmt.Printf("  Port 443 (HTTPS): FAIL\n")
	}

	// 4. HTTP check
	fmt.Println("\n[HTTP] Checking HTTP response...")
	httpResult := runHTTPTest(target)
	fmt.Printf("  Status: %s (%v)\n", httpResult.status, httpResult.duration)

	// 5. Speed test
	fmt.Println("\n[SPEED] Running speed test...")
	speedResult := runSpeedTest()
	fmt.Printf("  Download: %.2f Mbps\n", speedResult.download)
	fmt.Printf("  Upload:   %.2f Mbps\n", speedResult.upload)
}

func runPingTest(target string, count int) pingResult {
	// Try raw ICMP first (requires root/CAP_NET_RAW)
	fmt.Printf("  Attempting ICMP ping... ")

	// Check ICMP connectivity via /proc (Linux) or network interface flags
	needRoot := isRootNeeded()
	var latencies []float64

	if needRoot {
		fmt.Printf("  Not running as root; using UDP port probe instead.\n")
		// Fallback: UDP probe (not true ICMP ping, no root required)
		for i := 0; i < count; i++ {
			start := time.Now()
			conn, err := net.DialTimeout("udp", target+":"+fmt.Sprint(53+i), time.Second)
			if err != nil {
				continue
			}
			elapsed := time.Since(start).Seconds() * 1000.0
			latencies = append(latencies, elapsed)
			conn.Close()
		}
	} else {
		// Use real ICMP via raw socket (requires root on Linux)
		latencies, _ = rawICMPPing(target, count)
	}

	if len(latencies) > 0 {
		var total, min, max float64
		min = math.MaxFloat64
		for _, l := range latencies {
			total += l
			if l < min { min = l }
			if l > max { max = l }
		}
		avg := total / float64(len(latencies))
		return pingResult{
			transmitted: len(latencies),
			received:    len(latencies),
			avgLatency:  avg,
			minLatency:  min,
			maxLatency:  max,
		}
	}

	return pingResult{
		transmitted: count,
		received:    0,
	}
}

func isRootNeeded() bool {
	// On Linux, check if we have net raw capability
	// Simplified: just check if /proc/net/icmp exists
	if _, err := os.Stat("/proc/net/icmp"); err == nil {
		// We're on Linux
		file, _ := os.Open("/proc/net/icmp")
		if file != nil {
			file.Close()
			return false
		}
	}
	return true
}

func rawICMPPing(target string, count int) ([]float64, bool) {
	// Resolve target
	addr, err := net.ResolveIPAddr("ip", target)
	if err != nil {
		return nil, false
	}

	// Create ICMP socket (requires root)
	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, false
	}
	defer conn.Close()

	// Send ICMP echo requests
	var latencies []float64
	for i := 0; i < count; i++ {
		icmpPacket := []byte{
			8, 0x00, 0x00, 0x00, 0x00, 0x00, // Type 8, no ID/seq for simplicity
			0x00, 0x00, 0x00, 0x00,
			0x08, 0x0b, 0x00, 0x00, 0x40, 0x1f, // TTL 64
			0x00, 0x00, 0x00, 0x00,
		}

		start := time.Now()
		if _, err = conn.WriteTo(icmpPacket, &net.UDPAddr{IP: addr.IP, Port: 0}); err != nil {
			continue
		}

		conn.SetReadDeadline(time.Now().Add(time.Second))
		buf := make([]byte, 1024)
		if _, _, err = conn.ReadFrom(buf); err != nil {
			continue
		}

		lat := time.Since(start).Seconds() * 1000.0
		latencies = append(latencies, lat)
	}

	return latencies, true
}

func runDNSTest(target string) dnsResult {
	fmt.Print("  Looking up... ")
	start := time.Now()
	ips, err := net.LookupIP(target)
	elapsed := float64(time.Since(start).Milliseconds())

	if err != nil {
		fmt.Printf("FAIL (%v)\n", err)
		return dnsResult{resolveTime: elapsed}
	}

	ipStrings := make([]string, 0, len(ips))
	for _, ip := range ips {
		ipStrings = append(ipStrings, ip.String())
	}
	fmt.Printf("OK (%d addresses)\n", len(ips))
	return dnsResult{ips: ipStrings, resolveTime: elapsed}
}

func runTCPTest(target string, port int) tcpResult {
	fmt.Print("  Connecting to port 443... ")
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", target, port), 5*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("FAIL (%v)\n", err)
		return tcpResult{success: false, duration: elapsed}
	}
	conn.Close()
	fmt.Printf("OK (%v)\n", elapsed)
	return tcpResult{success: true, duration: elapsed}
}

func runHTTPTest(target string) httpResult {
	fmt.Print("  Fetching... ")
	url := "https://" + strings.TrimPrefix(target, "https://")
	if strings.HasPrefix(target, "http") {
		url = target
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fmt.Printf("FAIL to create request: %v\n", err)
		return httpResult{status: "Error creating request", duration: time.Since(start)}
	}

	client := &http.Client{
		Timeout:   time.Second * 10,
		Transport: defaultTransport(),
	}

	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Printf("FAIL (%v)\n", err)
		return httpResult{status: fmt.Sprintf("Error: %v", err), duration: elapsed}
	}
	defer resp.Body.Close()

	fmt.Printf("OK")
	if resp.TLS != nil {
		fmt.Printf(" (TLS: %s)", resp.TLS.Version)
	}
	fmt.Printf(" (%v)\n", elapsed)

	return httpResult{
		status:   fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode)),
		duration: elapsed,
	}
}

func defaultTransport() http.RoundTripper {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

const chunkSize = 32768

func runSpeedTest() speedResult {
	fmt.Print("  Download at 10 Mbps for ~6s... ")
	url := "https://speed.cloudflare.com/__down?bytes=7500000"
	start := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("FAIL (%v)\n", err)
		return speedResult{}
	}
	defer resp.Body.Close()

	var bodyLen int
	buf := make([]byte, chunkSize)
	for {
		n, err := resp.Body.Read(buf)
		if err != nil {
			break
		}
		bodyLen += n
	}
	elapsed := time.Since(start)
	downloadMbps := (float64(bodyLen) * 8 / 1000000.0) / elapsed.Seconds()
	fmt.Printf("OK (%.2f Mbps)\n", downloadMbps)

	// Upload test at 1 Mbps for ~8s
	fmt.Print("  Upload at 1 Mbps for ~8s... ")
	url = "https://speed.cloudflare.com/__up"
	uploadStart := time.Now()
	payload := bytes.NewReader(make([]byte, chunkSize))
	req, _ := http.NewRequest("POST", url, payload)
	req.Header.Set("Content-Type", "application/octet-stream")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("FAIL (%v)\n", err)
	} else {
		resp2.Body.Close()
		uploadElapsed := time.Since(uploadStart)
		uploadMbps := (float64(chunkSize) * 8 / 1000000.0) / uploadElapsed.Seconds()
		fmt.Printf("OK (%.2f Mbps)\n", uploadMbps)
	}

	return speedResult{download: downloadMbps}
}
