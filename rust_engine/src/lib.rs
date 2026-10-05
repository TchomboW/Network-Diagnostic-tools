use std::net::IpAddr;
use std::time::Duration;
use thiserror::Error;

#[derive(Error, Debug, PartialEq)]
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

pub struct DiagnosticEngine {
    target: String,
    default_timeout: Duration,
}

impl DiagnosticEngine {
    pub fn new(target: &str) -> Self {
        Self {
            target: target.to_string(),
            default_timeout: Duration::from_secs(3),
        }
    }

    pub async fn probe_latency(&self) -> Result<Duration, DiagnosticError> {
        if self.target.is_empty() {
            return Err(DiagnosticError::InvalidTarget(self.target.clone()));
        }

        // Parse target as IP address
        let ip: IpAddr = self
            .target
            .parse()
            .map_err(|_| DiagnosticError::InvalidTarget(self.target.clone()))?;

        // Real ping using the ping crate (async)
        let result = ping::tokio::ping(ip, self.default_timeout)
            .await
            .map_err(|e| {
                DiagnosticError::PingFailed(format!(
                    "Ping to {} failed: {}",
                    self.target, e
                ))
            })?;

        Ok(result.rtt)
    }

    pub fn check_health(&self) -> bool {
        !self.target.is_empty()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_engine_initialization() {
        let engine = DiagnosticEngine::new("8.8.8.8");
        assert!(engine.check_health());
    }

    #[tokio::test]
    async fn test_latency_measurement() {
        let engine = DiagnosticEngine::new("127.0.0.1");
        let result = engine.probe_latency().await.unwrap();
        assert!(result.as_micros() > 0);
    }

    #[tokio::test]
    async fn test_invalid_target() {
        let engine = DiagnosticEngine::new("");
        let result = engine.probe_latency().await;
        assert!(matches!(result, Err(DiagnosticError::InvalidTarget(_))));
    }
}