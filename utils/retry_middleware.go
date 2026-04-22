// utils/retry_middleware.go - Core retry middleware structure
package utils

import (
	"math/rand"
	"time"
)
type MiddlewareResult struct {
	Data  interface{} // The successful result or nil on failure
	Error error       // Any error that occurred during execution
}

// RetryMiddleware provides intelligent retry logic with exponential backoff for handling API rate limiting
type RetryMiddleware struct {
	MaxAttempts int           // Maximum number of retry attempts before giving up
	BaseDelay   time.Duration // Base delay between retry attempts (will increase exponentially)
}

// NewRetryMiddleware creates a new retry middleware with specified configuration.
// It automatically sets sensible defaults for invalid parameters.
func NewRetryMiddleware(maxAttempts int, delay time.Duration) *RetryMiddleware {
	// Set reasonable defaults for invalid inputs
	if maxAttempts <= 0 {
		maxAttempts = 3 // Default to 3 attempts if not specified or invalid
	}

	if delay <= 0 {
		delay = 100 * time.Millisecond // Default to 100ms base delay
	}

	return &RetryMiddleware{
		MaxAttempts: maxAttempts,
		BaseDelay:   delay,
	}
}

// Execute runs the provided function with intelligent retry logic for handling temporary failures.
// It automatically retries on common transient errors like timeouts and rate limiting, 
// using exponential backoff between attempts to avoid overwhelming APIs.
func (rm *RetryMiddleware) Execute(fn func() (interface{}, error)) MiddlewareResult {
	var lastError error

	for attempt := 0; attempt < rm.MaxAttempts; attempt++ {
		result, err := fn()

		if err == nil {
			return MiddlewareResult{Data: result} // Success - return immediately
		}

		lastError = err

		// Check if this is a retryable error (transient failures we can recover from)
		if !isRetryableError(err) {
			return MiddlewareResult{Error: err} // Permanent failure - fail immediately without retries
		}

		// Wait before next attempt with exponential backoff and jitter for safety
		if attempt < rm.MaxAttempts-1 {
			waitTime := rm.BaseDelay * time.Duration(attempt+1)

			// Add randomness (jitter) to prevent "thundering herd" problem where all clients retry simultaneously
			jitter := waitTime / 4 // Add up to 25% random variation
			totalWait := waitTime + time.Duration(float64(jitter)*float64(waitTime)*rand.Float64())

			time.Sleep(totalWait)
		}
	}

	return MiddlewareResult{Error: lastError} // All attempts exhausted - return final error
}

// isRetryableError determines if an error represents a transient failure that should trigger retry logic.
// It checks for common patterns like timeouts, connection issues, and rate limiting errors.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	// List of error patterns that indicate temporary failures suitable for retrying
	retryablePatterns := []string{
		"timeout",                          // Network timeout errors are typically transient
		"connection refused",               // Connection was reset - may succeed on retry
		"rate limited",                     // API rate limiting is often temporary
		"temporary failure",                // Explicit temporary error indicators
	}

	for _, pattern := range retryablePatterns {
		if contains(errMsg, pattern) {
			return true // Found a retryable error pattern
		}
	}

	return false // No known transient error patterns matched
}

// contains checks if a string contains another as a substring (case-sensitive).
// It handles empty strings and edge cases properly.
func contains(s, substr string) bool {
	// Empty search term matches anywhere
	if len(substr) == 0 {
		return true
	}

	// If haystack is shorter than needle, can't contain it
	if len(s) < len(substr) {
		return false
	}

	// Check if they're identical first (fast path for exact match)
	if s == substr {
		return true
	}

	// Otherwise, search through the string character by character
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true // Found a matching substring
		}
	}

	return false // No substring found
}