// utils/dns_cache_debug_final.go - Final comprehensive debug of the DNS cache issue (FIXED)
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== DNS Cache Final Comprehensive Debug ===\n")
	
	cache := NewDNSCache()
	testTTL := 50 * time.Millisecond
	cache.SetTTL(testTTL)
	
	// Check initial state - but let's be very thorough about what happens during initialization
	fmt.Println("1. Initial cache state (including during NewDNSCache):")
	stats := cache.GetStats()
	totalEntries, _ := stats["total_entries"].(int)
	fmt.Printf("   Total entries from GetStats: %d\n", totalEntries)
	
	// Check the map directly after creation
	cache.mu.RLock()
	initialMapLen := len(cache.entries)
	fmt.Printf("   Direct cache map length: %d\n", initialMapLen)
	if initialMapLen > 0 {
		for k, v := range cache.entries {
			fmt.Printf("   Entry found: %s -> %+v\n", k, v)
		}
	}
	cache.mu.RUnlock()
	
	// Now check if "8.8.8.8" exists before any resolution
	fmt.Println("\n2. Checking for specific entry \"8.8.8.8\":")
	cache.mu.RLock()
	entry, exists := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	
	fmt.Printf("   Exists: %v\n", exists)
	if !exists {
		fmt.Println("   (Entry doesn't exist - this is expected)")
		
		// Let's check what happens when we try to access entry fields anyway
		fmt.Println("\n3. Checking what happens when accessing non-existent entry fields:")
		now := time.Now()
		timestampZero := entry.Timestamp  // This should be zero-time
		durationZero := entry.TTL         // This should be zero duration
		
		fmt.Printf("   Zero-value Timestamp: %v (type: %T)\n", timestampZero, timestampZero)
		fmt.Printf("   Zero-value TTL:       %v (type: %T)\n", durationZero, durationZero)
		
		timeSince := now.Sub(timestampZero)
		expiredCondition := timeSince > durationZero
		
		fmt.Printf("   Time since zero-time: %v\n", timeSince)
		fmt.Printf("   TTL (zero):           %v\n", durationZero)
		fmt.Printf("   Is expired? (time.Since > TTL): %v\n", expiredCondition)
		
		fullCondition := !exists || expiredCondition
		fmt.Printf("   Full condition (!exists || isExpired): %v\n", fullCondition)
		
		if fullCondition {
			fmt.Println("   -> Should go to MISS branch (correct logic)")
		} else {
			fmt.Println("   -> Would go to HIT branch (WRONG! - but we already checked it's correct)")
		}
	} else {
		fmt.Printf("   Entry data: %+v\n", entry)
		
		now := time.Now()
		timestamp := entry.Timestamp
		currentTTL := entry.TTL
		timeSince := now.Sub(timestamp)
		
		fmt.Printf("   Timestamp: %v\n", timestamp)
		fmt.Printf("   TTL:       %v\n", currentTTL)
		fmt.Printf("   Time since creation: %v\n", timeSince)
		
		isExpired := timeSince > currentTTL
		fmt.Printf("   Is expired? (time.Since > TTL): %v\n", isExpired)
		
		if !isExpired {
			fmt.Println("   -> Should go to HIT branch")
		} else {
			fmt.Println("   -> Should go to MISS branch (entry exists but expired)")
		}
	}
	
	// Now let's try the resolution itself, with detailed logging
	fmt.Println("\n4. Attempting actual resolution:")
	cache.mu.RLock()
	entryBeforeExists, _ := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	
	fmt.Printf("   Before resolve: exists=%v\n", entryBeforeExists)
	
	ip1, hit1 := cache.Resolve("8.8.8.8")
	fmt.Printf("   Result after resolve: IP=%s, Hit=%v\n", ip1, hit1)
	
	// Check immediately after resolution
	cache.mu.RLock()
	entryAfter, existsAfter := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	
	fmt.Printf("\n5. After resolution:\n")
	fmt.Printf("   Entry exists: %v\n", existsAfter)
	if !existsAfter {
		fmt.Println("   ❌ BUG: Entry doesn't exist after resolution!")
		
		// Let's check what the map actually contains now
		cache.mu.RLock()
		allEntries := len(cache.entries)
		fmt.Printf("   Total entries in cache: %d\n", allEntries)
		for k, v := range cache.entries {
			fmt.Printf("   Entry found: %s -> %+v (exists=%v)\n", k, v, existsAfter)
		}
		cache.mu.RUnlock()
		
	} else {
		now := time.Now()
		timestamp := entryAfter.Timestamp
		currentTTL := entryAfter.TTL
		timeSinceNow := now.Sub(timestamp)
		
		fmt.Printf("   Timestamp: %v\n", timestamp)
		fmt.Printf("   TTL:       %v\n", currentTTL)
		fmt.Printf("   Time since creation: %v\n", timeSinceNow)
		
		isExpiredNow := timeSinceNow > currentTTL
		fmt.Printf("   Is expired: %v (time.Since > TTL)\n", isExpiredNow)
		
		if hit1 && existsAfter {
			if !isExpiredNow {
				fmt.Println("✅ Normal behavior for cache HIT with valid entry")
			} else {
				fmt.Println("⚠️  WARNING: Got HIT when entry was expired - TTL check has bug!")
			}
		} else if hit1 && !existsAfter {
			fmt.Println("❌ BUG: Got HIT but entry doesn't exist!")
		} else if !hit1 && existsAfter {
			fmt.Println("⚠️  WARNING: Got MISS but entry was created - this shouldn't happen!")
		} else if !hit1 && !existsAfter {
			fmt.Println("✅ Normal behavior for cache MISS (entry doesn't exist)")
		}
	}
	
	fmt.Println("\n=== Debug Complete ===")
}