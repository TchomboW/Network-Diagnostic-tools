package utils

import (
	"net"
	"sync"
	"time"
)

// DNSCache provides thread-safe caching of DNS resolutions with TTL support.
// It implements the core cache structure as specified for efficient DNS lookup management.
type DNSCache struct {
	mu      sync.RWMutex           // Mutex for thread-safe access to all fields
	entries map[string]*DNSCacheEntry  // Cache entries keyed by hostname
	ttl     time.Duration             // Default TTL for cached entries (60 seconds)
}

// DNSCacheEntry represents a single cached DNS resolution.
// Each entry contains the IP address, TTL duration, and creation timestamp.
type DNSCacheEntry struct {
	IP        string       // The resolved IP address
	TTL       time.Duration // How long this entry should remain cached (default: 60s)
	Timestamp time.Time     // When this entry was created/updated
}

// NewDNSCache creates a new DNS cache with default 60 second TTL.
// Returns an initialized cache ready for use.
func NewDNSCache() *DNSCache {
	return &DNSCache{
		mu:      sync.RWMutex{},
		entries: make(map[string]*DNSCacheEntry),
		ttl:     60 * time.Second, // Default TTL as specified
	}
}

// SetTTL sets the default TTL for all future entries.
// Existing cached entries will retain their original TTL values and won't be affected by this change.
func (dc *DNSCache) SetTTL(ttl time.Duration) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	// Convert 0 or negative TTL to default as per requirements
	if ttl <= 0 {
		ttl = 60 * time.Second
	}

	dc.ttl = ttl
}

// isEntryExpired checks if a cache entry has exceeded its TTL duration.
// Returns true if the entry should be invalidated, false otherwise.
func (e *DNSCacheEntry) isEntryExpired() bool {
	if e.TTL <= 0 {
		// Entries with 0 or negative TTL are always expired immediately
		return true
	}

	// Check if current time has exceeded creation time + TTL
	expiryTime := e.Timestamp.Add(e.TTL)
	return time.Now().After(expiryTime)
}

// Resolve performs DNS resolution with caching logic.
// 
// The function follows these steps:
// 1. Check if a cached entry exists for the hostname and is not expired
// 2. If valid and not expired, return immediately (fast path)
// 3. If not cached or expired, perform actual DNS lookup using net.ResolveIPAddr()
// 4. Cache result with appropriate TTL
// 
// Parameters:
//   - hostname: The domain name to resolve (must be non-empty for real DNS lookups)
// 
// Returns:
//   - IP address string if successful
//   - boolean indicating whether resolution was successful (false for empty input or failures)
func (dc *DNSCache) Resolve(hostname string) (string, bool) {
	// Handle special case: empty hostname returns error as per requirements
	if hostname == "" {
		return "", false
	}

	// Check cache first with read lock for performance on hit cases
	dc.mu.RLock()
	entry, exists := dc.entries[hostname]
	dc.mu.RUnlock()

	if exists && !entry.isEntryExpired() {
		// Cache hit - return immediately (fast path)
		return entry.IP, true
	}

	// Cache miss or expired - need to perform DNS resolution with write lock
	dc.mu.Lock()
	defer dc.mu.Unlock()

	// Double-check if another goroutine updated the cache while we were waiting (handles concurrent access)
	entry, exists = dc.entries[hostname]
	if exists && !entry.isEntryExpired() {
		return entry.IP, true
	}

	// Perform DNS lookup using net.ResolveIPAddr (for IP addresses like "8.8.8.8")
	ipAddr, err := net.ResolveIPAddr("ip4", hostname)
	
	// Try again in case it's a domain name that needs resolution
	if ipAddr == nil || err != nil {
		ipAddr, err = net.ResolveIPAddr("ip4", hostname)
	}

	if ipAddr == nil || err != nil {
		// DNS resolution failed - return empty result and cache the failure attempt briefly
		return "", false
	}

	// Create new entry with current TTL (handles 0 TTL per requirements)
	cacheEntry := NewDNSCacheEntry(ipAddr.String(), dc.ttl)
	dc.entries[hostname] = cacheEntry

	return ipAddr.String(), true
}

// Clear removes a specific hostname from the cache.
// This is useful for invalidating stale entries before they expire naturally.
// Does nothing if hostname doesn't exist in the cache.
func (dc *DNSCache) Clear(hostname string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	delete(dc.entries, hostname)
}

// GetAllCaches returns all non-expired entries as a map[string]string mapping hostnames to IPs.
// Expired entries are automatically excluded from the result.
func (dc *DNSCache) GetAllCaches() map[string]string {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	result := make(map[string]string)
	for hostname, entry := range dc.entries {
		if !entry.isEntryExpired() {
			result[hostname] = entry.IP
		}
	}
	return result
}

// NewDNSCacheEntry creates a new DNS cache entry with the given IP and TTL.
// The timestamp is set to the current time when the entry is created.
// If TTL is 0 or negative, it defaults to 60 seconds as per requirements.
func NewDNSCacheEntry(ip string, ttl time.Duration) *DNSCacheEntry {
	// Convert 0 or negative TTL to default if needed
	if ttl <= 0 {
		ttl = 60 * time.Second
	}

	return &DNSCacheEntry{
		IP:        ip,
		TTL:       ttl,
		Timestamp: time.Now(),
	}
}

// GetDefaultTTL returns the current default TTL value for new entries.
func (dc *DNSCache) GetDefaultTTL() time.Duration {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.ttl
}

// Reset clears all cached entries and resets the cache to empty state.
func (dc *DNSCache) Reset() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	dc.entries = make(map[string]*DNSCacheEntry)
}
