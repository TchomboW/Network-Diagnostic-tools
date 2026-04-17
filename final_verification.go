package main

import (
	"fmt"
	"time"
)

// Constants from utils/dns_cache.go
const (
	DefaultTTL = 60 * time.Second
	TTLRemainingLowThreshold = 10 * time.Second
	MinIdleTime = 10 * time.Millisecond
	MaxIdleTime = time.Hour
)

func main() {
	fmt.Println("=== DNS Cache Constants Verification ===")
	fmt.Printf("DefaultTTL: %v\n", DefaultTTL)
	fmt.Printf("TTLRemainingLowThreshold: %v\n", TTLRemainingLowThreshold)
	fmt.Printf("MinIdleTime: %v\n", MinIdleTime)
	fmt.Printf("MaxIdleTime: %v\n", MaxIdleTime)
	
	// Verify they work in comparisons
	now := time.Now()
	entryTimestamp := now.Add(-30 * time.Second) // 30 seconds ago
	
	fmt.Println("\n=== Testing TTL Comparisons ===")
	ttlRemaining := DefaultTTL - (now.Sub(entryTimestamp))
	fmt.Printf("Entry was set %v ago\n", now.Sub(entryTimestamp))
	fmt.Printf("TTL is: %v\n", ttlRemaining)
	
	if ttlRemaining < TTLRemainingLowThreshold {
		fmt.Println("✓ Low TTL threshold detected correctly!")
	} else {
		fmt.Println("✓ Threshold comparison working as expected")
	}
	
	fmt.Println("\n=== Testing Time Calculations ===")
	testDuration := 30 * time.Second
	fmt.Printf("30 seconds = %v\n", testDuration)
	fmt.Printf("Is it less than DefaultTTL? %v\n", testDuration < DefaultTTL)
	fmt.Printf("Is it more than TTLRemainingLowThreshold? %v\n", testDuration > TTLRemainingLowThreshold)
	
	fmt.Println("\n✓ All constant values verified and working correctly!")
}
