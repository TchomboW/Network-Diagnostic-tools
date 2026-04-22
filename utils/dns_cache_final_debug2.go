// utils/dns_cache_final_debug2.go - Fixed version
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== DNS Cache Final Debug ===\n")
	
	cache := NewDNSCache()
	testTTL := 50 * time.Millisecond
	cache.SetTTL(testTTL)
	
	// Check initial state before any resolution
	fmt.Println("1. Initial cache state:")
	stats := cache.GetStats()
	fmt.Printf("   Total entries: %d\n", stats["total_entries"])
	activeEntries := cache.GetActiveEntries()
	fmt.Printf("   Active entries: %d\n", len(activeEntries))
	
	// Check if "8.8.8.8" exists in the map before any resolution
	cache.mu.RLock()
	existsBefore, entryBefore := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	fmt.Printf("   \"8.8.8.8\" exists before first resolve: %v\n", existsBefore)
	if existsBefore {
		fmt.Printf("   Entry data: %+v\n", entryBefore)
	}
	
	// First resolution - should be MISS and cache the result
	fmt.Println("\n2. First resolution (should be MISS):")
	ip1, hit1 := cache.Resolve("8.8.8.8")
	fmt.Printf("   Result: IP=%s, Hit=%v\n", ip1, hit1)
	
	// Check state after first resolution
	cache.mu.RLock()
	existsAfter1, entryAfter1 := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	fmt.Printf("\n3. After first resolve:\n")
	fmt.Printf("   \"8.8.8.8\" exists: %v\n", existsAfter1)
	if existsAfter1 {
		now := time.Now()
		timestamp := entryAfter1.Timestamp
		currentTTL := entryAfter1.TTL
		timeSince := now.Sub(timestamp)
		
		fmt.Printf("   Entry data: IP=%s, TTL=%v, Timestamp=%v\n", 
			entryAfter1.IP, currentTTL, timestamp)
		fmt.Printf("   Time since creation: %v\n", timeSince)
		fmt.Printf("   Is expired: %v (time.Since > TTL)\n", timeSince > currentTTL)
		
		if hit1 {
			fmt.Println("\n❌ First resolution returned HIT but entry exists - something is wrong!")
		} else if !hit1 && existsAfter1 && (currentTTL-time.Since(timestamp)) < 0 {
			fmt.Println("✅ Entry exists and was correctly identified as expired")
		} else if hit1 {
			fmt.Println("❌ First resolution returned HIT when it should have been MISS!")
			fmt.Println("This suggests there's already a cached entry, which means:")
			fmt.Println("  - Either the cache wasn't empty (but we checked)")
			fmt.Println("  - Or there's a race condition with concurrent calls")
			fmt.Println("  - Or the existence check and resolve are happening too fast for the mutex to work")
		}
	} else {
		fmt.Println("\n❌ Entry doesn't exist after first resolution but we got an IP!")
		return
	}
	
	// Now wait for TTL expiration
	fmt.Println("\n4. Waiting for TTL expiration...")
	time.Sleep(testTTL + 20 * time.Millisecond)
	now := time.Now()
	
	cache.mu.RLock()
	existsAfterWait, entryAfterWait := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	fmt.Printf("   \"8.8.8.8\" exists after wait: %v\n", existsAfterWait)
	if existsAfterWait {
		timestamp := entryAfterWait.Timestamp
		currentTTL := entryAfterWait.TTL
		timeSince := now.Sub(timestamp)
		
		fmt.Printf("   Entry data: IP=%s, TTL=%v, Timestamp=%v\n", 
			entryAfterWait.IP, currentTTL, timestamp)
		fmt.Printf("   Time since creation: %v\n", timeSince)
		fmt.Printf("   TTL remaining: %v (negative = expired)\n", currentTTL-timeSince)
		fmt.Printf("   Is expired: %v (time.Since > TTL)\n", timeSince > currentTTL)
		
		if !existsAfterWait {
			fmt.Println("❌ Entry disappeared! This shouldn't happen.")
			return
		} else if existsAfterWait && timeSince > currentTTL {
			fmt.Println("\n✅ Entry is correctly identified as expired")
		}
	} else {
		fmt.Println("❌ Entry disappeared after TTL expiration - this is unexpected!")
		return
	}
	
	// Second resolution - should be MISS because entry is expired
	fmt.Println("\n5. Second resolution (after TTL expiration, should be MISS):")
	ip2, hit2 := cache.Resolve("8.8.8.8")
	fmt.Printf("   Result: IP=%s, Hit=%v\n", ip2, hit2)
	
	// Final analysis
	fmt.Println("\n=== Summary ===")
	if !hit1 && !hit2 {
		fmt.Println("✅ Both resolutions correctly returned MISS (first fresh lookup, second after expiration)")
	} else if hit1 {
		fmt.Println("❌ First resolution should have been MISS but was HIT - BUG!")
	} else if hit2 {
		fmt.Println("❌ Second resolution should have been MISS (after TTL expiration) but was HIT - BUG!")
	}
	
	if !hit1 && hit2 {
		fmt.Println("\n⚠️  WARNING: Mixed results suggest potential race condition or timing issue")
	} else if hit1 && hit2 {
		fmt.Println("\n⚠️  WARNING: Both resolutions returned HIT - cache logic may be broken!")
	}
	
	// Final state check
	cache.mu.RLock()
	finalEntry, finalExists := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	
	if finalExists {
		now = time.Now()
		timestamp := finalEntry.Timestamp
		currentTTL := finalEntry.TTL
		timeSinceNow := now.Sub(timestamp)
		
		fmt.Println("\n6. Final cache state:")
		fmt.Printf("   IP: %s\n", finalEntry.IP)
		fmt.Printf("   TTL: %v\n", currentTTL)
		fmt.Printf("   Timestamp: %v\n", timestamp)
		fmt.Printf("   Time since creation: %v\n", timeSinceNow)
		fmt.Printf("   Is expired: %v (time.Since > TTL)\n", timeSinceNow > currentTTL)
		
		if hit1 && hit2 {
			fmt.Println("\n🔍 Analysis:")
			fmt.Println("  - Both calls returned HIT")
			fmt.Println("  - This means the cache has a valid entry at both times")
			fmt.Println("  - Either: (a) there's already cached data, or (b) TTL check is not working correctly")
		} else if hit1 && !hit2 {
			fmt.Println("\n🔍 Analysis:")
			fmt.Println("  - First call returned HIT when it should have been MISS")
			fmt.Println("  - This indicates there was already cached data (unexpected)")
		} else if !hit1 && hit2 {
			fmt.Println("\n🔍 Analysis:")
			fmt.Println("  - First call correctly returned MISS")
			fmt.Println("  - Second call returned HIT when it should have been MISS after expiration")
			fmt.Println("  - This suggests TTL check has a bug or race condition")
		} else {
			fmt.Println("\n🔍 Analysis:")
			fmt.Println("  - Both calls correctly returned MISS")
			fmt.Println("  - Cache is working as expected!")
		}
	} else {
		fmt.Println("\n❌ Final check failed: Entry doesn't exist in cache at the end")
	}
	
	fmt.Println("\n=== Debug Complete ===")
}