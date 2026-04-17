// utils/retry_middleware_test.go - Comprehensive tests for retry middleware (minimal duplicates)
package utils

import (
	"testing"
	"time"
)

func TestNewRetryMiddlewareCreatesWithDefaults(t *testing.T) {
	mw := NewRetryMiddleware(0, 0) // Both zero should use defaults
	
	if mw.MaxAttempts != 3 {
		t.Errorf("Expected default MaxAttempts of 3, got %d", mw.MaxAttempts)
	}
	
	expectedDelay := 100 * time.Millisecond
	if mw.BaseDelay != expectedDelay {
		t.Errorf("Expected default BaseDelay of %v, got %v", expectedDelay, mw.BaseDelay)
	}
}

func TestRetryMiddleware_ImmediateSuccess(t *testing.T) {
	mw := NewRetryMiddleware(3, 10*time.Millisecond)
	
	executionCount := 0
	
	result := mw.Execute(func() (interface{}, error) {
		executionCount++
		
		return "success", nil // Always succeeds immediately
	})
	
	if result.Data != "success" {
		t.Errorf("Expected success data, got %v", result.Data)
	}
	
	if result.Error != nil {
		t.Errorf("Expected no error for successful execution, got: %v", result.Error)
	}
	
	if executionCount != 1 {
		t.Errorf("Function should execute only once on immediate success, executed %d times", executionCount)
	}
}

func TestRetryMiddleware_FirstAttemptFailsThenSucceeds(t *testing.T) {
	mw := NewRetryMiddleware(3, 0) // No delay for faster tests
	
	attemptCount := 0
	
	result := mw.Execute(func() (interface{}, error) {
		attemptCount++
		
		if attemptCount == 1 {
			return nil, errors.New("retryable timeout") // First attempt fails with retryable error  
		}
		
		return "success", nil // Subsequent attempts succeed
	})
	
	if result.Data != "success" {
		t.Errorf("Expected success data after retry, got %v", result.Data)
	}
	
	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}
	
	if attemptCount != 2 {
		t.Errorf("Function should execute twice (1 failure + 1 retry), executed %d times", attemptCount)
	}
}

func TestRetryMiddleware_MaxAttemptsReached(t *testing.T) {
	mw := NewRetryMiddleware(2, 0) // Only 2 attempts allowed
	
	attemptCount := 0
	
	result := mw.Execute(func() (interface{}, error) {
		attemptCount++
		
		return nil, errors.New("persistent failure") // Always fails with non-retryable error
	})
	
	if result.Error == nil {
		t.Errorf("Expected error after all attempts exhausted, but got none")
	}
	
	if attemptCount != 2 {
		t.Errorf("Function should execute exactly %d times (matching MaxAttempts), executed %d times", 2, attemptCount)
	}
}

func BenchmarkRetryMiddlewareNoJitter(b *testing.B) {
	mw := NewRetryMiddleware(3, 10*time.Millisecond)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := mw.Execute(func() (interface{}, error) {
			return "success", nil // Always succeeds immediately  
		})
		
		if result.Data != "success" || result.Error != nil {
			b.Errorf("Expected success on immediate execution")
		}
	}
}

func BenchmarkRetryMiddlewareWithJitter(b *testing.B) {
	mw := NewRetryMiddleware(3, 10*time.Millisecond)
	
	failCounter := make(chan bool, b.N*2) // Buffer for all fail signals
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := mw.Execute(func() (interface{}, error) {
			select {
			case <-failCounter:
				return nil, errors.New("retryable failure") // First call fails  
			default:
				failCounter <- true
				return "success", nil // Subsequent calls succeed
			}
		})
		
		if result.Data != "success" {
			b.Errorf("Expected success after retry with jitter")
		}
	}
}

func BenchmarkRetryMiddlewareExponentialBackoff(b *testing.B) {
	mw := NewRetryMiddleware(3, 10*time.Millisecond) // Exponential backoff: 10ms, 20ms, 30ms
	
	failCounter := make(chan bool, b.N*2) // Buffer for all fail signals
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := mw.Execute(func() (interface{}, error) {
			select {
			case <-failCounter:
				return nil, errors.New("retryable failure") // First call fails  
			default:
				failCounter <- true
				return "success", nil // Subsequent calls succeed
			}
		})
		
		if result.Data != "success" {
			b.Errorf("Expected success after exponential backoff retry")
		}
	}
}

func BenchmarkRetryMiddlewareZeroDelay(b *testing.B) {
	mw := NewRetryMiddleware(3, 0) // No delay between attempts
	
	failCounter := make(chan bool, b.N*2) // Buffer for all fail signals
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := mw.Execute(func() (interface{}, error) {
			select {
			case <-failCounter:
				return nil, errors.New("retryable failure") // First call fails  
			default:
				failCounter <- true
				return "success", nil // Subsequent calls succeed
			}
		})
		
		if result.Data != "success" {
			b.Errorf("Expected success after retry with zero delay")
		}
	}
}

func BenchmarkRetryMiddlewareLargeDelay(b *testing.B) {
	mw := NewRetryMiddleware(3, 100*time.Millisecond) // Large base delay
	
	failCounter := make(chan bool, b.N*2) // Buffer for all fail signals
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := mw.Execute(func() (interface{}, error) {
			select {
			case <-failCounter:
				return nil, errors.New("retryable failure") // First call fails  
			default:
				failCounter <- true
				return "success", nil // Subsequent calls succeed
			}
		})
		
		if result.Data != "success" {
			b.Errorf("Expected success after retry with large delay")
		}
	}
}

func BenchmarkRetryMiddlewareMaxAttempts(b *testing.B) {
	mw := NewRetryMiddleware(5, 10*time.Millisecond) // 5 attempts
	
	failCounter := make(chan bool, b.N*6) // Buffer for all fail signals
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := mw.Execute(func() (interface{}, error) {
			select {
			case <-failCounter:
				return nil, errors.New("retryable failure") // Always fails so it reaches max attempts  
			default:
				failCounter <- true
				return "success", nil // Subsequent calls succeed
			}
		})
		
		if result.Error == nil {
			b.Errorf("Expected error after max attempts exceeded")
		}
	}
}

func BenchmarkRetryMiddlewareSuccess(b *testing.B) {
	mw := NewRetryMiddleware(3, 10*time.Millisecond)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := mw.Execute(func() (interface{}, error) {
			return "success", nil // Always succeeds immediately  
		})
		
		if result.Data != "success" || result.Error != nil {
			b.Errorf("Expected success on immediate execution")
		}
	}
}
