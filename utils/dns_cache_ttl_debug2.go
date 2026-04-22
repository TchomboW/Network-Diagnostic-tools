// utils/dns_cache_ttl_debug2.go - More detailed TTL check debugging
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== Detailed DNS Cache TTL Check Debug ===\n")
	
	cache := NewDNSCache()
	testTTL := 50 * time.Millisecond
	cache.SetTTL(testTTL)
	
	// First resolution (should be miss and cache)
	fmt.Println("1. First resolution:")
	ip1, hit1 := cache.Resolve("8.8.8.8")
	fmt.Printf("   Result: IP=%s, Hit=%v\n", ip1, hit1)
	
	// Get detailed entry information
	entry, exists := cache.GetEntry("8.8.8.8")
	if !exists {
		fmt.Println("\n❌ Entry not found after first resolution")
		return
	}
	
	now := time.Now()
	timestamp := entry.Timestamp
	currentTTL := entry.TTL
	
	fmt.Printf("\n2. Entry details:\n")
	fmt.Printf("   Timestamp: %v\n", timestamp)
	fmt.Printf("   TTL:       %v\n", currentTTL)
	fmt.Printf("   Now:      %v\n", now)
	
	// Calculate when the entry should expire
	expireTime := timestamp.Add(currentTTL)
	fmt.Printf("   Expires at:  %v\n", expireTime)
	
	// Check if expired NOW (at time.Now())
	timeSince := now.Sub(timestamp)
	isExpiredNow := timeSince > currentTTL
	fmt.Printf("\n3. Current expiration check:\n")
	fmt.Printf("   Time since creation: %v\n", timeSince)
	fmt.Printf("   TTL:                 %v\n", currentTTL)
	fmt.Printf("   Is expired (time.Since > TTL): %v\n", isExpiredNow)
	
	// Now wait for TTL to expire
	fmt.Println("\n4. Waiting for TTL expiration...")
	time.Sleep(testTTL + 20 * time.Millisecond)
	now = time.Now()
	
	// Recalculate after waiting
	timestampAfterWait := entry.Timestamp
	currentTTLAfterWait := entry.TTL
	expireTimeAfterWait := timestamp.AfterWait().Add(currentTTLAfterWait)
	timeSinceAfterWait := now.Sub(timestampAfterWait)
	isExpiredAfterWait := timeSinceAfterWait > currentTTLAfterWait
	
	fmt.Printf("   Timestamp:    %v\n", timestampAfterWait)
	fmt.Printf("   TTL:          %v\n", currentTTLAfterWait)
	fmt.Printf("   Now (after wait):     %v\n", now)
	fmt.Printf("   Expires at:  %v\n", expireTimeAfterWait)
	fmt.Printf("   Time since creation:    %v\n", timeSinceAfterWait)
	fmt.Printf("   Is expired:             %v\n", isExpiredAfterWait)
	
	// Now do second resolution (should be miss because expired)
	fmt.Println("\n5. Second resolution (after TTL expiration):")
	ip2, hit2 := cache.Resolve("8.8.8.8")
	fmt.Printf("   Result: IP=%s, Hit=%v\n", ip2, hit2)
	
	// Check what the actual entry looks like now
	entry2, exists2 := cache.GetEntry("8.8.8.8")
	if !exists2 {
		fmt.Println("\n❌ Entry not found after second resolution")
	} else {
		now = time.Now()
		timestampNow := entry2.Timestamp
		currentTTLNow := entry2.TTL
		timeSinceNow := now.Sub(timestampNow)
		isExpiredNowAfterSecond := timeSinceNow > currentTTLNow
		
		fmt.Printf("\n6. Entry details after second resolution:\n")
		fmt.Printf("   Timestamp: %v\n", timestampNow)
		fmt.Printf("   TTL:       %v\n", currentTTLNow)
		fmt.Printf("   Now:      %v\n", now)
		fmt.Printf("   Time since creation:    %v\n", timeSinceNow)
		fmt.Printf("   Is expired (time.Since > TTL): %v\n", isExpiredNowAfterSecond)
		
		fmt.Println("\n=== Summary ===")
		if hit1 {
			fmt.Println("❌ First resolution should have been MISS but was HIT")
		} else if !hit2 {
			fmt.Println("✅ Second resolution correctly returned MISS after TTL expiration")
		} else {
			fmt.Println("❌ Second resolution incorrectly returned HIT (should be MISS)")
		}
		
		if isExpiredAfterWait && hit2 {
			fmt.Println("⚠️  Entry was expired but cache still returned a HIT - BUG DETECTED!")
		}
	}
}

// Helper to get timestamp after waiting
func (t *time.Time) AfterWait() time.Time {
	time.Sleep(20 * time.Millisecond) // Add some delay for the "waiting" simulation
	return time.Now().Add(-15 * time.Millisecond) // Return a slightly older time to simulate where we were when we started waiting
}