# Network Diagnostics Tool Suite

A cross-platform network diagnostic tool suite that performs real ICMP ping, DNS resolution measurement, TCP/443 connectivity testing, and HTTPS request verification with TLS inspection and speed tests. The suite is available in three implementations: a lightweight Python CLI, a production-grade Go CLI, and a high-performance Rust engine. All three versions now include a built-in **Self-Hosted Web Dashboard**.

## Installation & Build

### Python (No Build Required)
The Python version uses 100% standard library modules (no external dependencies).
```bash
git clone https://github.com/TchomboW/Network-Diagnostic-tools.git
cd Network-Diagnostic-tools
# Ready to use (Python 3.6+)
```

### Go
Requires Go 1.21+. Runs natively on Linux, macOS, and Windows (WSL).
```bash
cd Network-Diagnostic-tools
make build
# Binary is at: ./bin/network_tool
```

### Rust
Requires Cargo (Rust toolchain). Highly optimized release builds.
```bash
cd Network-Diagnostic-tools/rust_engine
cargo build --release
# Binary is at: ./target/release/network_diagnostic_engine
```

## Usage

### 1. CLI Mode (Terminal Output)

**Python:**
```bash
python network-diagnostic.py google.com --interval 5 --count 3
```

**Go:**
```bash
./bin/network_tool --target google.com
```

**Rust:**
```bash
./target/release/network_diagnostic_engine 8.8.8.8
```

### 2. Web Dashboard Mode (Recommended)
Start a local self-hosted web dashboard to monitor network metrics in real-time with interactive charts. This feature is available in **all three** languages.

```bash
# Using Python
python network-diagnostic.py --web google.com

# Using Go
./bin/network_tool --web --target google.com

# Using Rust
./target/release/network_diagnostic_engine --web google.com
```

This will start a background diagnostic loop and launch a local HTTP server (default port: `8080`). Simply open your browser to `http://localhost:8080` to view the live dashboard. No external cloud hosting is required; all data remains on your local machine.

**Custom Port:**
```bash
# Change the dashboard port to 9090
python network-diagnostic.py --web --port 9090 google.com
./bin/network_tool --web --port 9090 --target google.com
./target/release/network_diagnostic_engine --web --port 9090 google.com
```

## Features

| Feature | Python | Go | Rust |
| :--- | :---: | :---: | :---: |
| Real ICMP Echo Requests (RFC 792) | ✅ | ✅ | ✅ |
| DNS Resolution Timing | ✅ | ✅ | ✅ |
| TCP/443 Connectivity Testing | ✅ | ✅ | ✅ |
| HTTPS Request & TLS Inspection | ✅ | ✅ | ✅ |
| **Real-time Web Dashboard** | ✅ | ✅ | ✅ |
| Zero External Dependencies | ✅ | ✅ | ✅ |

## Architecture

The tool suite follows a modular architecture with clear separation of concerns:

```
Network-Diagnostic-tools/
├── network-diagnostic.py      # Python CLI (with stdlib Web Dashboard)
├── cmd/                       # Go command-line interface
│   ├── main.go                # Main entry point: CLI + Web routing
│   └── web/                   # Embedded dashboard static assets
│       ├── static/            # CSS, JS (htmx, Chart.js)
│       └── templates/         # HTML templates
├── pkg/                       # Go packages
│   ├── icmp/                  # ICMP ping implementation
│   ├── dns/                   # DNS resolution testing
│   ├── http/                  # HTTP request testing
│   └── speed/                 # Speed testing
├── rust_engine/               # High-performance Rust engine
│   ├── Cargo.toml             # Rust dependencies
│   └── src/                   # Rust binary logic (CLI & Web)
└── docs/                      # Documentation
```

## Testing

**Python:**
```bash
python3 -c "import py_compile; py_compile.compile('network-diagnostic.py', doraise=True)"
```

**Go:**
```bash
go vet ./...
go test ./...
```

**Rust:**
```bash
cd rust_engine && cargo test
```

## License

Apache License 2.0. See `LICENSE` for details.
