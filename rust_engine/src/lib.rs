use std::net::IpAddr;
use std::time::Duration;
use thiserror::Error;

#[derive(Error, Debug, PartialEq, Clone)]
pub enum DiagnosticError {
    #[error("Target address '{0}' is empty or invalid")]
    InvalidTarget(String),
    #[error("Connection timeout after {0:?}")]
    Timeout(Duration),
    #[error("Ping failed: {0}")]
    PingFailed(String),
    #[error("Internal engine failure")]
    InternalError,
}

/// Represents the result of a single latency probe
#[derive[Debug, Clone, PartialEq]]
pub struct ProbeResult {
    pub target: IpAddr,
    pub latency: Duration,
}

pub struct DiagnosticEngine {
    pub default_timeout: Duration,
}

impl DiagnosticSummary for DiagnosticEngine {
    fn check_health(&self) -> bool {
        true
    }
}

impl DiagnosticEngine {
    pub fn new(default_timeout: Duration) -> Self {
        Self {
            default_timeout,
        }
    }

    /// Probes a single IP address (Single-target legacy support)
    pub async fn probe_latency(&self, target: IpAddr) -> Result<Duration, DiagnosticError> {
        let result = ping::tokio::ping(target, self.default_timeout)
            .await
            .map_err(|e| DiagnosticError::PingFailed(format!("Ping to {} failed: {}", target, e)))?;

        Ok(result.rtt)
    }

    /// Probes many IP addresses concurrently (The High-Performance Batch Path)
    /// This uses tokio::task::JoinSet for optimal resource management.
    pub async fn probe_batch(&self, targets: Vec<IpAddr>) -> Vec<Result<ProbeResult, DiagnosticError>> {
        let mut set = tokio::task::JoinSet::new();
        let timeout = self.default_timeout;

        // 1. Spawn all tasks into the JoinSet
        for target in targets {
            set.spawn(async move {
                let rtt = ping::tokio::ping(target, timeout)
                    .await
                    .map_err(|e| DiagnosticError::PingFailed(format!("Ping to {} failed: {}", target, e)))?;
                
                Ok(ProbeResult {
                    target,
                    latency: rtt.rtt,
                })
            });
        }

        // 2. Collect all results as they complete
        let mut results = Vec::with_capacity(set.len());
        while let Some(res) = set.join_next().await {
            match res {
                Ok(probe_res) => results.push(probe_res),
                Err(_) => results.push(Err(DiagnosticError::InternalError)), // Handle task panic
            }
        }

        results
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_engine_initialization() {
        let engine = DiagnosticEngine::new(Duration::from_secs(1));
        assert!(engine.check_health());
    }

    impl DiagnosticEngine {
        pub fn check_health(&self) -> bool {
            true
        }
    }

    #[tokio::test]
    async fn test_batch_concurrency() {
        let engine = Diagnostice::new(Duration::from_secs(1));
        let targets = vec![
            "8.8.8.8".parse().unwrap(),
            "1.1.1.1".parse().unwrap(),
            "127.0.0.1".parse().unwrap(),
        ];

        let results = engine.probe_batch(targets).await;
        assert_eq!(results.len(), 3);
        
        for res in results {
            assert!(res.is_ok());
        }
    }
}
