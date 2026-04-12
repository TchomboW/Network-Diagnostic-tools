# Network Monitoring Tool

A comprehensive network monitoring tool built in Go with a terminal user interface (TUI). This tool performs real-time network diagnostics including ICMP ping, speed tests, DNS resolution, TCP latency, and HTTP time-to-first-byte (TTFB) measurements.

## Features

- **Real ICMP Ping**: Measures latency, packet loss, and jitter with multiple ping attempts
- **Speed Tests**: Download and upload speed measurements using Cloudflare and HTTPBin
- **DNS Resolution**: Tests DNS lookup times using default resolvers (8.8.8.8 and 1.1.1.1)
- **TCP Latency**: Measures connection time to TCP port 443
- **HTTP TTFB**: Time-to-first-byte for HTTPS requests (for hostnames only)
- **Zscaler Support**: Optional Zscaler latency test for systems with Zscaler agent installed
- **Interactive TUI**: Real-time monitoring dashboard with pause/resume functionality
- **Issue Detection**: Automatic detection of network issues with recommendations

## Requirements

- Go 1.19 or later
- Linux, macOS, or Windows
- ICMP privileges (may require running as root/admin on some systems)

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/networktools.git
   cd networktools
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Build the application:
   ```bash
   go build
   ```

## Usage

Run the application:
```bash
./network_tool
```

### Controls
- **Pause/Resume**: Toggle monitoring on/off
- **Restart**: Reset the monitoring session
- **Generate Report**: Create a summary report of current network status
- **Quit**: Exit the application

## Architecture

- **NetworkMonitor**: Core monitoring logic and state management
- **MonitorEngine**: Event-driven monitoring loop with configurable intervals
- **RealPinger**: ICMP-based ping implementation with loss and jitter calculation
- **TUI Components**: Terminal-based user interface using tview library

## Dependencies

- `github.com/rivo/tview`: Terminal user interface
- `golang.org/x/net/icmp`: ICMP protocol support
- `golang.org/x/net/ipv4`: IPv4 utilities

## License

MIT License - see LICENSE file for details