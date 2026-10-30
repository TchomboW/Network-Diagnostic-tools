use std::env;
use std::net::{IpAddr, SocketAddr};
use std::sync::Arc;

use axum::{
    extract::State,
    http::StatusCode,
    routing::{get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};
use tokio::sync::RwLock;

use network_diagnostic_engine::DiagnosticEngine;

#[derive(Serialize, Deserialize, Clone)]
pub struct WebMetrics {
    pub latency_ms: f64,
    pub target: String,
    pub issue_detected: bool,
    pub issue_summary: String,
}

#[derive(Clone)]
pub struct AppState {
    pub metrics: Arc<RwLock<WebMetrics>>,
    pub engine: DiagnosticEngine,
}

#[derive(Deserialize)]
struct SetTargetRequest {
    target: String,
}

#[derive(Deserialize)]
struct TogglePauseRequest {
    #[allow(dead_code)]
    pause: bool,
}

async fn get_metrics(State(state): State<AppState>) -> Json<WebMetrics> {
    let metrics = state.metrics.read().await;
    Json((*metrics).clone())
}

async fn set_target(
    State(state): State<AppState>,
    Json(req): Json<SetTargetRequest>,
) -> StatusCode {
    let target = match req.target.parse::<IpAddr>() {
        Ok(ip) => ip,
        Err(_) => return StatusCode::BAD_REQUEST,
    };

    // Update target by storing it in metrics
    let mut metrics = state.metrics.write().await;
    metrics.target = req.target;

    // Run a quick probe to update latency
    match state.engine.probe_latency(target).await {
        Ok(rtt) => {
            metrics.latency_ms = rtt.as_secs_f64() * 1000.0;
            metrics.issue_detected = false;
            metrics.issue_summary.clear();
        }
        Err(e) => {
            metrics.issue_detected = true;
            metrics.issue_summary = e.to_string();
        }
    }

    StatusCode::OK
}

async fn toggle_pause(
    State(state): State<AppState>,
    Json(req): Json<TogglePauseRequest>,
) -> StatusCode {
    let mut metrics = state.metrics.write().await;
    metrics.issue_detected = req.pause;
    if req.pause {
        metrics.issue_summary = "Paused by user".to_string();
    }
    StatusCode::OK
}

// Background task that periodically updates metrics
async fn diagnostics_loop(metrics: Arc<RwLock<WebMetrics>>, engine: DiagnosticEngine) {
    let mut interval = tokio::time::interval(tokio::time::Duration::from_secs(5));
    loop {
        interval.tick().await;
        let target = {
            let m = metrics.read().await;
            m.target.clone()
        };

        if let Ok(ip) = target.parse::<IpAddr>() {
            match engine.probe_latency(ip).await {
                Ok(rtt) => {
                    let mut m = metrics.write().await;
                    m.latency_ms = rtt.as_secs_f64() * 1000.0;
                    m.issue_detected = false;
                    m.issue_summary.clear();
                }
                Err(e) => {
                    let mut m = metrics.write().await;
                    m.issue_detected = true;
                    m.issue_summary = e.to_string();
                }
            }
        }
    }
}

#[tokio::main]
async fn main() {
    let args: Vec<String> = env::args().collect();
    let web_mode = args.iter().any(|a| a == "--web");

    if web_mode {
        let port = args
            .iter()
            .position(|a| a == "--port")
            .and_then(|i| args.get(i + 1))
            .and_then(|p| p.parse::<u16>().ok())
            .unwrap_or(8080);

        let target: IpAddr = "8.8.8.8".parse().unwrap();
        let engine = DiagnosticEngine::new(std::time::Duration::from_secs(5));

        let initial_metrics = WebMetrics {
            latency_ms: 0.0,
            target: target.to_string(),
            issue_detected: false,
            issue_summary: String::new(),
        };

        let state = AppState {
            metrics: Arc::new(RwLock::new(initial_metrics)),
            engine: engine.clone(),
        };

        // Start background diagnostics loop
        let loop_metrics = state.metrics.clone();
        let loop_engine = state.engine.clone();
        tokio::spawn(async move {
            diagnostics_loop(loop_metrics, loop_engine).await;
        });

        let address = SocketAddr::from(([0, 0, 0, 0], port));
        println!("Web dashboard starting on {}", address);
        println!("Open http://localhost:{} in your browser", port);

        let app = Router::new()
            .route("/api/metrics", get(get_metrics))
            .route("/api/set-target", post(set_target))
            .route("/api/toggle-pause", post(toggle_pause))
            .with_state(state);

        let listener = tokio::net::TcpListener::bind(&address).await.unwrap();
        axum::serve(listener, app).await.unwrap();
    } else {
        // CLI mode
        let target = if args.len() > 1 {
            args[1]
                .parse::<IpAddr>()
                .unwrap_or_else(|_| {
                    eprintln!("Usage: network_diagnostic_engine <IP_ADDRESS> [--web]");
                    std::process::exit(1);
                })
        } else {
            "8.8.8.8".parse().unwrap()
        };

        let engine = DiagnosticEngine::new(std::time::Duration::from_secs(5));
        println!("Pinging {} with 5 second timeout...", target);
        match engine.probe_latency(target).await {
            Ok(rtt) => println!("Success! RTT: {:?}", rtt),
            Err(e) => eprintln!("Ping failed: {}", e),
        }
    }
}
