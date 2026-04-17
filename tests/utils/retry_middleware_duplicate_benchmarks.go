// utils/retry_middleware_duplicate_benchmarks.go - Contains only duplicate benchmark functions
package utils

import (
	"errors"
	"testing"
)

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

func BenchmarkRetryMiddlewareNoJitter(b *testing.B) {
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