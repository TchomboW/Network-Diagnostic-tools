// utils/dns_cache_debug_entry_exist.go - Debug what happens when accessing map keys
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== DNS Cache Map Key Existence Debug ===\n")
	
	cache := NewDNSCache()
	testTTL := 50 * time.Millisecond
	cache.SetTTL(testTTL)
	
	// Let's trace through what happens when we check for a key that doesn't exist
	fmt.Println("1. Checking if \"8.8.8.8\" exists in empty cache:")
	cache.mu.RLock()
	entry, exists := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	
	fmt.Printf("   Returns: entry=%+v, exists=%v\n", entry, exists)
	fmt.Printf("   Type of entry: %T\n", entry)
	
	// Now let's see what happens when we actually access the fields
	fmt.Println("\n2. What happens when accessing fields of non-existent key:")
	now := time.Now()
	timestampField := entry.Timestamp  // This is a zero-time.Time
	durationField := entry.TTL         // This is a zero time.Duration
	
	fmt.Printf("   Timestamp field: %v (type: %T)\n", timestampField, timestampField)
	fmt.Printf("   TTL field:       %v (type: %T)\n", durationField, durationField)
	
	// Now let's see what happens when we try to compare these
	timeSince := now.Sub(timestampField)
	isExpired := timeSince > durationField
	
	fmt.Printf("\n   Time since zero-time: %v\n", timeSince)
	fmt.Printf("   TTL (zero):           %v\n", durationField)
	fmt.Printf("   Is expired? (time.Since > TTL): %v\n", isExpired)
	
	// Now let's see what happens with the full condition
	fullCondition := !exists || isExpired
	fmt.Printf("\n   Full condition (!exists || isExpired): %v\n", fullCondition)
	
	if !exists {
		fmt.Println("   -> Should go to MISS branch (entry doesn't exist)")
	} else if exists && isExpired {
		fmt.Println("   -> Should go to MISS branch (entry exists but expired)")
	} else if exists && !isExpired {
		fmt.Println("   -> Should go to HIT branch (entry exists and valid)")
	} else {
		fmt.Println("   -> UNEXPECTED STATE!")
	}
	
	// Now let's check what happens when we try to look up a key that DOES exist with zero values
	fmt.Println("\n3. What happens if there's already an entry with zero values:")
	cache.mu.Lock()
	varialZeroEntry := DNSCacheEntry{
		IP:        "",  // Empty IP
		TTL:       0,   // Zero duration
		Timestamp: time.Time{}, // Zero time
	}
	cache.entries["8.8.8.8"] = varialZeroEntry
	cache.mu.Unlock()
	
	fmt.Printf("   Now checking again:\n")
	cache.mu.RLock()
	entry2, exists2 := cache.entries["8.8.8.8"]
	cache.mu.RUnlock()
	
	fmt.Printf("   Returns: entry=%+v, exists=%v\n", entry2, exists2)
	
	// And now let's see what happens when we try to resolve this
	now = time.Now()
	timestampField2 := entry2.Timestamp  // This is still a zero-time.Time
	durationField2 := entry2.TTL         // This is still a zero time.Duration
	
	timeSince2 := now.Sub(timestampField2)
	isExpired2 := timeSince2 > durationField2
	
	fmt.Printf("\n   Time since zero-time: %v\n", timeSince2)
	fmt.Printf("   TTL (zero):           %v\n", durationField2)
	fmt.Printf("   Is expired? (time.Since > TTL): %v\n", isExpired2)
	
	fullCondition2 := !exists2 || isExpired2
	fmt.Printf("\n   Full condition (!exists || isExpired): %v\n", fullCondition2)
	
	if !exists2 {
		fmt.Println("   -> Should go to MISS branch (entry doesn't exist)")
	} else if exists2 && isExpired2 {
		fmt.Println("   -> Should go to MISS branch (entry exists but expired - CORRECT!)")
	} else if exists2 && !isExpired2 {
		fmt.Println("   -> Would go to HIT branch (but this shouldn't happen)")
	} else {
		fmt.Println("   -> UNEXPECTED STATE!")
	}
	
	// Now let's try the actual resolution and see what happens
	fmt.Println("\n4. Attempting actual resolution:")
	ip, hit := cache.Resolve("8.8.8.8")
	fmt.Printf("   Result: IP=%s, Hit=%v\n", ip, hit)
	
	if !hit {
		fmt.Println("✅ Got MISS as expected (entry was expired)")
	} else if hit && ip != "" {
		fmt.Println("❌ BUG: Got HIT when we should have gotten MISS!")
	} else if hit {
		fmt.Println("⚠️  Got HIT but no IP - something weird is happening")
	}
	
	fmt.Println("\n=== Debug Complete ===")
}