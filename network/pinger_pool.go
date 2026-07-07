package network

import (
	"errors"
	"fmt"
	"net"
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

// ICMPConnection represents a single ICMIPing connection that can be reused
type ICMPConnection struct {
	ID       int
	Conn     net.PacketConn // ICMP socket for ping operations
	LastUsed time.Time      // Timestamp of last usage for idle detection
}

// NewICMPConnection creates a new ICNMP connection wrapper with the given socket and ID
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
	mu          sync.RWMutex      // Thread-safe access to pool metadata
	maxSize     int               // Maximum number of connections in pool (0 means unlimited)
	currentSize int               // Track total connections created (active + idle)
	maxIdleTime time.Duration     // How long a connection can sit idle before being closed
	pool        chan *ICMPConnection // Buffered channel of ready-to-use connections
	stopChan    chan struct{}     // Signal to stop the background cleanup loop
}

// NewICMPConnectionPool creates a new ICMP connection pool with specified configuration
func NewICMPConnectionPool(maxSize int, maxIdleTime time.Duration) *ICMPConnectionPool {
	if maxSize <= 0 {
		maxSize = 1 // Minimum of 1 to allow at least one connection
	}

	if maxIdleTime <= 0 || maxIdleTime > MaxAllowedIdleTime {
		maxIdleTime = DefaultMaxIdleTime // Use defined default for idle timeout
	}

	p := &ICMPConnectionPool{
		maxSize:     maxSize,
		currentSize: 0,
		maxIdleTime: maxIdleTime,
		pool:        make(chan *ICMPConnection, maxSize),
		stopChan:    make(chan struct{}),
	}

	go p.startCleanupLoop()
	return p
}

// GetConnection retrieves an available connection from the pool or creates a new one if under max size
func (p *ICMPConnectionPool) GetConnection() (*ICMPConnection, error) {
	if p == nil {
		return nil, errors.New("connection pool not initialized")
	}

	// 1. Try to get an existing connection from the channel first (O(1) access)
	select {
	case conn := <-p.pool:
		// Check if it's expired before handing it back
		if time.Since(conn.LastUsed) > p.maxIdleTime {
			conn.Close()
			p.mu.Lock()
			p.currentSize--
			p.mu.Unlock()
			return p.GetConnection() // Recursive call to try next or create fresh
		}
		conn.LastUsed = time.Now() // Update usage timestamp
		return conn, nil

	default:
		// 2. No idle connection found, attempt to expand the pool up to maxSize
		p.mu.Lock()
		if p.currentSize < p.maxSize {
			p.currentSize++
			p.mu.Unlock()

			conn, err := p.createNewConnectionLocked()
			if err != nil {
				p.mu.Lock()
				p.currentSize-- // revert count on failure
				p.mu.Unlock()
				return nil, fmt.Errorf("failed to create new connection: %v", err)
			}
			return conn, nil
		}
		p.mu.Unlock()

		// 3. Pool is at capacity; return error
		return nil, fmt.Errorf("connection pool exhausted")
	}
}

// ReturnConnection marks a connection as available again for reuse in the pool
func (p *ICMPConnectionPool) ReturnConnection(conn *ICMPConnection) {
	if conn == nil || conn.Conn == nil {
		return
	}

	conn.LastUsed = time.Now() // Mark as recently used

	// Try to put it back into the pool channel
	select {
	case p.pool <- conn:
		// Successfully returned to idle queue
	default:
		// Pool is full (edge case), close and reduce count
		conn.Close()
		p.mu.Lock()
		p.currentSize--
		p.mu.Unlock()
	}
}

// CloseAllConnections safely closes all connections in the pool and clears them
func (p *ICMPConnectionPool) CloseAllConnections() {
	close(p.stopChan) // Signal cleanup loop to stop

	p.mu.Lock()
	defer p.mu.Unlock()

	// Drain the channel
	for len(p.pool) > 0 {
		conn := <-p.pool
		conn.Close()
	}
	p.currentSize = 0
}

// createNewConnectionLocked creates a new ICMP connection (must be called with lock held)
func (p *ICMPConnectionPool) createNewConnectionLocked() (*ICMPConnection, error) {
	// Create UDP socket for ICMP over IPv4 or IPv6 using net.ListenPacket
	conn, err := net.ListenPacket("ip:icmp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("failed to create ICMP connection: %v", err)
	}

	// Generate unique ID using current pool size for simplicity in this implementation
	id := p.currentSize + 1 

	return NewICMPConnection(conn, id), nil
}

// startCleanupLoop runs a background goroutine to purge expired connections
func (p *ICMPConnectionPool) startCleanupLoop() {
	ticker := time.NewTicker(p.maxIdleTime / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.performCleanup()
		case <-p.stopChan:
			return
		}
	}
}

func (p *ICMPConnectionPool) performCleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	numInPool := len(p.pool)
	for i := 0; i < numInPool; i++ {
		select {
		case conn := <-p.pool:
			if time.Since(conn.LastUsed) > p.maxIdleTime {
				conn.Close()
				p.currentSize--
			} else {
				// Not expired, put it back into the pool
				p.pool <- conn
			}
		default:
			break
		}
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

// performPing sends a ping request using the provided connection and target
func (p *ICMPConnectionPool) performPing(conn net.PacketConn, target string) error {
	// Implementation logic would go here
	return nil 
}

// isValidIPAddress checks if a string is a valid IPv4, IPv6 address, or localhost
func isValidIPAddress(ip string) bool {
	ip = strings.TrimSpace(ip)
	if len(ip) == 0 { return false }
	if ip == "localhost" || ip == "127.0.0.1" { return true }

	if ipv4 := net.ParseIP(ip); ipv4 != nil && ipv4.To4() != nil {
		return true
	}

	if strings.Contains(ip, ":") {
		parts := strings.Split(ip, ":")
		if len(parts) < 2 || len(parts) > 8 { return false }
		for _, part := range parts {
			if len(part) == 0 || len(part) > 4 { return false }
			isHex := true
			for _, char := range part {
				if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
					isHex = false; break
				}
			}
			if !isHex { return false }
		}
		return true
	}
	return false
}

// GetPoolStats returns current pool statistics for monitoring/debugging
func (p *ICMPConnectionPool) GetPoolStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"total_connections": p.currentSize,
		"pool_count":        len(p.pool),
		"max_size":          p.maxSize,
		"max_idle_time":     p.maxIdleTime.String(),
	}
}
