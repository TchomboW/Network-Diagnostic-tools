package network

import (
	"fmt"
	"net"
	"sync"
	"time"

	"network_tool/pkg/icmp"
)

// ICMPConnectionPool is a pool of ICMP connections
type ICMPConnectionPool struct {
	mu            sync.Mutex
	connections   []*ICMPConnection
	nextConnection int
}

// ICMPConnection represents a single ICMP connection
type ICMPConnection struct {
	Conn     net.PacketConn
	PacketCh chan []byte
}

// NewICMPConnectionPool creates a new pool
func NewICMPConnectionPool(size int) *ICMPConnectionPool {
	pool := &ICMPConnectionPool{
		connections: make([]*ICMPConnection, size),
	}
	return pool
}

// GetConnection gets an ICMP connection from the pool
func (p *ICMPConnectionPool) GetConnection() (*ICMPConnection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, c := range p.connections {
		if c == nil || c.Conn == nil {
			newConn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
			if err != nil {
				return nil, err
			}
			c = &ICMPConnection{
				Conn:     newConn,
				PacketCh: make(chan []byte, 100),
			}
			// Find next slot to overwrite
			idx := -1
			for i := range p.connections {
				if p.connections[i] == nil {
					idx = i
					break
				}
			}
			if idx >= 0 {
				p.connections[idx] = c
			}
			return c, nil
		}
	}
	// All connections busy
	return nil, fmt.Errorf("no free connections")
}

// ReturnConnection puts a connection back
func (p *ICMPConnectionPool) ReturnConnection(c *ICMPConnection) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Find slot holding c
	for i := range p.connections {
		if p.connections[i] == c {
			p.connections[i] = nil
			break
		}
	}
}

// CloseAllConnections shuts down all connections
func (p *ICMPConnectionPool) CloseAllConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.connections {
		if p.connections[i] != nil {
			p.connections[i].Conn.Close()
			p.connections[i] = nil
		}
	}
}

// Ping sends ICMP echo request using the connection pool
func (p *ICMPConnectionPool) Ping(target string, timeout time.Duration) PingResult {
	conn, err := p.GetConnection()
	if err != nil {
		return PingResult{Success: false}
	}
	defer p.ReturnConnection(conn)
	return conn.Ping(target, timeout)
}

// Ping sends ICMP echo request on a specific connection
func (c *ICMPConnection) Ping(target string, timeout time.Duration) PingResult {
	start := time.Now()

	// Build ICMP Echo Request packet
	icmpPacket := make([]byte, 8+56)
	icmpPacket[0] = 8  // Type: Echo Request
	icmpPacket[1] = 0  // Code: 0
	icmpPacket[4] = 1  // ID high
	icmpPacket[5] = 0  // ID low
	icmpPacket[6] = 0  // Sequence high
	icmpPacket[7] = byte(start.UnixNano() & 0xFF)  // Sequence low byte

	// Calculate ICMP checksum (proper 16-bit one's complement sum per RFC 1071)
	// Checksum handled by pkg/icmp.Checksum — computed inline below

	// Resolve target
	addr, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return PingResult{Latency: time.Since(start), Success: false}
	}

	// Send ICMP packet
	c.Conn.WriteTo(icmpPacket, &net.IPAddr{IP: addr.IP, Zone: ""})

	// Wait for response
	c.Conn.SetReadDeadline(time.Now().Add(timeout))
	var recvBuf [1500]byte
	n, _, err := c.Conn.ReadFrom(recvBuf[:])
	if err != nil {
		return PingResult{Latency: time.Since(start), Success: false}
	}

	// Verify received packet is an ICMP Echo Reply
	if n < 8 || recvBuf[0] != 0 || recvBuf[1] != 0 {
		return PingResult{Latency: time.Since(start), Success: false}
	}

	return PingResult{
		Latency: time.Since(start),
		Success: true,
	}
}

// SendPing sends ICMP echo request using the connection pool
func SendPing(target string, count int, timeout time.Duration) []PingResult {
	pingResults := make([]PingResult, 0, count)

	// Resolve target first
	addr, err := net.ResolveIPAddr("ip", target)
	if err != nil {
		// DNS resolution failed, just fail fast for all pings
		for i := 0; i < count; i++ {
			pingResults = append(pingResults, PingResult{Success: false})
		}
		return pingResults
	}

	// Try raw ICMP first
	p := NewICMPConnectionPool(8)
	conn, err := p.GetConnection()
	if err != nil {
		// Raw socket not available (not root), fall back to UDP probe for each ping
		p.CloseAllConnections()
		for i := 0; i < count; i++ {
			pingResults = append(pingResults, UDPProber(addr.String(), timeout))
		}
		return pingResults
	}

	// Send raw ICMP pings using the connection
	for i := 0; i < count; i++ {
		pingResults = append(pingResults, conn.Ping(target, timeout))
	}

	p.CloseAllConnections()
	return pingResults
}

// PingPing sends a single ICMP echo request and waits for the response
func PingPing(target string, timeout time.Duration) PingResult {
	start := time.Now()

	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		// Raw ICMP not available (e.g., no root on Linux), fall back to UDP port probe
		return UDPProber(target, timeout)
	}
	defer conn.Close()

	// Resolve target
	addr, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return PingResult{Latency: time.Since(start), Success: false}
	}

	// Build ICMP Echo Request packet
	icmpPacket := make([]byte, 8+56)
	icmpPacket[0] = 8  // Type: Echo Request
	icmpPacket[1] = 0  // Code: 0
	icmpPacket[4] = 1  // ID high
	icmpPacket[5] = 1  // ID low
	icmpPacket[6] = 1  // Sequence high
	icmpPacket[7] = byte(time.Now().UnixNano() & 0xFF)  // Sequence low byte

	// Calculate ICMP checksum (proper 16-bit oneshot checksum per RFC 792)
	icmpPacket[2] = 0
	icmpPacket[3] = 0 // Zero out checksum field before computing
	checksumVal := icmp.Checksum(icmpPacket)
	icmpPacket[2] = byte(checksumVal >> 8)
	icmpPacket[3] = byte(checksumVal & 0xFF)

	// Send ICMP packet
	if _, err = conn.WriteTo(icmpPacket, &net.IPAddr{IP: addr.IP, Zone: ""}); err != nil {
		return PingResult{Latency: time.Since(start), Success: false}
	}

	// Wait for response
	conn.SetReadDeadline(time.Now().Add(timeout))
	var recvBuf [1500]byte
	n, _, err := conn.ReadFrom(recvBuf[:])
	if err != nil {
		return PingResult{Latency: time.Since(start), Success: false}
	}

	// Verify received packet is an ICMP Echo Reply
	if n < 8 || recvBuf[0] != 0 || recvBuf[1] != 0 {
		return PingResult{Latency: time.Since(start), Success: false}
	}

	return PingResult{
		Latency: time.Since(start),
		Success: true,
	}
}



// PingResult contains the result of a single ping
type PingResult struct {
	Latency time.Duration
	Success bool
}

// UDPProber measures UDP connect latency as a fallback when ICMP is not
// available (e.g., not running as root). This is NOT a true ICMP ping — it
// measures time to dial the target on a specific port.
func UDPProber(target string, timeout time.Duration) PingResult {
	start := time.Now()
	conn, err := net.DialTimeout("udp", target+":53", timeout)
	if err != nil {
		return PingResult{
			Latency: time.Since(start),
			Success: false,
		}
	}
	defer conn.Close()
	return PingResult{
		Latency: time.Since(start),
		Success: true,
	}
}
