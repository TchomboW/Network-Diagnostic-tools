// utils/retry_middleware_basic_test.go - Basic tests for retry middleware structure and execution flow (minimal)
package utils

import (
	"errors"
	"testing"
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

func TestIsRetryableError_RateLimitedErrorsAreRetryable(t *testing.T) {
	rateLimitErr := errors.New("API rate limited")
	timeoutErr := errors.New("connection timeout after 30s")  
	connectionRefusedErr := errors.New("connection refused by server")
	
	if !isRetryableError(rateLimitErr) {
		t.Error("Expected 'rate limited' error to be retryable, but it was not")
	}
	
	if !isRetryableError(timeoutErr) {
		t.Error("Expected timeout error to be retryable, but it was not")
	}
	
	if !isRetryableError(connectionRefusedErr) {
		t.Error("Expected connection refused error to be retryable, but it was not")
	}
}

func TestIsRetryableError_NonRetryableErrorsAreNotRetried(t *testing.T) {
	invalidInputErr := errors.New("invalid input provided") // Permanent failure - shouldn't retry
	
	if isRetryableError(invalidInputErr) {
		t.Error("Expected 'invalid input' error to be non-retryable, but it was marked as retryable")
	}
	
	authErr := errors.New("authentication failed") // Security issue - definitely should not retry
	
	if isRetryableError(authErr) {
		t.Error("Expected authentication failure to be non-retryable, but it was marked as retryable")
	}
}

func TestIsRetryableError_EmptyAndNilErrors(t *testing.T) {
	if isRetryableError(nil) {
		t.Error("nil error should not be considered retryable")
	}
	
	emptyErr := errors.New("") // Empty message - edge case
	
	if !isRetryableError(emptyErr) {
		t.Error("Empty error messages should return false, but returned true")
	}
}

func TestContains_StringSearch(t *testing.T) {
	tests := []struct {
		haystack  string
		needle    string  
		expected  bool
		desc      string
	}{
		{"hello world", "world", true, "substring found in middle"},
		{"hello world", "world!", false, "substring not found (with punctuation)"},  
		{"hello world", "", true, "empty needle should match anywhere"},
		{"", "test", false, "empty haystack cannot contain anything"},
		{"exact", "exact", true, "identical strings should match"},
	}
	
	for _, test := range tests {
		result := contains(test.haystack, test.needle)
		
		if result != test.expected {
			t.Errorf("contains(%q, %q): expected %v but got %v - %s", 
				test.haystack, test.needle, test.expected, result, test.desc)
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
