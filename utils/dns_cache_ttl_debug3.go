// utils/dns_cache_ttl_debug3.go - Simplified TTL check debugging
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== DNS Cache TTL Check Debug ===\n")
	
	cache := NewDNSCache()
	testTTL := 50 * time.Millisecond
	cache.SetTTL(testTTL)
	
	// First resolution
	fmt.Println("1. First resolution (should be MISS):")
	ip1, hit1 := cache.Resolve("8.8.8.8")
	fmt.Printf("   Result: IP=%s, Hit=%v (expected MISS)\n", ip1, hit1)
	if hit1 {
		fmt.Println("❌ First resolution should have been MISS but was HIT!")
		return
	}
	
	// Get entry details after first resolution
	entry, exists := cache.GetEntry("8.8.8.8")
	if !exists {
		fmt.Println("\n❌ Entry not found after first resolution")
		return
	}
	
	timestamp := entry.Timestamp
	currentTTL := entry.TTL
	now1 := time.Now()
	timeSince1 := now1.Sub(timestamp)
	
	fmt.Printf("\n2. After first resolution:\n")
	fmt.Printf("   Timestamp:    %v\n", timestamp)
	fmt.Printf("   TTL:          %v\n", currentTTL)
	fmt.Printf("   Now:          %v\n", now1)
	fmt.Printf("   Time since:   %v\n", timeSince1)
	fmt.Printf("   Is expired:   %v (time.Since > TTL)\n", timeSince1 > currentTTL)
	
	// Wait for TTL to expire
	fmt.Println("\n3. Waiting for TTL expiration...")
	time.Sleep(testTTL + 20 * time.Millisecond) // Wait longer than TTL
	
	now2 := time.Now()
	timestampAfterWait := entry.Timestamp
	currentTTLAfterWait := entry.TTL
	timeSince2 := now2.Sub(timestampAfterWait)
	
	fmt.Printf("   Now:          %v\n", now2)
	fmt.Printf("   Time since:   %v\n", timeSince2)
	fmt.Printf("   TTL remaining: %v (negative = expired)\n", currentTTLAfterWait-timeSince2)
	fmt.Printf("   Is expired:   %v (time.Since > TTL)\n", timeSince2 > currentTTLAfterWait)
	
	// Second resolution - should be MISS because entry is expired
	fmt.Println("\n4. Second resolution (after TTL expiration, should be MISS):")
	ip2, hit2 := cache.Resolve("8.8.8.8")
	fmt.Printf("   Result: IP=%s, Hit=%v\n", ip2, hit2)
	
	// Check what the entry looks like now
	entry2, exists2 := cache.GetEntry("8.8.8.8")
	if !exists2 {
		fmt.Println("\n❌ Entry not found after second resolution")
		return
	}
	
	timestampNow := entry2.Timestamp
	currentTTLNow := entry2.TTL
	now3 := time.Now()
	timeSince3 := now3.Sub(timestampNow)
	isExpiredNow := timeSince3 > currentTTLNow
	
	fmt.Printf("\n5. After second resolution:\n")
	fmt.Printf("   Timestamp:    %v\n", timestampNow)
	fmt.Printf("   TTL:          %v\n", currentTTLNow)
	fmt.Printf("   Time since:   %v\n", timeSince3)
	fmt.Printf("   Is expired:   %v (time.Since > TTL)\n", isExpiredNow)
	
	// Final analysis
	fmt.Println("\n=== Analysis ===")
	if !hit1 && hit2 {
		fmt.Println("❌ BUG DETECTED: Got MISS on first call, HIT on second call after expiration!")
		fmt.Println("This means the TTL check is not working correctly.")
		
		// Check if this might be a race condition or timing issue
		if timeSince2 > currentTTLAfterWait {
			fmt.Println("\n✅ Entry WAS actually expired (time.Since > TTL)")
			fmt.Println("❌ But cache returned HIT anyway - there's a bug in the cache logic!")
		} else {
			fmt.Println("Entry was NOT expired, so this might be expected behavior")
		}
		
	} else if !hit1 && !hit2 {
		fmt.Println("✅ Both resolutions correctly returned MISS (first fresh lookup, second after expiration)")
	} else if hit1 || !hit1 {
		if hit1 {
			fmt.Println("❌ First resolution should have been MISS but was HIT")
		} else {
			fmt.Println("⚠️  Second resolution might be returning MISS when it could return HIT due to caching issues")
		}
	}
	
	// Print detailed cache state
	fmt.Println("\n6. Current cache state:")
	stats := cache.GetStats()
	fmt.Printf("   Stats: %v\n", stats)
	
	if entry2 != nil {
		ttlRemaining := currentTTLNow - timeSince3
		status := ""
		if ttlRemaining < 0 {
			status = " [EXPIRED]"
		} else if ttlRemaining < 10*time.Second {
			status = " [LOW TTL]"
		}
		fmt.Printf("   Entry: %s -> %s%s (TTL remaining: %v)%s\n", 
			entry2.IP, entry2.IP, status, ttlRemaining, "")
	}
	
	fmt.Println("\n=== Debug Complete ===")
}