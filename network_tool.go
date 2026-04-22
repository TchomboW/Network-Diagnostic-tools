package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rivo/tview"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/term"
)

type AppState struct {
	mu       sync.Mutex
	isPaused bool
}

type NetworkMonitor struct {
	mu           sync.RWMutex
	target       string
	baselineDown float64
	baselineUp   float64
	pinger       Pinger
}

func (nm *NetworkMonitor) GetTarget() string {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.target
}

func (nm *NetworkMonitor) SetTarget(t string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.target = t
}

func (nm *NetworkMonitor) GetBaselineDown() float64 {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.baselineDown
}

func (nm *NetworkMonitor) GetBaselineUp() float64 {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.baselineUp
}

func (nm *NetworkMonitor) setBaselineSpeeds(down, up float64) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.baselineDown = down
	nm.baselineUp = up
}

func NewNetworkMonitor() *NetworkMonitor {
	return &NetworkMonitor{
		target: "8.8.8.8",
		pinger: &RealPinger{timeout: 2 * time.Second, pingCount: 4},
	}
}

type PingResult struct {
	latency   time.Duration
	success   bool
	loss      float64
	jitter    float64 // in milliseconds
	timestamp time.Time
	errors    []string
}

type Pinger interface {
	pingTarget(target string, useUDP bool) (PingResult, error)
}

type RealPinger struct {
	timeout   time.Duration
	pingCount int
}

func (p *RealPinger) pingTarget(target string, useUDP bool) (PingResult, error) {
	result := PingResult{timestamp: time.Now()}
	ipAddr, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		result.errors = append(result.errors, fmt.Sprintf("DNS resolution failed: %v", err))
		return result, fmt.Errorf("failed to resolve IP address: %w", err)
	}

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		result.errors = append(result.errors, fmt.Sprintf("failed to listen for ICMP: %v", err))
		return result, fmt.Errorf("failed to listen for ICMP: %w", err)
	}
	defer conn.Close()

	var latencies []time.Duration
	sent := 0
	received := 0

	for i := 0; i < p.pingCount; i++ {
		msg := icmp.Message{
			Type: ipv4.ICMPTypeEcho, Code: 0,
			Body: &icmp.Echo{ID: os.Getpid() & 0xffff, Seq: i + 1, Data: []byte("ping")},
		}
		msgBytes, err := msg.Marshal(nil)
		if err != nil {
			result.errors = append(result.errors, fmt.Sprintf("failed to marshal ICMP: %v", err))
			continue
		}

		start := time.Now()
		if _, err := conn.WriteTo(msgBytes, &net.IPAddr{IP: ipAddr.IP}); err != nil {
			result.errors = append(result.errors, fmt.Sprintf("failed to send ICMP: %v", err))
			continue
		}
		sent++

		reply := make([]byte, 1500)
		conn.SetReadDeadline(time.Now().Add(p.timeout))
		_, _, err = conn.ReadFrom(reply)
		rtt := time.Since(start)
		if err != nil {
			result.errors = append(result.errors, fmt.Sprintf("ICMP timeout: %v", err))
			continue
		}
		latencies = append(latencies, rtt)
		received++
	}

	if received == 0 {
		return result, fmt.Errorf("no ICMP replies received")
	}

	// Calculate average latency
	var total time.Duration
	for _, lat := range latencies {
		total += lat
	}
	result.latency = total / time.Duration(received)
	result.success = true

	// Calculate loss
	result.loss = float64(sent-received) / float64(sent) * 100

	// Calculate jitter (standard deviation of latencies in ms)
	if len(latencies) > 1 {
		var sumSquares float64
		avgMs := float64(result.latency.Milliseconds())
		for _, lat := range latencies {
			diff := float64(lat.Milliseconds()) - avgMs
			sumSquares += diff * diff
		}
		variance := sumSquares / float64(len(latencies)-1)
		result.jitter = math.Sqrt(variance)
	} else {
		result.jitter = 0
	}

	return result, nil
}

type PingStub struct {
	pingTargetFunc func(target string, useUDP bool) (PingResult, error)
}

func (s *PingStub) pingTarget(target string, useUDP bool) (PingResult, error) {
	if s.pingTargetFunc != nil {
		return s.pingTargetFunc(target, useUDP)
	}
	return PingResult{latency: 10 * time.Millisecond, success: true, timestamp: time.Now()}, nil
}

func NewPingStub() *PingStub {
	return &PingStub{
		pingTargetFunc: func(target string, useUDP bool) (PingResult, error) {
			return PingResult{
				latency:   15 * time.Millisecond,
				success:   true,
				timestamp: time.Now(),
			}, nil
		},
	}
}

func NewAppState() *AppState {
	return &AppState{}
}

func (s *AppState) TogglePause() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isPaused = !s.isPaused
	return s.isPaused
}

func (s *AppState) IsPaused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isPaused
}

func (s *AppState) InitSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isPaused = false
}

type NetworkMetrics struct {
	latency        time.Duration
	jitter         float64
	loss           float64
	success        bool
	timestamp      time.Time
	dnsTime        time.Duration
	tcpLatency     time.Duration
	httpTTFB       time.Duration
	downloadSpeed  float64 // Mbps
	uploadSpeed    float64 // Mbps
	zscalerLatency time.Duration
	issueDetected  bool
	issueSummary   string
	errors         []string
}

func (nm *NetworkMonitor) runSpeedTest() (down float64, up float64, err error) {
	down, up, err = nm.performSpeedTest()
	if err == nil && (down > 0 || up > 0) {
		return down, up, nil
	}
	// Retry once if the initial attempt failed or returned no meaningful values.
	time.Sleep(500 * time.Millisecond)
	down, up, err = nm.performSpeedTest()
	return down, up, err
}

func (nm *NetworkMonitor) performSpeedTest() (float64, float64, error) {
	testURL := "https://speed.cloudflare.com/__down?bytes=10485760"
	client := &http.Client{Timeout: 15 * time.Second}

	start := time.Now()
	resp, err := client.Get(testURL)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return 0, 0, err
	}
	duration := time.Since(start).Seconds()
	if duration <= 0 {
		return 0, 0, fmt.Errorf("invalid download duration")
	}
	down := (float64(n) * 8) / (1024 * 1024) / duration

	start = time.Now()
	dummyData := make([]byte, 5*1024*1024)
	resp, err = client.Post("https://httpbin.org/post", "application/octet-stream", bytes.NewReader(dummyData))
	if err != nil {
		return down, 0, err
	}
	defer resp.Body.Close()
	duration = time.Since(start).Seconds()
	if duration <= 0 {
		return down, 0, fmt.Errorf("invalid upload duration")
	}
	up := (float64(len(dummyData)) * 8) / (1024 * 1024) / duration
	return down, up, nil
}

func (nm *NetworkMonitor) runZscalerSpeedTest() (time.Duration, error) {
	url := "https://www.zscaler.net/"
	client := &http.Client{Timeout: 10 * time.Second}

	start := time.Now()
	resp, err := client.Get(url)
	if err == nil {
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		return time.Since(start), nil
	}

	// Retry once when the first attempt fails.
	time.Sleep(500 * time.Millisecond)
	start = time.Now()
	resp, err = client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return time.Since(start), nil
}

func (nm *NetworkMonitor) initializeBaselineSpeeds() {
	down, up, err := nm.runSpeedTest()
	if err == nil {
		nm.setBaselineSpeeds(down, up)
	}
}

func (nm *NetworkMonitor) testDNSResolution(hostname string) (time.Duration, error) {
	if net.ParseIP(hostname) != nil {
		return 0, nil
	}

	dnsServers := []string{"8.8.8.8:53", "1.1.1.1:53"}
	var best time.Duration
	var success bool
	var lastErr error

	for _, server := range dnsServers {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 2 * time.Second}
				return d.DialContext(ctx, network, server)
			},
		}
		_, err := resolver.LookupIPAddr(ctx, hostname)
		cancel()
		if err == nil {
			elapsed := time.Since(start)
			if !success || elapsed < best {
				best = elapsed
			}
			success = true
			continue
		}
		lastErr = err
	}

	if success {
		return best, nil
	}
	return 0, lastErr
}

func (nm *NetworkMonitor) testTCPLatency(target string, port int) (time.Duration, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(target, fmt.Sprintf("%d", port)), 2*time.Second)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	return time.Since(start), nil
}

func (nm *NetworkMonitor) testHTTPLatency(url string) (time.Duration, error) {
	start := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return time.Since(start), nil
}

func createMetricsGrid() *tview.Grid {
	grid := tview.NewGrid().SetRows(0, 0, 0, 0, 0, 0, 0).SetColumns(0, 0).SetBorders(true)
	return grid
}

func generateProgressBar(value float64, max float64, color string) string {
	if value <= 0 || max <= 0 {
		return ""
	}
	percentage := (value / max) * 100
	if percentage > 100 {
		percentage = 100
	}
	barLength := 10
	filledLength := int((percentage / 100) * float64(barLength))
	var bar string
	for i := 0; i < barLength; i++ {
		if i < filledLength {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	tagColor := "white"
	switch color {
	case "#f00":
		tagColor = "red"
	case "#ff0":
		tagColor = "yellow"
	case "#0f0":
		tagColor = "green"
	case "#0ff":
		tagColor = "cyan"
	}
	return fmt.Sprintf("[%s]%s[white]", tagColor, bar)
}

func createMetricPanel(label string, value string, color string) *tview.TextView {
	text := tview.NewTextView().SetDynamicColors(true)
	if color == "#f00" {
		text.SetText(fmt.Sprintf("[white]%s[white]\n[red]%s[white]", label, value))
	} else if color == "#ff0" {
		text.SetText(fmt.Sprintf("[white]%s[white]\n[yellow]%s[white]", label, value))
	} else if color == "#0f0" {
		text.SetText(fmt.Sprintf("[white]%s[white]\n[green]%s[white]", label, value))
	} else if color == "#0ff" {
		text.SetText(fmt.Sprintf("[white]%s[white]\n[cyan]%s[white]", label, value))
	} else {
		text.SetText(fmt.Sprintf("[white]%s[white]\n[magenta]%s[white]", label, value))
	}
	return text
}

func createPotentialIssuesPanel() *tview.TextView {
	text := tview.NewTextView().SetDynamicColors(true)
	text.SetText("[white]POTENTIAL ISSUES:[white]\n[gray]Analyzing network conditions...[white]")
	return text
}

func createRecommendationsPanel() *tview.TextView {
	text := tview.NewTextView().SetDynamicColors(true)
	text.SetText("[white]RECOMMENDATIONS:[white]\n[gray]Waiting for sustained issue to begin analysis...[white]")
	return text
}

func createEventLog() *tview.TextView {
	text := tview.NewTextView().SetDynamicColors(true).SetMaxLines(100)
	text.SetBorder(true).SetTitle("Event Log")
	return text
}

type MonitorEvent struct {
	Metrics NetworkMetrics
	Error   error
}

type MonitorEngine struct {
	monitor         *NetworkMonitor
	state           *AppState
	events          chan MonitorEvent
	listeners       []chan MonitorEvent
	stop            chan struct{}
	lastSpeedTest   time.Time
	lastDownload    float64
	lastUpload      float64
}

func NewMonitorEngine(m *NetworkMonitor, s *AppState) *MonitorEngine {
	return &MonitorEngine{
		monitor:       m,
		state:         s,
		events:        make(chan MonitorEvent, 10),
		listeners:     make([]chan MonitorEvent, 0),
		stop:          make(chan struct{}),
		lastSpeedTest: time.Now().Add(-61 * time.Second), // run immediately on first cycle
	}
}

// AddListener adds a listener channel for broadcasting events.
func (e *MonitorEngine) AddListener(ch chan MonitorEvent) {
	e.listeners = append(e.listeners, ch)
}

func (e *MonitorEngine) Start() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if e.state.IsPaused() {
					continue
				}
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[engine] panic in runCycle: %v", r)
						}
					}()
					metrics, err := e.runCycle()
					event := MonitorEvent{Metrics: metrics, Error: err}
					// Broadcast to all listeners
					for _, ch := range e.listeners {
						select {
						case ch <- event:
						default:
						}
					}
				}()
			case <-e.stop:
				return
			}
		}
	}()
}

func (e *MonitorEngine) Stop() {
	close(e.stop)
}

func (e *MonitorEngine) runCycle() (NetworkMetrics, error) {
	var metrics NetworkMetrics

	target := e.monitor.GetTarget()

	// DNS Resolution
	dnsTime, err := e.monitor.testDNSResolution(target)
	if err != nil {
		metrics.errors = append(metrics.errors, fmt.Sprintf("dns resolution failed: %v", err))
	} else {
		metrics.dnsTime = dnsTime
	}

	// Ping/Latency (using stub for now as per existing logic)
	result, err := e.monitor.pinger.pingTarget(target, true)
	if err != nil {
		metrics.errors = append(metrics.errors, fmt.Sprintf("ping failed: %v", err))
	} else {
		metrics.latency = result.latency
		metrics.success = result.success
		metrics.loss = result.loss
		metrics.jitter = result.jitter
		if len(result.errors) > 0 {
			metrics.errors = append(metrics.errors, result.errors...)
		}
	}

	// TCP Latency
	tcpLat, err := e.monitor.testTCPLatency(target, 443)
	if err != nil {
		metrics.errors = append(metrics.errors, fmt.Sprintf("tcp latency failure: %v", err))
	} else {
		metrics.tcpLatency = tcpLat
	}

	// HTTP TTFB (only for hostname targets, not direct IPs)
	if net.ParseIP(target) == nil {
		httpUrl := "https://" + target
		httpTtfb, err := e.monitor.testHTTPLatency(httpUrl)
		if err != nil {
			metrics.errors = append(metrics.errors, fmt.Sprintf("http ttfb failure: %v", err))
		} else {
			metrics.httpTTFB = httpTtfb
		}
	}

	// Speed Test (run every 60 seconds)
	if time.Since(e.lastSpeedTest) > 60*time.Second {
		down, up, err := e.monitor.runSpeedTest()
		if err != nil {
			metrics.errors = append(metrics.errors, fmt.Sprintf("speed test failure: %v", err))
		} else {
			e.lastDownload = down
			e.lastUpload = up
		}
		e.lastSpeedTest = time.Now()
	}
	metrics.downloadSpeed = e.lastDownload
	metrics.uploadSpeed = e.lastUpload

	// Zscaler Speed Test is optional for systems with Zscaler installed.
	zscalerLatency, zErr := e.monitor.runZscalerSpeedTest()
	if zErr == nil {
		metrics.zscalerLatency = zscalerLatency
	} else {
		metrics.zscalerLatency = 0
	}

	baselineDown := e.monitor.GetBaselineDown()
	baselineUp := e.monitor.GetBaselineUp()
	var issues []string
	if len(metrics.errors) > 0 {
		issues = append(issues, "errors detected")
	}
	if baselineDown > 0 && metrics.downloadSpeed > 0 && metrics.downloadSpeed < baselineDown*0.75 {
		issues = append(issues, fmt.Sprintf("download %.2f lower than baseline %.2f", metrics.downloadSpeed, baselineDown))
	}
	if baselineUp > 0 && metrics.uploadSpeed > 0 && metrics.uploadSpeed < baselineUp*0.75 {
		issues = append(issues, fmt.Sprintf("upload %.2f lower than baseline %.2f", metrics.uploadSpeed, baselineUp))
	}
	if metrics.zscalerLatency > 2*time.Second {
		issues = append(issues, fmt.Sprintf("zscaler latency high: %v", metrics.zscalerLatency))
	}
	if metrics.dnsTime > 2*time.Second {
		issues = append(issues, fmt.Sprintf("dns lookup slow: %v", metrics.dnsTime))
	}
	if metrics.tcpLatency > 3*time.Second {
		issues = append(issues, fmt.Sprintf("tcp latency high: %v", metrics.tcpLatency))
	}

	if len(issues) > 0 {
		metrics.issueDetected = true
		metrics.issueSummary = strings.Join(issues, "; ")
	} else {
		metrics.issueDetected = false
		metrics.issueSummary = "No significant issues detected."
	}

	return metrics, nil
}

func main() {
	// Check for --web flag
	if len(os.Args) > 1 && os.Args[1] == "--web" {
		server := NewWebServer(":8080")
		server.Start()
		return
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Println("This application requires an interactive terminal. Please run in a terminal emulator.")
		os.Exit(1)
	}

	app := tview.NewApplication()
	mainPage := tview.NewFlex().SetDirection(tview.FlexRow)
	header := tview.NewTextView().SetTextAlign(tview.AlignCenter).SetText("Network Monitoring Dashboard")
	grid := createMetricsGrid()

	latencyPanel := createMetricPanel("LATENCY", "0ms", "#0f0")
	jitterPanel := createMetricPanel("JITTER", "0ms", "#ff0")
	lossPanel := createMetricPanel("PACKET LOSS", "0%", "#f00")
	tcpPanel := createMetricPanel("TCP (443)", "N/A", "#0ff")
	dnsPanel := createMetricPanel("DNS RESOLUTION", "N/A", "#ff0")
	httpPanel := createMetricPanel("HTTP TTFB", "N/A", "#f80")
	downloadPanel := createMetricPanel("DOWNLOAD", "0 Mbps", "#0ff")
	uploadPanel := createMetricPanel("UPLOAD", "0 Mbps", "#0f0")
	zscalerPanel := createMetricPanel("ZSCALER", "N/A", "#ff0")
	baselinePanel := createMetricPanel("BASELINE", "DL 0 / UL 0", "#0ff")
	errorPanel := createMetricPanel("ERROR SAMPLES", "[green]No errors detected.[white]", "#f00")

	grid.AddItem(latencyPanel, 0, 0, 1, 1, 0, 0, true)
	grid.AddItem(jitterPanel, 0, 1, 1, 1, 0, 0, false)
	grid.AddItem(lossPanel, 1, 0, 1, 1, 0, 0, true)
	grid.AddItem(tcpPanel, 1, 1, 1, 1, 0, 0, false)
	grid.AddItem(dnsPanel, 2, 0, 1, 1, 0, 0, true)
	grid.AddItem(httpPanel, 2, 1, 1, 1, 0, 0, false)
	grid.AddItem(downloadPanel, 3, 0, 1, 1, 0, 0, true)
	grid.AddItem(uploadPanel, 3, 1, 1, 1, 0, 0, false)
	grid.AddItem(zscalerPanel, 4, 0, 1, 1, 0, 0, true)
	grid.AddItem(baselinePanel, 4, 1, 1, 1, 0, 0, false)
	grid.AddItem(errorPanel, 5, 0, 1, 2, 0, 0, false)

	issuesPanel := createPotentialIssuesPanel()
	recommendationsPanel := createRecommendationsPanel()
	logView := createEventLog()
	state := NewAppState()
	monitor := NewNetworkMonitor()

	monitor.initializeBaselineSpeeds()
	if monitor.GetBaselineDown() > 0 || monitor.GetBaselineUp() > 0 {
		fmt.Fprintf(logView, "[green]Initial baseline speed test: DL %.2f Mbps / UL %.2f Mbps[white]\n", monitor.GetBaselineDown(), monitor.GetBaselineUp())
	} else {
		fmt.Fprintf(logView, "[yellow]Initial baseline speed test failed or returned zero values.[white]\n")
	}

	// Initialize Engine
	engine := NewMonitorEngine(monitor, state)
	engine.Start()

	// Register TUI as a listener
	tuiEvents := make(chan MonitorEvent, 10)
	engine.AddListener(tuiEvents)

	var lastMetrics NetworkMetrics

	// UI Update Loop (Consumes engine events)
	go func() {
		for event := range tuiEvents {
			if event.Error != nil {
				app.QueueUpdateDraw(func() {
					fmt.Fprintf(logView, "[red]ERROR: %v[white]\n", event.Error)
				})
				continue
			}

			metrics := event.Metrics
			app.QueueUpdateDraw(func() {
				latencyBar := generateProgressBar(float64(metrics.latency.Milliseconds()), 500, "#0f0")
				latencyPanel.SetText(fmt.Sprintf("[white]LATENCY[white]\n[green]%v %s[white]", metrics.latency, latencyBar))
				jitterPanel.SetText(fmt.Sprintf("[white]JITTER[white]\n[yellow]%.1fms[white]", metrics.jitter))
				lossBar := generateProgressBar(metrics.loss, 100, "#f00")
				lossPanel.SetText(fmt.Sprintf("[white]PACKET LOSS[white]\n[red]%.1f%% %s[white]", metrics.loss, lossBar))
				tcpPanel.SetText(fmt.Sprintf("[white]TCP (443)[white]\n[cyan]%v[white]", metrics.tcpLatency))
				dnsPanel.SetText(fmt.Sprintf("[white]DNS RESOLUTION[white]\n[yellow]%v[white]", metrics.dnsTime))
				httpPanel.SetText(fmt.Sprintf("[white]HTTP TTFB[white]\n[magenta]%v[white]", metrics.httpTTFB))

				downBar := generateProgressBar(metrics.downloadSpeed, 100, "#0ff")
				uploadBar := generateProgressBar(metrics.uploadSpeed, 100, "#0f0")
				downloadPanel.SetText(fmt.Sprintf("[white]DOWNLOAD[white]\n[cyan]%.2f Mbps %s[white]", metrics.downloadSpeed, downBar))
				uploadPanel.SetText(fmt.Sprintf("[white]UPLOAD[white]\n[green]%.2f Mbps %s[white]", metrics.uploadSpeed, uploadBar))
				zscalerBar := generateProgressBar(float64(metrics.zscalerLatency.Milliseconds()), 2000, "#ff0")
				zscalerValue := "N/A"
				if metrics.zscalerLatency > 0 {
					zscalerValue = fmt.Sprintf("%v %s", metrics.zscalerLatency, zscalerBar)
				}
				zscalerPanel.SetText(fmt.Sprintf("[white]ZSCALER[white]\n[yellow]%s[white]", zscalerValue))
				baselinePanel.SetText(fmt.Sprintf("[white]BASELINE[white]\n[cyan]DL %.2f Mbps / UL %.2f Mbps[white]", monitor.GetBaselineDown(), monitor.GetBaselineUp()))

				issuesText := "[white]POTENTIAL ISSUES:[white]\n"
				if metrics.issueDetected {
					issuesText += fmt.Sprintf("[red]%s[white]\n", metrics.issueSummary)
				} else {
					issuesText += "[green]No issues detected.[white]"
				}
				issuesPanel.SetText(issuesText)

				recommendationsText := "[white]RECOMMENDATIONS:[white]\n"
				if metrics.issueDetected {
					if metrics.downloadSpeed > 0 && monitor.GetBaselineDown() > 0 && metrics.downloadSpeed < monitor.GetBaselineDown()*0.75 {
						recommendationsText += "[yellow]- Your download speed dropped significantly below baseline.[white]\n"
					}
					if metrics.zscalerLatency > 2*time.Second {
						recommendationsText += "[yellow]- Zscaler latency is high; check VPN or proxy routes.[white]\n"
					}
					if len(metrics.errors) > 0 {
						recommendationsText += "[yellow]- Re-run the tests and inspect errors in the event log.[white]\n"
					}
				} else {
					recommendationsText += "[green]- Network looks stable. Continue monitoring in real time.[white]"
				}
				recommendationsPanel.SetText(recommendationsText)

				errorText := "[white]ERROR SAMPLES (last 3):[white]\n"
				if len(metrics.errors) > 0 {
					for i, errStr := range metrics.errors {
						if i >= 3 {
							break
						}
						errorText += fmt.Sprintf("[red]%d. %s[white]\n", i+1, errStr)
					}
				} else {
					errorText += "[green]No errors detected.[white]"
				}
				errorPanel.SetText(errorText)
				lastMetrics = metrics
			})
		}
	}()

	restartChan := make(chan bool, 1)
	reportChan := make(chan bool, 1)

	pauseButton := tview.NewButton("Pause")
	restartButton := tview.NewButton("Restart")
	generateReportButton := tview.NewButton("Generate Report")
	quitButton := tview.NewButton("Quit")

	menuBar := tview.NewFlex().AddItem(pauseButton, 0, 1, true).
		AddItem(restartButton, 0, 1, true).
		AddItem(generateReportButton, 0, 1, true).
		AddItem(quitButton, 0, 1, true)

	pauseButton.SetSelectedFunc(func() {
		go func() {
			isPaused := state.TogglePause()
			app.QueueUpdateDraw(func() {
				if isPaused {
					pauseButton.SetLabel("Resume")
					fmt.Fprintf(logView, "[yellow]Monitoring paused.\n")
				} else {
					pauseButton.SetLabel("Pause")
					fmt.Fprintf(logView, "[green]Monitoring resumed.\n")
				}
				app.Draw()
			})
		}()
	})

	restartButton.SetSelectedFunc(func() {
		go func() {
			select {
			case restartChan <- true:
			default:
			}
		}()
	})

	generateReportButton.SetSelectedFunc(func() {
		go func() {
			select {
			case reportChan <- true:
			default:
			}
		}()
	})

	quitButton.SetSelectedFunc(func() {
		app.Stop()
	})

	mainPage.AddItem(header, 1, 1, false)
	mainPage.AddItem(menuBar, 1, 1, false)
	mainPage.AddItem(grid, 0, 3, true)
	mainPage.AddItem(issuesPanel, 4, 1, false)
	mainPage.AddItem(recommendationsPanel, 5, 1, false)
	mainPage.AddItem(logView, 6, 1, true)

	// Handle Restart Logic via Engine/State
	go func() {
		for {
			select {
			case <-restartChan:
				fmt.Fprintf(logView, "[yellow]Restart requested...[white]\n")
				state.InitSession()
				// Note: In a real app, we might want to restart the engine itself
			case <-reportChan:
				app.QueueUpdateDraw(func() {
					report := "[cyan]REPORT:\n"
					if lastMetrics.issueDetected {
						report += fmt.Sprintf("- Issue detected: %s\n", lastMetrics.issueSummary)
					} else {
						report += "- No issue detected. Network appears stable.\n"
					}
					report += fmt.Sprintf("- Target: %s\n", monitor.GetTarget())
					report += fmt.Sprintf("- Latency: %v\n", lastMetrics.latency)
					report += fmt.Sprintf("- Jitter: %.1f ms\n", lastMetrics.jitter)
					report += fmt.Sprintf("- Packet loss: %.1f%%\n", lastMetrics.loss)
					report += fmt.Sprintf("- TCP latency (443): %v\n", lastMetrics.tcpLatency)
					if lastMetrics.httpTTFB > 0 {
						report += fmt.Sprintf("- HTTP TTFB: %v\n", lastMetrics.httpTTFB)
					} else {
						report += "- HTTP TTFB: N/A\n"
					}
					report += fmt.Sprintf("- Download: %.2f Mbps\n", lastMetrics.downloadSpeed)
					report += fmt.Sprintf("- Upload: %.2f Mbps\n", lastMetrics.uploadSpeed)
					report += fmt.Sprintf("- Baseline DL: %.2f Mbps, UL: %.2f Mbps\n", monitor.GetBaselineDown(), monitor.GetBaselineUp())
					if lastMetrics.zscalerLatency > 0 {
						report += fmt.Sprintf("- Zscaler latency: %v\n", lastMetrics.zscalerLatency)
					} else {
						report += "- Zscaler latency: N/A\n"
					}
					report += fmt.Sprintf("- DNS lookup: %v\n", lastMetrics.dnsTime)
					if len(lastMetrics.errors) > 0 {
						report += "- Recent errors:\n"
						for i, errStr := range lastMetrics.errors {
							if i >= 5 {
								break
							}
							report += fmt.Sprintf("  %d. %s\n", i+1, errStr)
						}
					} else {
						report += "- Recent errors: none\n"
					}
					report += "[white]"
					fmt.Fprint(logView, report)
				})
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		engine.Stop()
		app.Stop()
	}()

	if err := app.SetRoot(mainPage, true).EnableMouse(true).SetFocus(pauseButton); err != nil {
		fmt.Println(err)
	}
	if err := app.Run(); err != nil {
		fmt.Println(err)
	}
}
