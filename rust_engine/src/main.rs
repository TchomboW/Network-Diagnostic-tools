use std::env;
use std::net::IpAddr;

use network_diagnostic_engine::DiagnosticEngine;

#[tokio::main]
async fn main() {
    let args: Vec<String> = env::args().collect();
    let target = if args.len() > 1 {
        args[1].parse::<IpAddr>().unwrap_or_else(|_| {
            eprintln!("Usage: network_diagnostic_engine <IP_ADDRESS>");
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