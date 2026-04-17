// utils/dns_cache_debug_resolve2.go - Debug specific to the return value issue (FINAL FIXED)
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== DNS Cache Resolve Return Value Debug ===\n")
	
	cache := NewDNSCache()
	testTTL := 50 * time.Millisecond
	cache.SetTTL(testTTL)
	
	// Check if "8.8.8.8" exists before any resolution
	fmt.Println("1. Checking initial state:")
	cache.mu.RLock()
	entry, exists := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	
	fmt.Printf("   \"8.8.8.8\" exists initially: %v\n", exists)
	if !exists {
		fmt.Println("   (Entry doesn't exist - this is expected)")
	} else {
		fmt.Printf("   Entry data: %+v\n", entry)
	}
	
	// Now manually trace what happens when we try to check the condition
	fmt.Println("\n2. Manual trace of Resolve logic:")
	cache.mu.RLock()
	entryCheck, existsCheck := cache.entries["8.8.8.8"]
	fmt.Printf("   After lock: entry found=%v\n", existsCheck)
	
	// If doesn't exist, we should get a MISS
	if !existsCheck {
		fmt.Println("   -> Should go to MISS branch (entry doesn't exist)")
		
		// But what happens with the condition check?
		now := time.Now()
		timestampZero := entryCheck.Timestamp  // This is zero-time
		durationZero := entryCheck.TTL         // This is zero duration
		
		fmt.Printf("   Entry.Check: Timestamp=%v, TTL=%v\n", timestampZero, durationZero)
		
		timeSinceZeroTime := now.Sub(timestampZero)
		expiredCondition := timeSinceZeroTime > durationZero
		
		fmt.Printf("   Now: %v\n", now)
		fmt.Printf("   Time since zero-time: %v\n", timeSinceZeroTime)
		fmt.Printf("   TTL (zero): %v\n", durationZero)
		fmt.Printf("   Is expired? (time.Since > TTL): %v\n", expiredCondition)
		
		// The full condition is: !exists || time.Since(entry.Timestamp) > entry.TTL
		fullCondition := !existsCheck || expiredCondition
		fmt.Printf("   Full condition (!exists || isExpired): %v\n", fullCondition)
		
		if fullCondition {
			fmt.Println("   -> Would go to MISS branch (correct)")
		} else {
			fmt.Println("   -> Would go to HIT branch (WRONG!)")
		}
	} else if existsCheck {
		// If it exists, check the TTL condition
		now := time.Now()
		timestamp := entryCheck.Timestamp
		duration := entryCheck.TTL
		
		fmt.Printf("   After lock: entry found=%v\n", existsCheck)
		fmt.Printf("   Entry.Check: Timestamp=%v, TTL=%v\n", timestamp, duration)
		
		now = time.Now()
		timestampNow := now.Sub(timestamp)
		isExpiredNow := timestampNow > duration
		
		fmt.Printf("   Now: %v\n", now)
		fmt.Printf("   Time since creation: %v\n", timestampNow)
		fmt.Printf("   Is expired? (time.Since > TTL): %v\n", isExpiredNow)
		
		fullCondition := !existsCheck || isExpiredNow
		fmt.Printf("   Full condition (!exists || isExpired): %v\n", fullCondition)
		
		if !fullCondition {
			fmt.Println("   -> Would go to HIT branch (entry exists and not expired)")
		} else {
			fmt.Println("   -> Would go to MISS branch (entry exists but expired)")
		}
	}
	
	// Now let's actually try the resolution
	fmt.Println("\n3. Attempting actual resolution:")
	ip, hit := cache.Resolve("8.8.8.8")
	fmt.Printf("   Result: IP=%s, Hit=%v\n", ip, hit)
	
	// Check what happened to the cache state after resolution
	cache.mu.RLock()
	entryAfter, existsAfter := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	
	fmt.Printf("\n4. After resolution:\n")
	fmt.Printf("   Entry exists: %v\n", existsAfter)
	if !existsAfter {
		fmt.Println("❌ BUG: Entry doesn't exist after resolution!")
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
		
		if !hit && existsAfter {
			fmt.Println("\n⚠️  WARNING: Got MISS but entry was created - this shouldn't happen!")
		} else if hit && !existsAfter {
			fmt.Println("❌ BUG: Got HIT but entry doesn't exist!")
		} else if hit && existsAfter && isExpiredNow {
			fmt.Println("\n⚠️  WARNING: Got HIT when entry was expired - TTL check has bug!")
		} else if hit && !existsAfter || hit && existsAfter && !isExpiredNow {
			fmt.Println("✅ Normal behavior for cache HIT")
		} else if !hit && existsAfter && isExpiredNow {
			fmt.Println("❌ BUG: Got MISS when entry was expired - should be HIT!")
		}
	}
	
	fmt.Println("\n=== Debug Complete ===")
}