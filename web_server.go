package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
)

//go:embed web/templates
var templateFS embed.FS

//go:embed web/static
var staticFS embed.FS

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

// WebServer wraps the HTTP server and the metrics cache.
type WebServer struct {
	addr      string
	cache     *MetricsCache
	monitor   *NetworkMonitor
	state     *AppState
	engine    *MonitorEngine
	tmpl      *template.Template
	stopCh    chan struct{}
}

// NewWebServer creates a new web server instance.
func NewWebServer(addr string) *WebServer {
	monitor := NewNetworkMonitor()
	monitor.initializeBaselineSpeeds()
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
				m.latency, m.dnsTime, m.tcpLatency, m.httpTTFB, m.downloadSpeed, m.uploadSpeed, m.errors)
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
		m.latency, m.dnsTime, m.tcpLatency, m.httpTTFB, m.downloadSpeed, m.uploadSpeed, m.errors)
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
