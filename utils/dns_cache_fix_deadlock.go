// utils/dns_cache_fix_deadlock.go - Debug and fix the deadlock in performLookup
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== DNS Cache Deadlock Debug ===\n")
	
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
	
	// Let's check what happens when we try to do the first resolution
	fmt.Println("\n2. Attempting first resolution:")
	fmt.Println("   This should trigger performLookup which has a deadlock issue...")
	
	// Let me manually trace through what happens:
	cache.mu.RLock()
	entryCheck, existsCheck := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	
	fmt.Printf("   Step 1: Lock acquired and entry checked: exists=%v\n", existsCheck)
	if !existsCheck {
		fmt.Println("   -> Should call performLookup (MISS branch)")
		
		// Now let's trace through what happens in performLookup:
		fmt.Println("\n   Step 2: Entering performLookup...")
		fmt.Println("     - Lock acquired for writing (performLookup uses Lock())")
		fmt.Println("     - DNS resolution performed")
		fmt.Println("     - Entry cached with new timestamp and TTL")
		fmt.Println("     - But then LOCK NOT RELEASED! DEADLOCK!")
		
		// Let's check if there are any locks still held
		cache.mu.RLock()
		cacheEntriesCount := len(cache.entries)
		cache.mu.RUnlock()
		fmt.Printf("\n   After attempt: Cache entries count: %d\n", cacheEntriesCount)
		
		// Try to do another read lock - this should deadlock if the write lock isn't released
		fmt.Println("\n   Attempting another lock (this will hang if there's a deadlock):")
		cache.mu.RLock()
		fmt.Println("   -> Got another read lock (no deadlock, so the issue is elsewhere)")
		cache.mu.RUnlock()
		
	} else {
		fmt.Println("   -> Entry already exists, should check TTL...")
		now := time.Now()
		timestamp := entryCheck.Timestamp
		currentTTL := entryCheck.TTL
		timeSince := now.Sub(timestamp)
		isExpired := timeSince > currentTTL
		
		fmt.Printf("   Timestamp: %v\n", timestamp)
		fmt.Printf("   TTL:       %v\n", currentTTL)
		fmt.Printf("   Time since creation: %v\n", timeSince)
		fmt.Printf("   Is expired? (time.Since > TTL): %v\n", isExpired)
		
		if !isExpired {
			fmt.Println("   -> Should go to HIT branch")
		} else {
			fmt.Println("   -> Should go to MISS branch (entry exists but expired)")
		}
	}
	
	// Now let's try a different approach - let me check if there might be a recursive lock issue
	fmt.Println("\n3. Checking for potential lock issues:")
	
	// Try to acquire a write lock immediately after an RLock
	cache.mu.RLock()
	fmt.Println("   Acquired read lock")
	
	// Now try to get a write lock while still holding the read lock (should cause deadlock)
	go func() {
		cache.mu.Lock()  // This should block because we already hold a read lock
		fmt.Println("   -> Got write lock (unexpected - no deadlock)")
		cache.mu.Unlock()
	}()
	
	time.Sleep(1 * time.Second) // Wait to see if the goroutine completes
	
	fmt.Println("\n4. Checking final cache state:")
	cache.mu.RLock()
	entryAfter, existsAfter := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	
	if !existsAfter {
		fmt.Println("   Entry doesn't exist in cache (but we tried to create it)")
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
	}
	
	fmt.Println("\n=== Debug Complete ===")
}