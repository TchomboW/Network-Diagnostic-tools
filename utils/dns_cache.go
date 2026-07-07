package utils

import (
	"fmt"
	"hash/fnv"
	"net"
	"sync"
	"time"
)

// DNSCacheEntry represents a cached DNS resolution result
type DNSCacheEntry struct {
	IP        string
	TTL       time.Duration
	Timestamp time.Time
}

// cacheShard is an internal structure for a single locked piece of the cache
type cacheShard struct {
	mu      sync.RWMutex
	entries map[string]DNSCacheEntry
}

// DNSCache provides thread-safe, sharded DNS caching to reduce lock contention
type DNSCache struct {
	shards       []*cacheShard
	numShards    int
	defaultTTL   time.Duration
}

// NewDNSCache creates a new sharded DNS cache instance
func NewDNSCache() *DNSCache {
	const shardCount = 16 // Power of 2 is ideal for distribution
	dc := &DNSCache{
		numShards:  shardCount,
		shards:     make([]*cacheShard, shardCount),
		defaultTTL: time.Second * 60, // Default 1 minute
	}

	for i := 0; i < shardCount; i++ {
		dc.shards[i] = &cacheShard{
			entries: make(map[string]DNSCacheEntry),
		}
	}

	return dc
}

// getShardIndex calculates the index for a given hostname using FNV-1a hash
func (dc *DNSCache) getShardIndex(hostname string) int {
	h := fnv.New32a()
	h.Write([]byte(hostname))
	return int(h.Sum32()) % dc.numShards
}

// SetTTL configures the default time-to-live for all new entries
func (dc *DNSCache) SetTTL(ttl time.Duration) {
	dc.defaultTTL = ttl
}

// GetTTL returns the current configured TTL
func (dc *DNSCache) GetTTL() time.Duration {
	return dc.defaultTTL
}

// Resolve attempts to resolve a hostname, using cached results when available
func (dc *DNSCache) Resolve(hostname string) (string, bool) {
	idx := dc.getShardIndex(hostname)
	shard := dc.shards[idx]

	shard.mu.RLock()
	entry, exists := shard.entries[hostname]
	shard.mu.RUnlock()

	if !exists || time.Since(entry.Timestamp) > entry.TTL {
		// Cache miss or expired - perform fresh DNS lookup
		fmt.Printf("DNS cache miss/expired for: %s\n", hostname)
		return dc.performLookup(hostname, false)
	}

	fmt.Printf("DNS cache hit for: %s -> %s (TTL remaining: %v)\n", 
		hostname, entry.IP, entry.TTL-time.Since(entry.Timestamp))
	return entry.IP, true
}

// performLookup performs a fresh DNS resolution and caches the result into the correct shard
func (dc *DNSCache) performLookup(hostname string, useCached bool) (string, bool) {
	ipAddr, err := net.ResolveIPAddr("ip4", hostname)
	if err != nil {
		fmt.Printf("DNS lookup failed for %s: %v\n", hostname, err)
		return "", false
	}

	newEntry := DNSCacheEntry{
		IP:        ipAddr.IP.String(),
		TTL:       dc.defaultTTL,
		Timestamp: time.Now(),
	}

	idx := dc.getShardIndex(hostname)
	shard := dc.shards[idx]

	shard.mu.Lock()
	shard.entries[hostname] = newEntry
	shard.mu.Unlock()

	fmt.Printf("DNS resolved and cached: %s -> %s\n", hostname, ipAddr.IP)
	return ipAddr.IP.String(), true
}

// Clear removes a specific DNS entry from its corresponding shard
func (dc *DNSCache) Clear(hostname string) {
	idx := dc.getShardIndex(hostname)
	shard := dc.shards[idx]

	shard.mu.Lock()
	defer shard.mu.Unlock()
	delete(shard.entries, hostname)
	fmt.Printf("Cleared DNS entry for: %s\n", hostname)
}

// ClearAll removes all cached DNS entries
func (dc *DNSCache) ClearAll() {
	for _, shard := range dc.shards {
		shard.mu.Lock()
		shard.entries = make(map[string]DNSCacheEntry)
		shard.mu.Unlock()
	}
	fmt.Println("Cleared all DNS cache entries")
}

// GetStats returns current cache statistics for monitoring/debugging
func (dc *DNSCache) GetStats() map[string]interface{} {
	totalEntries := 0
	expiredCount := 0

	for _, shard := range dc.shards {
		shard.mu.RLock()
		count := len(shard.entries)
		totalEntries += count
		for _, entry := range shard.entries {
			if time.Since(entry.Timestamp) > entry.TTL {
				expiredCount++
			}
		}
		shard.mu.RUnlock()
	}

	hitRate := 0.0
	if totalEntries > 0 {
		hitRate = (100.0 * float64(totalEntries-expiredCount)) / float64(totalEntries)
	}

	return map[string]interface{}{
		"total_entries": totalEntries,
		"expired_count": expiredCount,
		"hit_rate":      fmt.Sprintf("%.2f%%", hitRate),
	}
}

// GetActiveEntries returns all currently valid (non-expired) DNS cache entries
func (dc *DNSCache) GetActiveEntries() map[string]string {
	active := make(map[string]string)
	for _, shard := range dc.shards {
		shard.mu.RLock()
		for host, entry := range shard.entries {
			if time.Since(entry.Timestamp) <= entry.TTL {
				active[host] = entry.IP
			}
		}
		shard.mu.RUnlock()
	}
	return active
}
