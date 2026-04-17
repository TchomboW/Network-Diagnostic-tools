package utils

import (
	"sync"
	"testing"
	"time"
)

// TestNewDNSCacheCreatesEmptyCache verifies that a newly created DNS cache starts empty
// and has the correct default TTL of 60 seconds.
func TestNewDNSCacheCreatesEmptyCache(t *testing.T) {
	cache := NewDNSCache()

	// Verify cache starts empty (no entries)
	allCaches := cache.GetAllCaches()
	if len(allCaches) != 0 {
		t.Errorf("Expected cache to start with 0 entries, got %d", len(allCaches))
	}

	// Verify default TTL is 60 seconds
	defaultTTL := cache.GetDefaultTTL()
	expectedTTL := 60 * time.Second
	if defaultTTL != expectedTTL {
		t.Errorf("Expected default TTL of %v, got %v", expectedTTL, defaultTTL)
	}

	// Test that Resolve on empty cache returns false for invalid hostname
	ip, success := cache.Resolve("")
	if success {
		t.Error("Empty hostname should return failure")
	}
	if ip != "" {
		t.Errorf("Expected empty IP for empty hostname, got %s", ip)
	}

	t.Log("✓ TestNewDNSCacheCreatesEmptyCache passed: Cache starts empty with 60s TTL")
}

// TestResolveFirstLookupPerformsActualDNS verifies that the first lookup of a hostname
// performs an actual DNS resolution and succeeds for valid hostnames.
func TestResolveFirstLookupPerformsActualDNS(t *testing.T) {
	cache := NewDNSCache()

	// Test with a known IP address (8.8.8.8 is Google's public DNS)
	ip, success := cache.Resolve("8.8.8.8")
	if !success {
		t.Errorf("Expected successful resolution for 8.8.8.8, got failure")
	}
	if ip == "" {
		t.Error("Expected non-empty IP address from resolution")
	}

	// Verify the result is cached by checking GetAllCaches
	allCaches := cache.GetAllCaches()
	if len(allCaches) != 1 {
		t.Errorf("Expected 1 entry after first lookup, got %d", len(allCaches))
	}
	if allCaches["8.8.8.8"] != ip {
		t.Errorf("Cached IP doesn't match resolved IP")
	}

	t.Log("✓ TestResolveFirstLookupPerformsActualDNS passed: DNS resolution performed successfully")
}

// TestResolveCachedReturnsImmediately verifies that subsequent lookups of the same hostname
// return from cache instantly (<1ms) and return the same IP address.
func TestResolveCachedReturnsImmediately(t *testing.T) {
	cache := NewDNSCache()

	// First lookup - should perform DNS resolution (takes some time)
	startFirst := time.Now()
	ip1, success1 := cache.Resolve("8.8.8.8")
	durationFirst := time.Since(startFirst)

	if !success1 {
		t.Fatal("First resolution failed")
	}

	// Second lookup - should be from cache (should be much faster)
	startSecond := time.Now()
	ip2, success2 := cache.Resolve("8.8.8.8")
	durationSecond := time.Since(startSecond)

	if !success2 {
		t.Fatal("Second resolution failed")
	}

	// Verify the same IP is returned both times
	if ip1 != ip2 {
		t.Errorf("Expected same IP from cache, got %s vs %s", ip1, ip2)
	}

	// Verify second lookup was much faster (cached response should be < 1ms)
	maxCachedTime := time.Millisecond
	if durationSecond > maxCachedTime {
		t.Errorf("Expected cached resolution to complete in < %v, took %v", maxCachedTime, durationSecond)
	}

	// First resolution should take more time than second (if it did DNS lookup)
	if durationFirst > maxCachedTime*10 && durationFirst < durationSecond {
		t.Errorf("Expected first lookup to be slower than cached, but got %v vs %v", durationFirst, durationSecond)
	}

	// Verify cache size after both lookups (should still be 1 entry)
	allCaches := cache.GetAllCaches()
	if len(allCaches) != 1 {
		t.Errorf("Expected 1 entry in cache, got %d", len(allCaches))
	}

	t.Logf("✓ TestResolveCachedReturnsImmediately passed: First lookup took %v, cached lookup took %v (<1ms)", durationFirst, durationSecond)
}

// TestClearRemovesEntry verifies that Clear() removes a hostname from the cache
// and subsequent Resolve performs actual DNS again.
func TestClearRemovesEntry(t *testing.T) {
	cache := NewDNSCache()

	// First lookup - should perform DNS resolution
	ip1, success1 := cache.Resolve("8.8.8.8")
	if !success1 {
		t.Fatal("First resolution failed")
	}
	
	// Verify it's cached
	allCachesBefore := cache.GetAllCaches()
	if len(allCachesBefore) != 1 {
		t.Errorf("Expected 1 entry after first lookup, got %d", len(allCachesBefore))
	}

	// Clear the hostname
	cache.Clear("8.8.8.8")

	// Verify it's removed from cache
	allCachesAfter := cache.GetAllCaches()
	if len(allCachesAfter) != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", len(allCachesAfter))
	}

	// Second lookup - should perform DNS again (not from cache)
	ip2, success2 := cache.Resolve("8.8.8.8")
	if !success2 {
		t.Fatal("Second resolution failed after clear")
	}

	// Verify the same IP is returned (should be a valid resolution)
	if ip1 != ip2 {
		t.Errorf("Expected same IP after re-resolution, got %s vs %s", ip1, ip2)
	}

	// Verify cache has the entry again
	allCachesRe := cache.GetAllCaches()
	if len(allCachesRe) != 1 {
		t.Errorf("Expected 1 entry after second lookup, got %d", len(allCachesRe))
	}

	// Test clearing non-existent hostname (should not panic or error)
	cache.Clear("nonexistent.host") // Should do nothing gracefully
	
	// Verify still only 1 entry in cache
	allCachesFinal := cache.GetAllCaches()
	if len(allCachesFinal) != 1 {
		t.Errorf("Expected 1 entry after clearing non-existent host, got %d", len(allCachesFinal))
	}

	t.Log("✓ TestClearRemovesEntry passed: Clear removes entries and triggers re-resolution")
}

// TestSetTTLChangesDefault sets a new TTL and verifies subsequent lookups use the new TTL.
func TestSetTTLChangesDefault(t *testing.T) {
	cache := NewDNSCache()
	
	// Verify initial default is 60 seconds
	if cache.GetDefaultTTL() != 60*time.Second {
		t.Errorf("Expected initial TTL of 60s, got %v", cache.GetDefaultTTL())
	}

	// Set a new TTL
	newTTL := 30 * time.Second
	cache.SetTTL(newTTL)

	// Verify the TTL was set correctly
	if cache.GetDefaultTTL() != newTTL {
		t.Errorf("Expected TTL of %v after SetTTL, got %v", newTTL, cache.GetDefaultTTL())
	}

	// Test with 0 TTL (should default to 60s)
	cache.SetTTL(0)
	if cache.GetDefaultTTL() != 60*time.Second {
		t.Errorf("Expected TTL of 60s after setting 0, got %v", cache.GetDefaultTTL())
	}

	t.Log("✓ TestSetTTLChangesDefault passed: SetTTL works correctly")
}

// TestConcurrentAccess tests thread safety under concurrent access.
func TestConcurrentAccess(t *testing.T) {
	cache := NewDNSCache()
	
	var wg sync.WaitGroup
	const numGoroutines = 50
	
	// Concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Some goroutines do lookups
			if id%2 == 0 {
				cache.Resolve("8.8.8.8")
			} else {
				// Others do cache operations
				cache.Clear("8.8.8.8")
				cache.GetAllCaches()
			}
		}(i)
	}
	
	wg.Wait()

	// Verify no race conditions - should not have panicked or produced inconsistent state
	allCaches := cache.GetAllCaches()
	if len(allCaches) > 1 {
		t.Errorf("Concurrent access resulted in unexpected cache size: %d", len(allCaches))
	}

	t.Log("✓ TestConcurrentAccess passed: Thread-safe operations completed without race conditions")
}

// TestExpiredEntriesAreRemoved tests that expired entries are automatically excluded from GetAllCaches
func TestExpiredEntriesAreRemoved(t *testing.T) {
	cache := NewDNSCache()
	
	// Manually add an entry with 0 TTL (expires immediately) - this should not appear in GetAllCaches
	entry := &DNSCacheEntry{
		IP:        "127.0.0.1",
		TTL:       time.Duration(0), // Expires immediately
		Timestamp: time.Now(),
	}
	cache.mu.Lock()
	cache.entries["expired.host"] = entry
	cache.mu.Unlock()

	// Verify it's NOT in the cache initially because TTL=0 expires immediately
	allCaches := cache.GetAllCaches()
	if len(allCaches) != 0 {
		t.Errorf("Expected 0 entries (TTL=0 expires immediately), got %d", len(allCaches))
	}

	// Add a valid entry that will expire
	cache.mu.Lock()
	cache.entries["valid.host"] = &DNSCacheEntry{
		IP:        "127.0.0.2",
		TTL:       5 * time.Millisecond, // Very short TTL for testing
		Timestamp: time.Now(),
	}
	cache.mu.Unlock()

	// Verify it's in the cache initially with valid entry
	allCaches = cache.GetAllCaches()
	if len(allCaches) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(allCaches))
	}

	// After a short delay, verify it's no longer in GetAllCaches (expired entries excluded)
	time.Sleep(10 * time.Millisecond) // Give time for expiration
	allCaches = cache.GetAllCaches()
	if len(allCaches) != 0 {
		t.Errorf("Expected 0 non-expired entries after expiration, got %d", len(allCaches))
	}

	t.Log("✓ TestExpiredEntriesAreRemoved passed: Expired entries are not returned by GetAllCaches")
}
