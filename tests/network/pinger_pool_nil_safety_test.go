package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNilPoolGetConnection verifies that calling GetConnection on a nil pool returns error without panic
func TestNilPoolGetConnection(t *testing.T) {
	var nilPool *ICMPConnectionPool
	
	// Should return error without panicking
	conn, err := nilPool.GetConnection()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "connection pool not initialized")
	assert.Nil(t, conn)
}

// TestNilConnectionsSlice verifies that accessing GetConnection when connections slice is nil returns error
func TestNilConnectionsSliceGetConnection(t *testing.T) {
	// Create a pool but simulate uninitialized state by setting connections to nil after creation
	pool := NewICMPConnectionPool(5, time.Minute)
	
	// Simulate uninitialized state by directly accessing the field (this tests defensive coding)
	// In practice, this would be caught earlier, but we're testing the defensive check exists
	pool.connections = nil
	
	conn, err := pool.GetConnection()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no connections available in pool")
	assert.Nil(t, conn)
}

// TestConcurrentGetConnectionAfterNilSafety verifies thread-safe access after nil safety checks
func TestConcurrentGetConnectionAfterNilSafety(t *testing.T) {
	pool := NewICMPConnectionPool(5, time.Hour)
	
	// Simulate concurrent access where one goroutine might encounter a nil state
	var poolToUse = pool  // Capture variable for closure
	
	go func() {
		// Normal usage should work fine
		conn, err := pool.GetConnection()
		assert.Nil(t, err)
		if conn != nil && conn.Conn != nil {
			conn.Close()
		}
		pool.ReturnConnection(conn)
	}()
	
	go func() {
		// Simulate concurrent access pattern
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				conn, err := pool.GetConnection()
				if err != nil {
					t.Logf("Goroutine %d got expected error: %v", id, err)
					return
				}
				pool.ReturnConnection(conn)
			}(i)
		}
		wg.Wait()
	}()
	
	time.Sleep(100 * time.Millisecond)  // Give goroutines time to run
	
	// Verify pool is still in valid state (no panic occurred)
	assert.NotNil(t, pool)
	stats := pool.GetPoolStats()
	assert.NotNil(t, stats["total_connections"])
}

// TestNilConnectionInSlice verifies that nil connections in the pool are properly handled
func TestNilConnectionInSliceGetConnection(t *testing.T) {
	pool := NewICMPConnectionPool(3, time.Hour)
	
	// Manually create a slice with nil entries to simulate partial initialization
	pool.connections = make([]*ICMPConnection, 3)
	
	conn, err := pool.GetConnection()
	assert.NotNil(t, conn)
	assert.Nil(t, err)
}

// TestGetConnectionWithInvalidPoolState verifies various invalid states return appropriate errors
func TestGetConnectionWithInvalidPoolState(t *testing.T) {
	t.Run("nil pool", func(t *testing.T) {
		var nilPool *ICMPConnectionPool
		conn, err := nilPool.GetConnection()
		assert.NotNil(t, err)
		assert.Nil(t, conn)
	})
	
	t.Run("connections slice is nil", func(t *testing.T) {
		pool := NewICMPConnectionPool(5, time.Minute)
		pool.connections = nil
		conn, err := pool.GetConnection()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "no connections available")
		assert.Nil(t, conn)
	})
	
	t.Run("pool completely exhausted", func(t *testing.T) {
		pool := NewICMPConnectionPool(1, time.Hour)
		
		// Use the only connection and ensure it can't be reused immediately
		conn, err := pool.GetConnection()
		assert.Nil(t, err)
		
		// Close the connection so it becomes unavailable
		if conn.Conn != nil {
			conn.Close()
		}
		
		// Try to get a connection again - should fail since we can't create more than 1
		time.Sleep(50 * time.Millisecond) // Give some time for cleanup
		
		conn, err = pool.GetConnection()
		assert.NotNil(t, err)
		assert.Nil(t, conn)
	})
}

// TestNilPoolReturnConnection verifies that returning a connection on nil pool doesn't panic
func TestNilPoolReturnConnection(t *testing.T) {
	var nilPool *ICMPConnectionPool
	
	conn := createMockConnection(nilPool, 1)
	
	// Should not panic even though pool is nil
	assert.NotPanics(t, func() {
		nilPool.ReturnConnection(conn)
	})
}

// TestConcurrentGetAndReturnAfterNilFix verifies thread safety of Get/Return after nil safety fixes
func TestConcurrentGetAndReturnAfterNilFix(t *testing.T) {
	pool := NewICMPConnectionPool(5, time.Hour)
	
	var wg sync.WaitGroup
	numIterations := 100
	
	// Launch concurrent get and return operations
	for i := 0; i < numIterations; i++ {
		wg.Add(2)
		
		go func() {
			defer wg.Done()
			conn, err := pool.GetConnection()
			assert.Nil(t, err)
			if conn != nil && conn.Conn != nil {
				conn.Close()
			}
		}()
		
		go func() {
			defer wg.Done()
			pool.ReturnConnection(createMockConnection(pool, 0))
		}()
	}
	
	wg.Wait()
	
	// Verify pool is in consistent state after concurrent operations
	assert.NotNil(t, pool)
	stats := pool.GetPoolStats()
	assert.NotNil(t, stats["total_connections"])
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
				assert.NotNil(t, id) // Just to use the parameter
				assert.NotNil(t, err)
				assert.Nil(t, conn)
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
		assert.NotNil(t, err)
		assert.Nil(t, conn)
	})
	
	t.Run("mixed nil and valid connections", func(t *testing.T) {
		pool := NewICMPConnectionPool(3, time.Hour)
		
		// Create some connections but leave gaps (nil entries)
		conn1 := createMockConnection(pool, 0)
		pool.ReturnConnection(conn1)
		
		// Get a connection - should work despite potential nil gaps in future iterations
		conn2, err := pool.GetConnection()
		assert.Nil(t, err)
		assert.NotNil(t, conn2)
		
		// Return it back to pool for reuse testing
		pool.ReturnConnection(conn2)
	})
}
