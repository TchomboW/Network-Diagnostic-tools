// utils/dns_cache_debug_resolve.go - Debug the Resolve function specifically (FINAL FIXED 2)
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== DNS Cache Resolve Function Debug ===\n")
	
	cache := NewDNSCache()
	testTTL := 50 * time.Millisecond
	cache.SetTTL(testTTL)
	
	// Check initial state
	fmt.Println("1. Initial cache state:")
	cache.mu.RLock()
	initialCount := len(cache.entries)
	cache.mu.RUnlock()
	fmt.Printf("   Cache entries: %d\n", initialCount)
	
	// Try to resolve a hostname that doesn't exist yet
	fmt.Println("\n2. Attempting first resolution (should be MISS):")
	ip1, hit1 := cache.Resolve("8.8.8.8")
	fmt.Printf("   Result: IP=%s, Hit=%v\n", ip1, hit1)
	
	// Check the entry right after resolution
	fmt.Println("\n3. Checking what happened:")
	cache.mu.RLock()
	entryAfterResolve, existsAfter := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	fmt.Printf("   Entry exists: %v\n", existsAfter)
	if !existsAfter {
		fmt.Println("❌ Entry doesn't exist but we got an IP!")
		return
	}
	
	now := time.Now()
	timestamp := entryAfterResolve.Timestamp
	currentTTL := entryAfterResolve.TTL
	timeSince := now.Sub(timestamp)
	
	fmt.Printf("   Timestamp: %v\n", timestamp)
	fmt.Printf("   TTL:       %v\n", currentTTL)
	fmt.Printf("   Time since creation: %v\n", timeSince)
	fmt.Printf("   Is expired: %v (time.Since > TTL)\n", timeSince > currentTTL)
	
	// Now let's manually trace through what should have happened
	fmt.Println("\n4. Manual trace of Resolve logic:")
	fmt.Println("   Step 1: Look up '8.8.8.8' in entries map...")
	cache.mu.RLock()
	entry, exists := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	
	fmt.Printf("   Step 2: Check existence - exists=%v\n", exists)
	if !exists {
		fmt.Println("   -> Should have been MISS (entry doesn't exist)")
	} else {
		fmt.Println("   -> Entry exists, checking TTL...")
		
		now = time.Now()
		timestampCheck := entry.Timestamp
		currentTLLOnceMore := entry.TTL
		timeSinceOnceMore := now.Sub(timestampCheck)
		isExpired := timeSinceOnceMore > currentTLLOnceMore
		
		fmt.Printf("   Step 3: Check TTL - is_expired=%v (time.Since=%v, TTL=%v)\n", 
			isExpired, timeSinceOnceMore, currentTLLOnceMore)
		
		if !isExpired {
			fmt.Println("   -> Should have been HIT (entry exists and not expired)")
		} else {
			fmt.Println("   -> Should have been MISS (entry exists but expired)")
		}
	}
	
	// Now let's check what the actual condition was at resolution time
	fmt.Println("\n5. Checking when the entry was actually created:")
	cache.mu.RLock()
	entryAtCheck, existsAtCheck := cache.entries["8.8.8.8"]
	if existsAtCheck {
		timestampActual := entryAtCheck.Timestamp
		currentTTLActual := entryAtCheck.TTL
		nowActually := time.Now()
		timeSinceActually := nowActually.Sub(timestampActual)
		
		fmt.Printf("   Actual creation timestamp: %v\n", timestampActual)
		fmt.Printf("   TTL at check: %v\n", currentTTLActual)
		fmt.Printf("   Time since now: %v\n", timeSinceActually)
		expiredAtCheck := timeSinceActually > currentTTLActual
		fmt.Printf("   Is expired at this moment: %v (time.Since > TTL)\n", 
			expiredAtCheck)
		
		if !existsAtCheck {
			fmt.Println("\n❌ Entry doesn't exist at check - why did we get a HIT?")
		} else if existsAtCheck && timeSinceActually > currentTTLActual {
			fmt.Println("✅ Entry was expired, so MISS is correct")
		} else if !existsAtCheck {
			fmt.Println("❌ Entry doesn't exist - this shouldn't happen in this branch!")
		} else if existsAtCheck && timeSinceActually <= currentTTLActual {
			fmt.Println("✅ Entry exists and NOT expired - should be HIT but we expected MISS")
			fmt.Println("\nThis suggests the entry was created BEFORE our check started!")
			fmt.Println("Maybe there's some initialization or previous execution happening?")
		} else if !existsAtCheck && timeSinceActually > currentTTLActual {
			fmt.Println("Entry was expired, so MISS is correct")
		}
	}
	
	// Final summary
	fmt.Println("\n=== Summary ===")
	if hit1 && (!existsAfter || existsAfter) {
		fmt.Println("❌ BUG: Got HIT when we should have gotten MISS")
		fmt.Println("This means the cache has a valid entry even though it shouldn't exist yet.")
	} else if hit1 {
		fmt.Println("⚠️  Unclear why we got HIT - need more investigation")
	} else {
		fmt.Println("✅ First resolution correctly returned MISS")
	}
	
	if !hit1 && existsAfter {
		fmt.Println("❌ Another bug: Got MISS but entry was created anyway")
		fmt.Println("This suggests the TTL check logic has a problem.")
	}
	
	fmt.Println("\n=== Debug Complete ===")
}