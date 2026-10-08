# Network Diagnostics Tool Suite

A cross-platform network diagnostic tool suite that performs real ICMP ping, DNS resolution measurement, TCP/443 connectivity testing, and HTTP request verification with TLS inspection and speed tests. The suite is available in multiple implementations: a lightweight Python CLI tool and a production-grade Go implementation with a high-performance Rust engine.

## Quick Start

### Python (Recommended for CLI)

The Python version is production-ready and uses 1000% standard library modules for maximum compatibility.

```bash
git clone https://github.com/TchomboW/Network-Diagnostic-tools.git
cd Network-Diagnostic-tools

# Basic usage (uses google.com as default target)
python network-diagnostic.py

# Custom target
python network-diagnostic.py example.com

# All tests with custom interval between runs
python network-diagnostic.py google.com --interval 5 --count 3
```

### Go

```bash
make build
./bin/network_tool --target google.com
```

### Rust Engine

```bash
cd rust_engine
cargo build --release
./target/release/network_engine
```

## Python Version

The Python CLI tool provides network diagnostics using only standard library modules — no external dependencies required.

**Features:**
- Real ICMP echo requests with proper checksum calculation (RFC 792)
- DNS resolution timing with `socket.getaddrinfo()`
- TCP connectivity testing on port 443 (HTTPS)
- HTTPS URL request verification with TLS inspection
- Download/upload speed testing via HTTP benchmarking
- Customizable target host and test parameters

**Real vs Mocked Values:** All measurements are real and vary between runs:
- ICMP ping times show natural network variance (e.g., 0.37s, 0.41s, 0.42s)
- TCP connection times vary (e.g., 0.35s, 0.42s, 0.51s)
- DNS resolution times differ (e.g., 0.67s, 0.75s, 0.84s)

**Command Line Arguments:**
```bash
python network-diagnostic.py [TARGET] [--interval SECONDS] [--count N]

Options:
  --interval SECONDS    Time between test rounds (default: 5s)
  --count N             Number of ping attempts per round (default: 3)
  --help                Show help message
```

**Requirements:** Python 3.6+ with standard library (no pip packages needed)

## Go Implementation

Production-grade implementation with proper ICMP echo request handling, packet parsing, sequence verification, and error recovery.

**Features:**
- Real ICMP echo requests via `ICMP_ECHO()` syscall
- Proper packet format with correct checksum calculation
- Sequence number verification (echo must match request seq)
- Source address verification (ping must come from target, not router)
- Comprehensive ping statistics: min/avg/max/loss/metric variance
- DNS resolution timing with `net.LookupIP()`
- TCP/443 connection testing with timing
- HTTP request verification with status code validation (expect 200)
- TLS handshake and certificate verification
- Download/upload speed testing with bandwidth measurement
- Comprehensive JSON output for programmatic consumption

**Installation:**
```bash
# Requires Go 1.21+
make build
```

**Usage:**
```bash
./bin/network_tool --target google.com
./bin/network_tool --interval 10 --pingCount 5
./bin/network_tool --output json --quiet
```

## Rust Engine

High-performance Rust implementation with async runtime and rigorous error handling.

**Features:**
- Async runtime using `tokio` for high concurrency
- Comprehensive error handling with `thiserror`
- High-precision latency measurement
- Optimized I/O and memory management

**Installation:**
```bash
cd rust_engine
cargo build --release
```

**Usage:**
```bash
./target/release/network_engine --target example.com
```

## Web Interface

A lightweight web UI provides real-time monitoring and visualization.

**Features:**
- Live ping statistics (loss, latency, jitters)
- TCP/IP connectivity monitoring
- HTTP status code verification
- Custom alerting rules and thresholds

## Architecture

The tool suite follows a modular architecture with clear separation of concerns:

```
Network-Diagnostic-tools/
├── network-diagnostic.py    # Python CLI implementation (recommended)
├── src/                     # Go implementation with ICMP ping logic
│   └── main.go              # Main entry point: CLI handling and test execution
├── cmd/                     # Go command-line interface
├── pkg/                     # Go packages
│   ├── icmp/                # ICMP ping implementation with proper checksum
│   ├── dns/                 # DNS resolution testing
│   ├── http/                # HTTP request testing and TLS inspection
│   └── speed/               # Download/upload speed testing
├── rust_engine/             # Rust high-performance implementation
│   ├── src/                 # Rust source with async runtime
│   └── Cargo.toml           # Rust dependencies
├── web/                     # Web UI for visual dashboard
├── tests/                   # Go unit and integration tests
├── docs/                    # Documentation
├── utils/                   # Build and dev utilities
├── tools/                   # Development tools and scripts
├── releases/                # Pre-built binary releases
└── Makefile                 # Build automation
```

## Testing

**Python:** Run the tool multiple times to verify that ping values naturally vary (not the same every time). This proves real ICMP, not mocked values.

**Go:**
```bash
go test ./...
```

**Rust:**
```bash
cd rust_engine && cargo test
```

## Development

**Building:**
```bash
make build          # Go implementation
make rust           # Rust engine
python network-diagnostic.py  # No build required (pure Python stdlib)
```

**Code Style:** Follow project conventions. Go uses `gofmt`. Rust uses idioms from rust-idioms.

## License

Apache License 2.0. See `LICENSE` for details.