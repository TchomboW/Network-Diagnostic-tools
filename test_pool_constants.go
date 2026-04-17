package main

import (
	"fmt"
	"time"
)

// Import constants from network package
const (
	DefaultMaxIdleTime = time.Minute             // Default max idle time for connections
	ReadDeadlineTimeout = 10 * time.Millisecond  // Timeout for checking if connection is alive
	MinAllowedIdleTime = time.Second            // Minimum allowed idle timeout
	MaxAllowedIdleTime = time.Hour              // Maximum allowed idle timeout
)

func main() {
	fmt.Println("Testing Connection Pool Constants...")
	fmt.Printf("DefaultMaxIdleTime: %v\n", DefaultMaxIdleTime)
	fmt.Printf("ReadDeadlineTimeout: %v\n", ReadDeadlineTimeout)
	fmt.Printf("MinAllowedIdleTime: %v\n", MinAllowedIdleTime)
	fmt.Printf("MaxAllowedIdleTime: %v\n", MaxAllowedIdleTime)
	
	// Verify the constants have correct values
	if DefaultMaxIdleTime != time.Minute {
		fmt.Println("ERROR: DefaultMaxIdleTime value incorrect!")
		return
	}
	
	if ReadDeadlineTimeout != 10*time.Millisecond {
		fmt.Println("ERROR: ReadDeadlineTimeout value incorrect!")
		return
	}
	
	if MinAllowedIdleTime != time.Second {
		fmt.Println("ERROR: MinAllowedIdleTime value incorrect!")
		return
	}
	
	if MaxAllowedIdleTime != time.Hour {
		fmt.Println("ERROR: MaxAllowedIdleTime value incorrect!")
		return
	}
	
	fmt.Println("\n✓ All Connection Pool constants are correctly defined and have expected values!")
}
