// utils/dns_cache.go - DNS Caching with TTL Support
package main

import (
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	// DefaultTTL is the default time-to-live for cached DNS entries
	DefaultTTL = 60 * time.Second

	// MinIdleTime is the minimum idle timeout for connection reuse
	MinIdleTime = 10 * time.Millisecond

	// MaxIdleTime is the maximum idle timeout for connection reuse
	MaxIdleTime = time.Hour

	// TTLRemainingLowThreshold is when we start warning about low TTL remaining
	TTLRemainingLowThreshold = 10 * time.Second
)

// DNSCacheEntry represents a cached DNS resolution result
type DNSCacheEntry struct {
	IP        string
	TTL       time.Duration
	Timestamp time.Time
}

// DNSCache provides thread-safe DNS caching with TTL support
type DNSCache struct {
	mu      sync.RWMutex
	entries map[string]DNSCacheEntry
	ttl     time.Duration
}

// NewDNSCache creates a new DNS cache instance
func NewDNSCache() *DNSCache {
	return &DNSCache{
		entries: make(map[string]DNSCacheEntry),
		ttl:     DefaultTTL, // Use the defined default TTL constant
	}
}

// SetTTL configures the default time-to-live for cached DNS results
func (dc *DNSCache) SetTTL(ttl time.Duration) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.ttl = ttl
	fmt.Printf("DNS Cache TTL updated to: %v\n", dc.ttl)
}

// GetTTL returns the current TTL setting for DNS cache entries
func (dc *DNSCache) GetTTL() time.Duration {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.ttl
}

// Resolve attempts to resolve a hostname, using cached results when available
// Returns: IP address and whether the result was from cache (true = cache hit)
func (dc *DNSCache) Resolve(hostname string) (string, bool) {
	// First check if we have a valid cached entry
	dc.mu.RLock()
	entry, exists := dc.entries[hostname]
	dc.mu.RUnlock()

	if !exists || time.Since(entry.Timestamp) > entry.TTL {
		// Cache miss or expired - perform fresh DNS lookup
		fmt.Printf("DNS cache miss for: %s\n", hostname)
		return dc.performLookup(hostname, false)
	}

	// Cache hit - return stored IP address
	fmt.Printf("DNS cache hit for: %s -> %s (TTL remaining: %v)\n",
		hostname, entry.IP, entry.TTL-time.Since(entry.Timestamp))
	return entry.IP, true
}

// performLookup performs a fresh DNS resolution and caches the result
func (dc *DNSCache) performLookup(hostname string, useCached bool) (string, bool) {
	ipAddr, err := net.ResolveIPAddr("ip4", hostname)
	if err != nil {
		fmt.Printf("DNS lookup failed for %s: %v\n", hostname, err)
		return "", false
	}

	// Cache the successful result
	newEntry := DNSCacheEntry{
		IP:        ipAddr.IP.String(),
		TTL:       dc.ttl,
		Timestamp: time.Now(),
	}

	dc.mu.Lock()
	dc.entries[hostname] = newEntry
	dc.mu.Unlock()

	fmt.Printf("DNS resolved and cached: %s -> %s\n", hostname, ipAddr.IP)
	return ipAddr.IP.String(), true
}

// Clear removes a specific DNS entry from the cache
func (dc *DNSCache) Clear(hostname string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if _, exists := dc.entries[hostname]; exists {
		delete(dc.entries, hostname)
		fmt.Printf("Cleared DNS entry for: %s\n", hostname)
	} else {
		fmt.Printf("DNS entry not found for: %s\n", hostname)
	}
}

// ClearAll removes all cached DNS entries
func (dc *DNSCache) ClearAll() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	count := len(dc.entries)
	if count > 0 {
		dc.entries = make(map[string]DNSCacheEntry)
		fmt.Printf("Cleared all %d DNS cache entries\n", count)
	} else {
		fmt.Println("No DNS entries to clear")
	}
}

// GetStats returns current cache statistics for monitoring/debugging
func (dc *DNSCache) GetStats() map[string]interface{} {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	count := len(dc.entries)
	if count == 0 {
		return map[string]interface{}{
			"total_entries": 0,
			"expired_count": 0,
			"hit_rate":      "N/A",
		}
	}

	// Count expired entries
	expiredCount := 0
	for _, entry := range dc.entries {
		if time.Since(entry.Timestamp) > entry.TTL {
			expiredCount++
		}
	}

	return map[string]interface{}{
		"total_entries": count,
		"expired_count": expiredCount,
		"hit_rate":      fmt.Sprintf("%.2f%%", (100.0*float64(count-expiredCount))/float64(count)),
	}
}

// GetActiveEntries returns all currently valid (non-expired) DNS cache entries
func (dc *DNSCache) GetActiveEntries() map[string]string {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	active := make(map[string]string)
	for host, entry := range dc.entries {
		if time.Since(entry.Timestamp) <= entry.TTL {
			active[host] = entry.IP
		}
	}
	return active
}

// GetEntry returns a specific DNS cache entry (for testing/debugging)
func (dc *DNSCache) GetEntry(hostname string) (*DNSCacheEntry, bool) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	entry, exists := dc.entries[hostname]
	if !exists || time.Since(entry.Timestamp) > entry.TTL {
		return nil, false
	}

	return &entry, true
}

// String returns a formatted string representation of the DNS cache contents
func (dc *DNSCache) String() string {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	result := "DNS Cache Contents:\n"
	if len(dc.entries) == 0 {
		return result + "  (empty)"
	}

	for host, entry := range dc.entries {
		ttlRemaining := entry.TTL - time.Since(entry.Timestamp)
		status := ""
		if ttlRemaining < 0 {
			status = " [EXPIRED]"
		} else if ttlRemaining < TTLRemainingLowThreshold {
			status = " [LOW TTL]"
		}
		result += fmt.Sprintf("  %s -> %s%s (TTL: %v)\n", host, entry.IP, status, ttlRemaining)
	}
	return result
}
