# Network-Diagnostic-tools

High-performance network utility suite for modern infrastructure.

## 🚀 Architecture Improvements (Latest)

We have implemented advanced concurrency patterns to maximize throughput and minimize latency:

### 1. Optimized ICMP Connection Pooling (`network/pinger_pool.go`)
* **Mechanism**: Transitioned from slice-based management to a **Buffered Channel** architecture.
* **Performance**: Achieved $O(1)$ complexity for connection retrieval and return operations.
* **Efficiency**: Implemented a background cleanup goroutine to manage idle connections without blocking the critical path.

### 2. Sharded DNS Cache (`utils/dns_cache.go`)
* **Mechanism**: Implemented **Lock Striping (Map Shing)** across 16 independent shards.
* **Performance**: Drastically reduced lock contention in high-concurrency environments by using FNV-1a hash distribution.
* **Reliability**: Provides thread-safe, highly scalable DNS resolution with TTL support.

## 🛠️ Core Components
- `cmd/`: Entry points for the toolset.
- `network/`: High-performance ICMP pooling logic.
- `utils/`: Optimized utility libraries (DNS Caching).
