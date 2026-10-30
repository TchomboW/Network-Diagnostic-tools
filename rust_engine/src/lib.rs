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
#[derive(Debug, Clone, PartialEq)]
pub struct ProbeResult {
    pub target: IpAddr,
    pub latency: Duration,
}

pub struct DiagnosticEngine {
    pub default_timeout: Duration,
}

impl DiagnosticEngine {
    pub fn new(default_timeout: Duration) -> Self {
        Self {
            default_timeout,
        }
    }

    pub fn check_health(&self) -> bool {
        true
    }

    /// Probes a single IP address
    pub async fn probe_latency(&self, target: IpAddr) -> Result<Duration, DiagnosticError> {
        ping::tokio::ping(target, self.default_timeout)
            .await
            .map(|reply| reply.rtt)
            .map_err(|e| DiagnosticError::PingFailed(format!("Ping to {} failed: {}", target, e)))
    }

    /// Probes many IP addresses concurrently
    pub async fn probe_batch(&self, targets: Vec<IpAddr>) -> Vec<Result<ProbeResult, DiagnosticError>> {
        let mut set = tokio::task::JoinSet::new();
        let timeout = self.default_timeout;

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

        let mut results = Vec::with_capacity(set.len());
        while let Some(res) = set.join_next().await {
            match res {
                Ok(probe_res) => results.push(probe_res),
                Err(_) => results.push(Err(DiagnosticError::InternalError)),
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

    #[tokio::test]
    async fn test_batch_concurrency() {
        let engine = DiagnosticEngine::new(Duration::from_secs(1));
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
