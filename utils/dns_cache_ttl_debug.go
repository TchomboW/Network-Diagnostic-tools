// utils/dns_cache_ttl_debug.go - Debug version for TTL expiration test
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== DNS Cache TTL Expiration Test ===\n")
	
	cache := NewDNSCache()
	testTTL := 50 * time.Millisecond
	cache.SetTTL(testTTL)
	
	fmt.Printf("1. Setting TTL to: %v\n", testTTL)
	
	// First resolution (should be miss and cache)
	fmt.Println("\n2. First DNS resolution:")
	ip1, hit1 := cache.Resolve("8.8.8.8")
	fmt.Printf("   IP: %s, Hit: %v, Expected: Miss (false)\n", ip1, hit1)
	
	// Check entry exists and is valid
	entry, exists := cache.GetEntry("8.8.8.8")
	if !exists {
		fmt.Println("\n❌ Entry not found after first resolution")
		return
	}
	fmt.Printf("   Entry TTL: %v\n", entry.TTL)
	fmt.Printf("   Entry Timestamp: %v\n", entry.Timestamp)
	
	// Calculate when it expires
	expirationTime := entry.Timestamp.Add(entry.TTL)
	now := time.Now()
	ttlRemaining := entry.TTL - now.Sub(entry.Timestamp)
	fmt.Printf("   TTL remaining: %v (expires at %v)\n", ttlRemaining, expirationTime)
	
	// Wait for TTL to expire plus some buffer
	fmt.Println("\n3. Waiting for TTL expiration...")
	time.Sleep(testTTL + 20*time.Millisecond)
	now = time.Now()
	ttlAfterWait := entry.TTL - now.Sub(entry.Timestamp)
	fmt.Printf("   Time waited: ~%v\n", testTTL+10*time.Millisecond)
	fmt.Printf("   TTL after wait: %v (should be negative)\n", ttlAfterWait)
	
	// Second resolution (should still be miss because entry expired)
	fmt.Println("\n4. Second DNS resolution (after TTL expiration):")
	ip2, hit2 := cache.Resolve("8.8.8.8")
	fmt.Printf("   IP: %s\n", ip2)
	fmt.Printf("   Hit: %v\n", hit2)
	
	// Check if this is a hit or miss
	if !hit2 {
		fmt.Println("\n✅ SUCCESS: Got cache MISS as expected after TTL expiration")
	} else {
		fmt.Println("\n❌ FAILURE: Expected cache MISS but got HIT")
	}
	
	// Verify IPs are the same (same hostname resolution)
	if ip1 == ip2 && ip1 != "" {
		fmt.Printf("✅ Both resolutions returned same IP: %s\n", ip1)
	} else if ip1 == "" || ip2 == "" {
		fmt.Println("\n❌ FAILURE: One or both DNS resolutions failed")
	} else {
		fmt.Printf("⚠️  Different IPs returned: %s vs %s (unexpected but not critical)\n", ip1, ip2)
	}
	
	// Check entry again after second resolution
	entry2, exists2 := cache.GetEntry("8.8.8.8")
	if !exists2 {
		fmt.Println("\n❌ Entry disappeared after second resolution")
		return
	}
	newTTLRemaining := entry2.TTL - time.Since(entry2.Timestamp)
	fmt.Printf("✅ New TTL remaining: %v\n", newTTLRemaining)
	
	fmt.Println("\n=== Test Summary ===")
	if !hit2 && ip1 != "" {
		fmt.Println("✅ All checks passed!")
	} else if hit2 {
		fmt.Println("❌ Main issue: Got cache HIT instead of MISS after TTL expiration")
	} else if ip1 == "" || ip2 == "" {
		fmt.Println("❌ DNS resolution failed in one or both attempts")
	} else {
		fmt.Println("⚠️  Minor issues detected but core functionality working")
	}
}