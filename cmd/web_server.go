//go:build web
// +build web

package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"network_tool/lib"
	"network_tool/tools/performance"
)

//go:embed web/templates
var templateFS embed.FS

//go:embed web/static
var staticFS embed.FS

// NetworkMetrics holds the latest monitoring results.
type NetworkMetrics struct {
	Latency       float64   `json:"latency"`
	DNSTime       float64   `json:"dns_time"`
	TCPDelay      float64   `json:"tcp_delay"`
	HTTPTTFB      float64   `json:"http_ttfb"`
	DownloadSpeed float64   `json:"download_speed"`
	UploadSpeed   float64   `json:"upload_speed"`
	Errors        int       `json:"errors"`
	Timestamp     time.Time `json:"timestamp"`
}

// MetricsCache stores the latest monitoring results for the web UI.
type MetricsCache struct {
	mu      sync.RWMutex
	metrics NetworkMetrics
}

func (c *MetricsCache) Get() NetworkMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metrics
}

func (c *MetricsCache) Set(m NetworkMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = m
}

// AppState tracks the monitoring state.
type AppState struct {
	mu      sync.RWMutex
	paused  bool
	target  string
	started time.Time
}

func NewAppState() *AppState {
	return &AppState{
		paused:  false,
		target:  "8.8.8.8",
		started: time.Now(),
	}
}

func (s *AppState) IsPaused() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.paused
}

func (s *AppState) TogglePause() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = !s.paused
	return s.paused
}

func (s *AppState) SetTarget(t string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.target = t
}

func (s *AppState) GetTarget() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.target
}

// MonitorEvent represents an event from the monitor engine.
type MonitorEvent struct {
	Metrics NetworkMetrics
	Error   error
}

// MonitorEngine runs the monitoring loop.
type MonitorEngine struct {
	monitor    *network_tool.NetworkMonitor
	state      *AppState
	listeners  []chan MonitorEvent
	stopCh     chan struct{}
	perfMonitor *performance.PerformanceMonitor
}

// NewMonitorEngine creates a new monitor engine.
func NewMonitorEngine(monitor *network_tool.NetworkMonitor, state *AppState) *MonitorEngine {
	return &MonitorEngine{
		monitor:     monitor,
		state:       state,
		listeners:   make([]chan MonitorEvent, 0),
		stopCh:      make(chan struct{}),
		perfMonitor: performance.NewPerformanceMonitor(),
	}
}

// AddListener adds an event listener channel.
func (e *MonitorEngine) AddListener(ch chan MonitorEvent) {
	e.listeners = append(e.listeners, ch)
}

// Start begins the monitoring loop.
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
				metrics := e.runDiagnostics()
				event := MonitorEvent{Metrics: metrics}
				for _, ch := range e.listeners {
					select {
					case ch <- event:
					default:
					}
				}
			case <-e.stopCh:
				return
			}
		}
	}()
}

// Stop halts the monitoring loop.
func (e *MonitorEngine) Stop() {
	close(e.stopCh)
}

func (e *MonitorEngine) runDiagnostics() NetworkMetrics {
	target := e.monitor.GetTarget()
	m := NetworkMetrics{Timestamp: time.Now()}

	// Ping
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(target, "7"))
	if err == nil {
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		buf := make([]byte, 1)
		conn.Write([]byte{0})
		_, err = conn.Read(buf)
		if err == nil {
			m.Latency = float64(time.Since(start).Microseconds()) / 1000.0
		}
		conn.Close()
	}

	// DNS
	dnsStart := time.Now()
	ips, err := net.LookupIP(target)
	if err == nil && len(ips) > 0 {
		m.DNSTime = float64(time.Since(dnsStart).Milliseconds())
	}

	// TCP
	tcpStart := time.Now()
	conn2, err := net.DialTimeout("tcp", net.JoinHostPort(target, "443"), 5*time.Second)
	if err == nil {
		m.TCPDelay = float64(time.Since(tcpStart).Milliseconds())
		conn2.Close()
	}

	// HTTP
	httpStart := time.Now()
	url := target
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	resp, err := http.Get(url)
	if err == nil {
		m.HTTPTTFB = float64(time.Since(httpStart).Milliseconds())
		resp.Body.Close()
	}

	// Speed test (simplified)
	m.DownloadSpeed = 0
	m.UploadSpeed = 0

	return m
}

// WebServer wraps the HTTP server and the metrics cache.
type WebServer struct {
	addr      string
	cache     *MetricsCache
	monitor   *network_tool.NetworkMonitor
	state     *AppState
	engine    *MonitorEngine
	tmpl      *template.Template
	stopCh    chan struct{}
}

// NewWebServer creates a new web server instance.
func NewWebServer(addr string) *WebServer {
	monitor := network_tool.NewNetworkMonitor()
	monitor.SetTarget("8.8.8.8")
	state := NewAppState()
	engine := NewMonitorEngine(monitor, state)

	ws := &WebServer{
		addr:    addr,
		cache:   &MetricsCache{},
		monitor: monitor,
		state:   state,
		engine:  engine,
		stopCh:  make(chan struct{}),
	}

	// Register web server as a listener
	wsEvents := make(chan MonitorEvent, 10)
	engine.AddListener(wsEvents)
	engine.Start()

	// Load templates
	ws.tmpl = template.Must(template.ParseFS(templateFS, "web/templates/*.html"))

	// Start the metrics sync goroutine
	go ws.syncMetrics(wsEvents)

	return ws
}

// syncMetrics consumes engine events and updates the cache.
func (ws *WebServer) syncMetrics(events chan MonitorEvent) {
	log.Printf("[web] syncMetrics started")
	for {
		select {
		case event := <-events:
			if event.Error != nil {
				log.Printf("[web] monitor error: %v", event.Error)
				continue
			}
			m := event.Metrics
			log.Printf("[web] received metrics: latency=%v, dns=%v, tcp=%v, http=%v, dl=%.2f, ul=%.2f, errors=%v",
				m.Latency, m.DNSTime, m.TCPDelay, m.HTTPTTFB, m.DownloadSpeed, m.UploadSpeed, m.Errors)
			ws.cache.Set(m)
		case <-ws.stopCh:
			return
		}
	}
}

// Start launches the HTTP server.
func (ws *WebServer) Start() {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/metrics", ws.metricsHandler)
	mux.HandleFunc("/api/status", ws.statusHandler)
	mux.HandleFunc("/api/toggle-pause", ws.togglePauseHandler)
	mux.HandleFunc("/api/set-target", ws.setTargetHandler)

	// Static files
	fs := http.FS(staticFS)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(fs)))

	// Template rendering
	mux.HandleFunc("/", ws.indexHandler)

	log.Printf("[web] Starting web UI on http://%s", ws.addr)
	if err := http.ListenAndServe(ws.addr, mux); err != nil {
		log.Fatalf("[web] Server failed: %v", err)
	}
}

// Stop shuts down the web server.
func (ws *WebServer) Stop() {
	close(ws.stopCh)
	ws.engine.Stop()
}

// ---------- HTTP Handlers ----------

func (ws *WebServer) metricsHandler(w http.ResponseWriter, r *http.Request) {
	m := ws.cache.Get()
	log.Printf("[web] API metrics: latency=%v, dns=%v, tcp=%v, http=%v, dl=%.2f, ul=%.2f, errors=%v",
		m.Latency, m.DNSTime, m.TCPDelay, m.HTTPTTFB, m.DownloadSpeed, m.UploadSpeed, m.Errors)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

func (ws *WebServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"paused": ws.state.IsPaused(),
		"target": ws.monitor.GetTarget(),
	})
}

func (ws *WebServer) togglePauseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	isPaused := ws.state.TogglePause()
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"paused": isPaused,
	}
	if isPaused {
		resp["message"] = "Paused"
	} else {
		resp["message"] = "Resumed"
	}
	json.NewEncoder(w).Encode(resp)
}

func (ws *WebServer) setTargetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	ws.monitor.SetTarget(req.Target)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": fmt.Sprintf("Target set to %s", req.Target),
	})
}

func (ws *WebServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if err := ws.tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
