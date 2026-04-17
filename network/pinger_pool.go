package network

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// IdleTimeout Constants for connection pool management
const (
	DefaultMaxIdleTime  = time.Minute           // Default max idle time for connections
	ReadDeadlineTimeout = 10 * time.Millisecond // Timeout for checking if connection is alive
	MinAllowedIdleTime  = time.Second           // Minimum allowed idle timeout
	MaxAllowedIdleTime  = time.Hour             // Maximum allowed idle timeout
)

// ICMPConnection represents a single ICMP ping connection that can be reused
type ICMPConnection struct {
	ID       int
	Conn     net.PacketConn // ICMP socket for ping operations
	LastUsed time.Time      // Timestamp of last usage for idle detection
}

// NewICMPConnection creates a new ICMP connection wrapper with the given socket and ID
func NewICMPConnection(conn net.PacketConn, id int) *ICMPConnection {
	if conn == nil {
		return nil
	}
	return &ICMPConnection{
		ID:       id,
		Conn:     conn,
		LastUsed: time.Now(),
	}
}

// Close safely closes the underlying ICMP connection socket
func (ic *ICMPConnection) Close() error {
	if ic.Conn != nil {
		return ic.Conn.Close()
	}
	return fmt.Errorf("connection already closed")
}

// ICMPConnectionPool manages multiple ICMP connections for efficient ping operations with thread-safe access
type ICMPConnectionPool struct {
	mu          sync.RWMutex      // Thread-safe access to pool operations
	connections []*ICMPConnection // Slice of available/pooled connections
	maxSize     int               // Maximum number of connections in pool (0 means unlimited)
	maxIdleTime time.Duration     // How long a connection can sit idle before being closed
	nextID      int               // Counter for generating unique connection IDs
}

// NewICMPConnectionPool creates a new ICMP connection pool with specified configuration
// maxSize: Maximum number of connections. 0 means unlimited (but will auto-cleanup idle).
// maxIdleTime: How long an idle connection can sit before being closed. 0 or negative uses default of 1 minute.
func NewICMPConnectionPool(maxSize int, maxIdleTime time.Duration) *ICMPConnectionPool {
	// Validate and set defaults
	if maxSize < 0 {
		maxSize = -1 // Negative means unlimited (but will cleanup idle connections)
	}

	if maxSize == 0 {
		maxSize = 1 // Minimum of 1 to allow at least one connection
	}

	if maxIdleTime <= 0 || maxIdleTime > MaxAllowedIdleTime {
		maxIdleTime = DefaultMaxIdleTime // Use defined default for idle timeout
	}

	return &ICMPConnectionPool{
		mu:          sync.RWMutex{},
		connections: make([]*ICMPConnection, 0),
		maxSize:     maxSize,
		maxIdleTime: maxIdleTime,
		nextID:      1, // Start from 1 for better readability
	}
}

// GetConnection retrieves an available connection from the pool or creates a new one if under max size
// Returns error if pool is exhausted and cannot create more connections
func (p *ICMPConnectionPool) GetConnection() (*ICMPConnection, error) {
	// Check if pool itself is properly initialized
	if p == nil {
		return nil, errors.New("connection pool not initialized")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if connections slice exists before accessing it
	if p.connections == nil {
		return nil, errors.New("no connections available in pool")
	}

	// Now it's safe to access connections slice and check for idle connections
	for _, conn := range p.connections {
		if conn != nil && time.Since(conn.LastUsed) < p.maxIdleTime && !isConnClosed(conn.Conn) {
			conn.LastUsed = time.Now() // Mark as in-use
			return conn, nil
		} else if conn != nil && isConnClosed(conn.Conn) {
			// Remove expired/closed connection from pool
			fmt.Printf("Removing closed connection ID: %d\n", conn.ID)
		}
	}

	// Clean up any closed connections first
	p.cleanupClosedConnections()

	// Try to create a new connection if under max size or if unlimited
	if p.maxSize <= 0 || len(p.connections) < p.maxSize {
		conn, err := p.createNewConnectionLocked()
		if err == nil && conn != nil {
			p.connections = append(p.connections, conn)
			return conn, nil
		} else if err != nil {
			fmt.Printf("Failed to create new connection: %v\n", err)
		}
	}

	// Pool exhausted - return error after attempting cleanup
	p.cleanupClosedConnections()
	if p.maxSize > 0 && len(p.connections) >= p.maxSize {
		return nil, fmt.Errorf("connection pool exhausted")
	}

	// Try one more time if we have room now
	if p.maxSize <= 0 || len(p.connections) < p.maxSize {
		conn, err := p.createNewConnectionLocked()
		if err == nil && conn != nil {
			p.connections = append(p.connections, conn)
			return conn, nil
		}
	}

	return nil, fmt.Errorf("connection pool exhausted and no connections available")
}

// ReturnConnection marks a connection as available again for reuse in the pool
func (p *ICMPConnectionPool) ReturnConnection(conn *ICMPConnection) {
	if conn == nil || conn.Conn == nil {
		fmt.Printf("Invalid connection returned, closing\n")
		conn.Close()
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	conn.LastUsed = time.Now() // Mark as recently used and available

	// Check if this connection is already in the pool to avoid duplicates
	for _, existing := range p.connections {
		if existing.ID == conn.ID && existing.Conn != nil {
			return // Already exists, just update last used time
		}
	}

	// Remove any closed connections first
	p.cleanupClosedConnections()

	// Add back to pool if not present and under max size (or unlimited)
	if p.maxSize <= 0 || len(p.connections) < p.maxSize {
		p.connections = append(p.connections, conn)
	} else {
		// Pool is full, remove the oldest connection first
		fmt.Printf("Pool full for connection ID: %d, removing oldest\n", conn.ID)
		if len(p.connections) > 0 {
			oldestConn := p.connections[0]
			p.connections = append(p.connections[:0], p.connections[1:]...)
			oldestConn.Close() // Clean up the removed connection
		}
		p.connections = append(p.connections, conn)
	}
}

// SendPing uses the provided connection to send a ping and measure latency
func (p *ICMPConnectionPool) SendPing(conn *ICMPConnection, target string) (time.Duration, error) {
	if conn == nil || conn.Conn == nil {
		return 0, fmt.Errorf("invalid connection")
	}

	if target == "" {
		return 0, fmt.Errorf("target cannot be empty")
	}

	// Validate target format - check if it's a valid IP or localhost
	validTarget := isValidIPAddress(target) || target == "localhost"
	if !validTarget {
		return 0, fmt.Errorf("invalid target format: %v (must be a valid IP address or localhost)", target)
	}

	// Use the provided connection to send ping and measure latency
	startTime := time.Now()

	err := p.performPing(conn.Conn, target)
	duration := time.Since(startTime)

	if err != nil {
		return duration, fmt.Errorf("ping failed: %v", err)
	}

	return duration, nil
}

// CloseAllConnections safely closes all connections in the pool and clears them
func (p *ICMPConnectionPool) CloseAllConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, conn := range p.connections {
		conn.Close()
	}

	p.connections = make([]*ICMPConnection, 0) // Clear the connections slice
}

// createNewConnectionLocked creates a new ICMP connection (must be called with lock held)
func (p *ICMPConnectionPool) createNewConnectionLocked() (*ICMPConnection, error) {
	// Create UDP socket for ICMP over IPv4 or IPv6 using net.ListenPacket
	conn, err := net.ListenPacket("ip:icmp", "0.0.0.0") // Use raw IP ICMP protocol
	if err != nil {
		return nil, fmt.Errorf("failed to create ICMP connection: %v", err)
	}

	id := p.nextID
	p.nextID++ // Increment for next unique ID

	return NewICMPConnection(conn, id), nil
}

// performPing sends a ping request using the provided connection and target
func (p *ICMPConnectionPool) performPing(conn net.PacketConn, target string) error {
	// This is where actual ICMP packet sending logic would go
	// For now, we're simulating with UDP-based ping measurement

	return nil // Simulated success for now
}

// cleanupClosedConnections removes any closed connections from the pool
func (p *ICMPConnectionPool) cleanupClosedConnections() {
	activeConnections := make([]*ICMPConnection, 0, len(p.connections))

	for _, conn := range p.connections {
		if !isConnClosed(conn.Conn) {
			// Check if connection has expired based on idle time
			if time.Since(conn.LastUsed) >= p.maxIdleTime {
				fmt.Printf("Removing expired connection ID: %d\n", conn.ID)
				conn.Close()
			} else {
				activeConnections = append(activeConnections, conn)
			}
		} else {
			fmt.Printf("Removing closed connection ID: %d\n", conn.ID)
			conn.Close()
		}
	}

	p.connections = activeConnections
}

// isConnClosed checks if a packet connection has been closed
func isConnClosed(conn net.PacketConn) bool {
	if conn == nil {
		return true
	}

	// Try to set a read deadline to check if the connection is still alive
	conn.SetReadDeadline(time.Now().Add(ReadDeadlineTimeout))
	buf := make([]byte, 1)
	_, _, err := conn.ReadFrom(buf)

	if err != nil {
		return true // Connection is closed or unreachable
	}

	// Reset deadline
	conn.SetReadDeadline(time.Time{})
	return false
}

// isValidIPAddress checks if a string is a valid IPv4, IPv6 address, or localhost
func isValidIPAddress(ip string) bool {
	// Trim whitespace and check if empty
	ip = strings.TrimSpace(ip)
	if len(ip) == 0 {
		return false
	}

	// Check for localhost first (before trying IP parsing)
	if ip == "localhost" || ip == "127.0.0.1" {
		return true
	}

	// Try parsing as IPv4 first (simpler validation)
	if ipv4 := net.ParseIP(ip); ipv4 != nil && ipv4.To4() != nil {
		return true
	}

	// Check for IPv6 format more rigorously
	if strings.Contains(ip, ":") {
		// Must have proper structure: at least one colon followed by hex digits
		parts := strings.Split(ip, ":")

		if len(parts) < 2 {
			return false
		}

		// Each part should be valid hexadecimal (0-9, a-f, A-F) and length 1-4 chars for IPv6
		for _, part := range parts {
			if len(part) == 0 || len(part) > 4 {
				return false
			}

			// Check that all characters are valid hex digits
			for _, char := range part {
				isHexDigit := (char >= '0' && char <= '9') ||
					(char >= 'a' && char <= 'f') ||
					(char >= 'A' && char <= 'F')
				if !isHexDigit {
					return false
				}
			}
		}

		// Additional IPv6-specific validations:
		// 1. Cannot have more than 7 colons (IPv6 has max 8 groups)
		if strings.Count(ip, ":") > 7 {
			return false
		}

		// Check for :: abbreviation format - if present in the middle, must have parts on both sides
		if strings.Contains(ip, "::") && !strings.HasPrefix(ip, "::") && !strings.HasSuffix(ip, "::") {
			// Already validated format above, but ensure there are parts before and after
			return true
		}

		return true // Passed all IPv6 validation checks
	}

	return false
}

// GetPoolStats returns current pool statistics for monitoring/debugging
func (p *ICMPConnectionPool) GetPoolStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"total_connections": len(p.connections),
		"max_size":          p.maxSize,
		"max_idle_time":     p.maxIdleTime.String(),
	}
}
