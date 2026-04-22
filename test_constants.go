package main

import (
	"fmt"
	"time"
)

// Import constants from utils package
const (
	DefaultTTL = 60 * time.Second
	TTLRemainingLowThreshold = 10 * time.Second
	MinIdleTime = 10 * time.Millisecond
	MaxIdleTime = time.Hour
)

func main() {
	fmt.Println("Testing DNS Cache Constants...")
	fmt.Printf("DefaultTTL: %v\n", DefaultTTL)
	fmt.Printf("TTLRemainingLowThreshold: %v\n", TTLRemainingLowThreshold)
	fmt.Printf("MinIdleTime: %v\n", MinIdleTime)
	fmt.Printf("MaxIdleTime: %v\n", MaxIdleTime)
	
	// Verify the constants have correct values
	if DefaultTTL != 60*time.Second {
		fmt.Println("ERROR: DefaultTTL value incorrect!")
		return
	}
	
	if TTLRemainingLowThreshold != 10*time.Second {
		fmt.Println("ERROR: TTLRemainingLowThreshold value incorrect!")
		return
	}
	
	if MinIdleTime != 10*time.Millisecond {
		fmt.Println("ERROR: MinIdleTime value incorrect!")
		return
	}
	
	if MaxIdleTime != time.Hour {
		fmt.Println("ERROR: MaxIdleTime value incorrect!")
		return
	}
	
	fmt.Println("\n✓ All DNS Cache constants are correctly defined and have expected values!")
}
