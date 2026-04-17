// utils/dns_cache_test.go - Unit tests for DNS caching functionality (FINAL FIXED VERSION)
package main

import (
	"strings"
	"testing"
	"time"
)

func TestDNSCacheBasicFunctionality(t *testing.T) {
	cache := NewDNSCache()
	
	// Test 1: Initial state should be empty
	stats := cache.GetStats()
	totalEntries, ok := stats["total_entries"].(int)
	if !ok || totalEntries != 0 {
		t.Errorf("Expected 0 entries, got %v (type: %T)", stats["total_entries"], stats["total_entries"])
	}
	
	// Test 2: Set a custom TTL and verify it's applied
	customTTL := 30 * time.Second
	cache.SetTTL(customTTL)
	if cache.GetTTL() != customTTL {
		t.Errorf("Expected TTL %v, got %v", customTTL, cache.GetTTL())
	}
	
	// Test 3: Resolve a hostname (should perform fresh lookup and cache result)
	ip, hit := cache.Resolve("8.8.8.8")
	if !hit {
		t.Error("Expected cache miss for first resolution")
	}
	if ip == "" {
		t.Error("IP should not be empty after successful DNS resolution")
	}
	
	// Test 4: Same hostname should hit cache (second lookup)
	ip2, hit2 := cache.Resolve("8.8.8.8")
	if !hit2 {
		t.Error("Expected cache hit for second resolution of same hostname")
	}
	if ip != ip2 {
		t.Errorf("Expected same IP from cache: %s vs %s", ip, ip2)
	}
	
	// Test 5: Verify stats show cached entry
	stats = cache.GetStats()
	totalEntries, ok = stats["total_entries"].(int)
	if !ok || totalEntries != 1 {
		t.Errorf("Expected 1 cached entry, got %d (type assertion failed: %v)", 
			stats["total_entries"], ok)
	}
	
	// Additional verification of other stats
	hitRate, hitRateOk := stats["hit_rate"].(string)
	if !hitRateOk || hitRate == "N/A" {
		t.Errorf("Expected valid hit rate string, got %v", stats["hit_rate"])
	}
	expiredCount, expiredOk := stats["expired_count"].(int)
	if !expiredOk || expiredCount != 0 {
		t.Errorf("Expected 0 expired entries, got %d (type assertion failed: %v)", 
			stats["expired_count"], expiredOk)
	}
	
	t.Logf("Test DNSCacheBasicFunctionality passed - Stats: %v", stats)
}

func TestDNSCacheTTLExpiration(t *testing.T) {
	cache := NewDNSCache()
	
	// Set very short TTL for testing
	testTTL := 50 * time.Millisecond
	cache.SetTTL(testTTL)
	
	// Resolve hostname (should cache with testTTL)
	ip1, _ := cache.Resolve("8.8.8.8")
	if ip1 == "" {
		t.Fatal("Initial DNS resolution failed")
	}
	
	// Verify entry exists and is valid
	entry, exists := cache.GetEntry("8.8.8.8")
	if !exists {
		t.Fatal("DNS entry should exist after successful resolution")
	}
	
	// Test: Wait for TTL to expire and verify new lookup occurs
	time.Sleep(testTTL + 10*time.Millisecond) // Slightly longer than TTL
	
	ip2, hit := cache.Resolve("8.8.8.8")
	if hit {
		t.Error("Expected cache miss after TTL expiration")
	}
	
	// IPs should be the same (same hostname), but it was a fresh lookup
	if ip1 != ip2 {
		t.Errorf("Same hostname should resolve to same IP: %s vs %s", ip1, ip2)
	}
	
	// Verify cache still has entry after re-resolution
	entry, exists = cache.GetEntry("8.8.8.8")
	if !exists {
		t.Fatal("DNS entry should exist after re-resolution post-expiration")
	}
	
	newTTLRemaining := entry.TTL - time.Since(entry.Timestamp)
	t.Logf("After TTL expiration and re-resolution, new TTL remaining: %v", newTTLRemaining)
}

func TestDNSCacheClearOperations(t *testing.T) {
	cache := NewDNSCache()
	
	// Add some entries
	cache.Resolve("8.8.8.8")
	cache.Resolve("1.1.1.1")
	cache.Resolve("9.9.9.9")
	
	initialCount := len(cache.GetActiveEntries())
	if initialCount != 3 {
		t.Errorf("Expected 3 cached entries, got %d", initialCount)
	}
	
	// Test clearing individual entry
	cache.Clear("8.8.8.8")
	remainingCount := len(cache.GetActiveEntries())
	if remainingCount != 2 {
		t.Errorf("Expected 2 entries after clearing one, got %d", remainingCount)
	}
	
	// Verify the cleared entry is gone
	_, exists := cache.GetEntry("8.8.8.8")
	if exists {
		t.Error("Cleared entry should not be in cache anymore")
	}
	
	// Test clearing all entries
	cache.ClearAll()
	finalCount := len(cache.GetActiveEntries())
	if finalCount != 0 {
		t.Errorf("Expected 0 entries after ClearAll, got %d", finalCount)
	}
}

func TestDNSCacheConcurrentAccess(t *testing.T) {
	cache := NewDNSCache()
	
	// Simulate concurrent reads and writes
	done := make(chan bool, 10)
	
	// Multiple goroutines trying to resolve the same hostname
	for i := 0; i < 5; i++ {
		go func(id int) {
			ip, hit := cache.Resolve("8.8.8.8")
			
			if id == 0 {
				// First goroutine should have a cache miss
				if !hit && ip != "" {
					t.Logf("Goroutine %d: Initial lookup successful", id)
				} else if hit {
					t.Logf("Goroutine %d: Unexpected cache hit on first call", id)
				} else {
					t.Errorf("Goroutine %d: Failed to resolve hostname", id)
				}
			} else {
				// Other goroutines should have cache hits (after first completes)
				if hit && ip != "" {
					t.Logf("Goroutine %d: Cache hit successful", id)
				} else if !hit && ip == "" {
					t.Errorf("Goroutine %d: Failed to resolve hostname after initial lookup", id)
				}
			}
			
			done <- true
		}(i + 1)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		select {
		case <-done:
			// All good
		case <-time.After(2 * time.Second):
			t.Errorf("Goroutine timed out")
		}
	}
	
	// Final verification that cache is in consistent state
	stats := cache.GetStats()
	totalEntries, ok := stats["total_entries"].(int)
	if !ok || totalEntries != 1 {
		t.Errorf("Expected exactly 1 cached entry after concurrent access, got %d (type: %T)", 
			stats["total_entries"], stats["total_entries"])
	}
}

func TestDNSCacheEmptyState(t *testing.T) {
	cache := NewDNSCache()
	
	// Should handle empty cache gracefully
	ip, hit := cache.Resolve("nonexistent.invalid.domain")
	if ip != "" {
		t.Errorf("Expected empty IP for invalid hostname, got: %s", ip)
	}
	if hit {
		t.Error("Should not have cache hit for failed DNS resolution")
	}
	
	stats := cache.GetStats()
	totalEntries, ok := stats["total_entries"].(int)
	if !ok || totalEntries != 0 {
		t.Errorf("Expected 0 entries after failed resolution, got %d (type: %T)", 
			stats["total_entries"], stats["total_entries"])
	}
}

func TestDNSCacheStringRepresentation(t *testing.T) {
	cache := NewDNSCache()
	
	// Empty cache string representation
	str := cache.String()
	expected := "DNS Cache Contents:\n  (empty)"
	if str != expected {
		t.Errorf("Empty cache string mismatch. Expected:\n%s\nGot:\n%s", expected, str)
	}
	
	// Add an entry and test representation
	cache.Resolve("8.8.8.8")
	str = cache.String()
	if len(str) < 20 {
		t.Errorf("String representation too short: %s", str)
	}
	
	// Use strings.Contains for more reliable substring matching (handles newlines)
	if !strings.Contains(str, "DNS Cache Contents:") {
		t.Error("Missing 'DNS Cache Contents:' in string representation")
	}
	
	// Check for actual IP entries in the output
	if len(cache.GetActiveEntries()) > 0 {
		cacheStr := cache.String()
		if !strings.Contains(cacheStr, "8.8.8.8") {
			t.Error("Expected to see '8.8.8.8' in DNS cache string representation after adding entry")
		}
	}
}

// Helper function for testing (kept for backward compatibility)
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAfterSubstring(s, substr))
}

func containsAfterSubstring(s, substr string) bool {
	for i := 1; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark for DNS cache performance
func BenchmarkDNSCacheResolve(b *testing.B) {
	cache := NewDNSCache()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ip, _ := cache.Resolve("8.8.8.8") // First call will be miss, rest hits
		if ip == "" && i > 0 {
			b.FailNow()
		}
	}
}

// Benchmark for DNS cache with TTL expiration
func BenchmarkDNSCacheResolveWithTTL(b *testing.B) {
	cache := NewDNSCache()
	testTTL := 100 * time.Millisecond
	cache.SetTTL(testTTL)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Alternate between resolving same host (cache hit) and waiting for expiry
		if i%2 == 0 {
			ip, _ := cache.Resolve("8.8.8.8") // Hit after first call
			if ip != "" && i > 0 {
				continue
			}
		} else {
			time.Sleep(testTTL + 5*time.Millisecond) // Expire and force fresh lookup
			ip, hit := cache.Resolve("1.1.1.1") // Miss after expiry
			if hit || ip == "" {
				b.FailNow()
			}
		}
	}
}
