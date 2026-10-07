package main

import (
	"bytes"
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
		case sig := <-sigCh:
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

	// Resolve target first
	addr, err := net.ResolveIPAddr("ip", target)
	if err != nil {
		fmt.Printf("DNS resolution failed: %v\n", err)
		return pingResult{
			transmitted: count,
			received:    0,
		}
	}

	// Try to open raw ICMP socket (requires root on Linux, works without on macOS)
	icmpConn, icmpErr := net.ListenPacket("ip4:icmp", "0.0.0.0")
	needRoot := icmpErr != nil
	if needRoot {
		// Raw sockets require root. Fall back to UDP probe approach.
		fmt.Printf("not running as root; using UDP port probe (not true ICMP ping).\n")
		latencies := make([]float64, 0, count)
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
		if len(latencies) > 0 {
			total, min, max := computeLatencyStats(latencies)
			return pingResult{
				transmitted: count,
				received:    len(latencies),
				avgLatency:  total,
				minLatency:  min,
				maxLatency:  max,
			}
		}
		return pingResult{
			transmitted: count,
			received:    0,
		}
	}
	defer icmpConn.Close()

	// Use real ICMP via raw socket
	fmt.Printf("using raw ICMP (root/CAP_NET_RAW).\n")
	latencies := make([]float64, 0, count)
	for i := 0; i < count; i++ {
		// Build ICMP Echo Request (Type 8, Code 0) with proper checksum
		seq := uint16(i)
		id := uint16(os.Getpid())

		// Build ICMP packet with real ID/sequence
		// Size: 64 bytes (8 header + 56 payload) - standard size for ping
		icmpPacket := make([]byte, 64)
		icmpPacket[0] = 8  // Type: Echo Request
		icmpPacket[1] = 0  // Code: 0
		icmpPacket[4] = byte(id >> 8)      // ICMP ID high byte
		icmpPacket[5] = byte(id & 0xFF)    // ICMP ID low byte
		icmpPacket[6] = byte(seq >> 8)     // Sequence high byte
		icmpPacket[7] = byte(seq & 0xFF)   // Sequence low byte
		// Payload - timestamp for round-trip measurement
		timestamp := time.Now().UnixNano() & 0xFFFFFFFF
		icmpPacket[8] = byte(timestamp >> 24)
		icmpPacket[9] = byte(timestamp >> 16)
		icmpPacket[10] = byte(timestamp >> 8)
		icmpPacket[11] = byte(timestamp)

		// Compute ICMP checksum (proper 16-bit one's complement sum per RFC 792)
		// Note: checksum field already zeroed by make()
		icmpPacket[2] = 0
		icmpPacket[3] = 0
		iccksum := uint32(0)
		for i := 0; i < len(icmpPacket); i += 2 {
			if i+1 < len(icmpPacket) {
				iccksum += uint32(icmpPacket[i])<<8 | uint32(icmpPacket[i+1])
			} else {
				iccksum += uint32(icmpPacket[i]) << 8
			}
		}
		for (iccksum >> 16) != 0 {
			iccksum = (iccksum & 0xFFFF) + (iccksum >> 16)
		}
		icmpPacket[2] = byte(^(iccksum & 0xFFFF) >> 8)
		icmpPacket[3] = byte(^((iccksum & 0xFFFF)) & 0xFF)

		start := time.Now()
		// Send
		if _, err = icmpConn.WriteTo(icmpPacket, &net.UDPAddr{IP: addr.IP, Port: 0}); err != nil {
			continue
		}
		// Wait for response with timeout
		icmpConn.SetReadDeadline(time.Now().Add(time.Second))
		recvBuf := make([]byte, 1500)
		n, _, err := icmpConn.ReadFrom(recvBuf)
		if err != nil {
			continue
		}
		// Verify response is Echo Reply (Type 0, Code 0)
		if n < 8 || recvBuf[0] != 0 || recvBuf[1] != 0 {
			continue
		}
		lat := time.Since(start).Seconds() * 1000.0
		latencies = append(latencies, lat)
	}

	if len(latencies) > 0 {
		total, min, max := computeLatencyStats(latencies)
		return pingResult{
			transmitted: count,
			received:    len(latencies),
			avgLatency:  total,
			minLatency:  min,
			maxLatency:  max,
		}
	}

	return pingResult{
		transmitted: count,
		received:    0,
	}
}

// computeLatencyStats calculates average, min, and max from a slice of latencies
func computeLatencyStats(latencies []float64) (avg float64, min float64, max float64) {
	if len(latencies) == 0 {
		return 0, 0, 0
	}
	total := 0.0
	min = latencies[0]
	max = latencies[0]
	for _, l := range latencies {
		total += l
		if l < min {
			min = l
		}
		if l > max {
			max = l
		}
	}
	avg = total / float64(len(latencies))
	return avg, min, max
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

	// Upload test - send 512KB payload to get meaningful measurement
	fmt.Print("  Upload at 10 Mbps for ~8s... ")
	url = "https://speed.cloudflare.com/__up"
	payload := bytes.NewReader(make([]byte, 512*1024))
	req, _ := http.NewRequest("POST", url, payload)
	req.Header.Set("Content-Type", "application/octet-stream")
	var uploadMbps float64
	uploadStart := time.Now()
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("FAIL (%v)\n", err)
	} else {
		resp2.Body.Close()
		uploadElapsed := time.Since(uploadStart)
		uploadMbps = (512.0 * 8 / 1000.0) / uploadElapsed.Seconds()
		fmt.Printf("OK (%.2f Mbps)\n", uploadMbps)
	}

	return speedResult{download: downloadMbps, upload: uploadMbps}
}
