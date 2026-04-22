// utils/dns_cache_debug.go - Debug version to see actual cache.String() output
package main

import "fmt"

func main() {
	cache := NewDNSCache()
	
	// Test empty cache string representation
	fmt.Println("=== EMPTY CACHE ===")
	str1 := cache.String()
	fmt.Printf("Empty cache String(): '%s'\n", str1)
	expected := "DNS Cache Contents:\n  (empty)"
	fmt.Printf("Expected: '%s'\n", expected)
	fmt.Printf("Match: %v\n\n", str1 == expected)
	
	// Test with an entry added
	cache.Resolve("8.8.8.8")
	fmt.Println("=== CACHE WITH ENTRY ===")
	str2 := cache.String()
	fmt.Printf("Cache String(): '%s'\n", str2)
	fmt.Printf("Contains 'DNS Cache Contents:': %v\n", containsSubstring(str2, "DNS Cache Contents:"))
	
	// Test the substring function used in tests
	fmt.Println("\n=== SUBSTRING TEST ===")
	testStr := "DNS Cache Contents:\n  8.8.8.8 -> 8.8.8.8 (TTL: 60s)"
	fmt.Printf("Test string: '%s'\n", testStr)
	fmt.Printf("Contains 'DNS Cache Contents:': %v\n", containsSubstring(testStr, "DNS Cache Contents:"))
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAfterSubstring(s, substr))
}

func containsAfterSubstring(s, substr string) bool {
	for i := 1; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}