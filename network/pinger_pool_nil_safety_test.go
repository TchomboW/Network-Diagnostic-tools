package network

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// createMockConnection creates a mock ICMP connection for testing purposes
func createMockConnection(pool *ICMPConnectionPool, id int) *ICMPConnection {
	conn := NewICMPConnection(nil, id)
	return conn
}

// TestNilPoolGetConnection verifies that calling GetConnection on a nil pool returns error without panic
func TestNilPoolGetConnection(t *testing.T) {
	var nilPool *ICMPConnectionPool
	
	conn, err := nilPool.GetConnection()
	if err == nil {
		t.Fatal("Expected error when getting connection from nil pool")
	}
	if conn != nil {
		t.Fatalf("Expected nil connection but got: %v", conn)
	}
	fmt.Printf("[PASS] TestNilPoolGetConnection: Error correctly returned - %s\n", err.Error())
}

// TestNilConnectionsSlice verifies that accessing GetConnection when connections slice is nil returns error
func TestNilConnectionsSliceGetConnection(t *testing.T) {
	pool := NewICMPConnectionPool(5, time.Minute)
	pool.connections = nil
	
	conn, err := pool.GetConnection()
	if err == nil {
		t.Fatal("Expected error when getting connection from pool with nil connections slice")
	}
	if conn != nil {
		t.Fatalf("Expected nil connection but got: %v", conn)
	}
	fmt.Printf("[PASS] TestNilConnectionsSliceGetConnection: Error correctly returned - %s\n", err.Error())
}

// TestConcurrentAccessAfterNilSafety verifies thread-safe access after nil safety checks
func TestConcurrentAccessAfterNilSafety(t *testing.T) {
	pool := NewICMPConnectionPool(5, time.Hour)
	
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := pool.GetConnection()
			if err != nil {
				fmt.Printf("[FAIL] Goroutine %d got error: %v\n", id, err)
				return
			}
			pool.ReturnConnection(conn)
		}(i)
	}
	wg.Wait()
	
	if pool == nil {
		t.Fatal("Pool was nil after concurrent operations")
	}
	stats := pool.GetPoolStats()
	if stats["total_connections"] == nil {
		t.Fatal("Total connections not available in stats")
	}
	fmt.Printf("[PASS] TestConcurrentAccessAfterNilSafety: Pool survived concurrent access\n")
}

// TestInvalidIPv6Format verifies that the bug fix for IPv6 validation works correctly
func TestInvalidIPv6Format(t *testing.T) {
	// This is the specific case from the bug report - should be rejected now
	result := isValidIPAddress("192.168.1")
	if result {
		t.Fatal("Expected '192.168.1' to be invalid but it was accepted")
	}
	fmt.Printf("[PASS] TestInvalidIPv6Format: Malformed IPv4/IPv6 correctly rejected\n")
	
	// Other malformed cases that should also fail
	if isValidIPAddress("192.168.") {
		t.Fatal("Expected '192.168.' to be invalid")
	}
	if isValidIPAddress(":168:1") {
		t.Fatal("Expected ':168:1' to be invalid")
	}
	
	fmt.Println("[PASS] TestInvalidIPv6Format: Additional malformed IPv4/IPv6 correctly rejected\n")
}

// TestConcurrentGetAndReturnAfterNilFix verifies thread-safe access after nil safety fixes
func TestConcurrentGetAndReturnAfterNilFix(t *testing.T) {
	pool := NewICMPConnectionPool(5, time.Hour)
	
	var wg sync.WaitGroup
	numIterations := 10
	
	for i := 0; i < numIterations; i++ {
		wg.Add(2)
		
		go func() {
			defer wg.Done()
			conn, err := pool.GetConnection()
			if err != nil {
				fmt.Printf("[FAIL] Get connection error: %v\n", err)
				return
			}
			pool.ReturnConnection(conn)
		}()
		
		go func() {
			defer wg.Done()
			pool.ReturnConnection(createMockConnection(pool, 0))
		}()
	}
	
	wg.Wait()
	if pool == nil {
		t.Fatal("Pool was nil after concurrent operations")
	}
	stats := pool.GetPoolStats()
	if stats["total_connections"] == nil {
		t.Fatal("Total connections not available in stats")
	}
	fmt.Println("[PASS] TestConcurrentGetAndReturnAfterNilFix: Pool survived concurrent Get/Return\n")
}

// TestNilPoolReturnConnection verifies that returning a connection on nil pool doesn't panic
func TestNilPoolReturnConnection(t *testing.T) {
	var nilPool *ICMPConnectionPool
	
	conn := createMockConnection(nilPool, 1)
	
	// Should not panic even though pool is nil
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Panic occurred when returning connection to nil pool: %v", r)
		}
	}()
	nilPool.ReturnConnection(conn)
	fmt.Println("[PASS] TestNilPoolReturnConnection: No panic when returning connection to nil pool\n")
}

// TestValidIPv6Validation verifies valid IPv6 addresses are accepted
func TestValidIPv6Validation(t *testing.T) {
	validIpv6s := []string{
		"2001:0db8:85a3:0000:0000:8a2e:0370:7334",  // Full format
		"2001:db8::8a2e:370:7334",                    // With :: compression
		"2001:db8::",                                 // Trailing ::
		"::1",                                        // Loopback
	}
	
	for _, ipv6 := range validIpv6s {
		if !isValidIPAddress(ipv6) {
			t.Fatalf("Expected '%s' to be valid IPv6 but it was rejected", ipv6)
		}
	}
	fmt.Printf("[PASS] TestValidIPv6Validation: All valid IPv6 addresses accepted\n")
}

// TestConcurrentAccessAfterNilSafety2 verifies thread-safe access after nil safety checks
func TestConcurrentAccessAfterNilSafety2(t *testing.T) {
	var nilPool *ICMPConnectionPool
	
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := nilPool.GetConnection()
			if conn != nil {
				t.Fatalf("Goroutine %d: Expected nil connection but got: %v", id, conn)
			}
			if err == nil {
				t.Errorf("Goroutine %d: Expected error from nil pool but got none", id)
			} else {
				fmt.Printf("[PASS] Goroutine %d: Correctly rejected with error - %s\n", id, err.Error())
			}
		}(i)
	}
	wg.Wait()
}

// TestNilSafetyEdgeCases verifies various edge cases for nil safety
func TestNilSafetyEdgeCases(t *testing.T) {
	t.Run("nil pool with multiple goroutines", func(t *testing.T) {
		var nilPool *ICMPConnectionPool
		
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				conn, err := nilPool.GetConnection()
				if conn != nil {
					t.Errorf("Goroutine %d: Expected nil connection but got: %v", id, conn)
				}
				if err == nil {
					t.Errorf("Goroutine %d: Expected error from nil pool but got none", id)
				} else {
					fmt.Printf("[PASS] Goroutine %d: Correctly rejected - %s\n", id, err.Error())
				}
			}(i)
		}
		wg.Wait()
	})
	
	t.Run("nil connections after initialization", func(t *testing.T) {
		pool := NewICMPConnectionPool(5, time.Minute)
		
		// Simulate a race condition where connections get cleared
		pool.mu.Lock()
		pool.connections = nil
		pool.mu.Unlock()
		
		conn, err := pool.GetConnection()
		if conn != nil {
			t.Fatalf("Expected nil connection but got: %v", conn)
		}
		if err == nil {
			t.Fatal("Expected error when connections slice is nil")
		}
		fmt.Printf("[PASS] TestNilSafetyEdgeCases:nil connections after initialization\n")
	})
	
	t.Run("mixed nil and valid connections", func(t *testing.T) {
		pool := NewICMPConnectionPool(3, time.Hour)
		
		// Create some connections but leave gaps (nil entries)
		conn1 := createMockConnection(pool, 0)
		pool.ReturnConnection(conn1)
		
		// Get a connection - should work despite potential nil gaps in future iterations
		conn2, err := pool.GetConnection()
		if err != nil {
			t.Fatalf("Expected to get connection but got error: %v", err)
		}
		if conn2 == nil {
			t.Fatal("Expected non-nil connection")
		}
		
		// Return it back to pool for reuse testing
		pool.ReturnConnection(conn2)
		fmt.Printf("[PASS] TestNilSafetyEdgeCases:mixed nil and valid connections\n")
	})
}
