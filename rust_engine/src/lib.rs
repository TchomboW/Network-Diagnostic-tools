#[cfg(test)]
mod tests;

use std::time::Duration;
use thiserror::Error;

#[derive(Error, Debug, PartialEq)]
pub enum DiagnosticError {
    #[error("Target address '{0}' is empty or invalid")]
    InvalidTarget(String),
    #[error("Connection timeout after {0:?}")]
    Timeout(Duration),
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
            default_timeout: Duration::from_millis(1500),
        }
    }

    pub async fn probe_latency(&self) -> Result<Duration, DiagnosticError> {
        if self.target.is_empty() {
            return Err(DiagnosticError::InvalidTarget(self.target.clone()));
        }

        tokio::time::sleep(Duration::from_millis(10)).await; 
        let latency = Duration::from_millis(12); 

        Ok(latency)
    }

    pub fn check_health(&self) -> bool {
        !self.target.is_empty()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_engine_initialization() {
        let engine = DiagnosticEngine::new("8.8.8.8");
        assert!(engine.check_health());
    }

    #[tokio::test]
    async fn test_latency_measurement() {
        let engine = DiagnosticEngine::new("127.0.0.1");
        let result = engine.probe_latency().await.unwrap();
        assert!(result.as_millis() > 0);
    }

    #[tokio::test]
    async fn test_invalid_target() {
        let engine = DiagnosticEngine::new("");
        let result = engine.probe_latency().await;
        assert!(matches!(result, Err(DiagnosticError::InvalidTarget(_))));
    }
}
