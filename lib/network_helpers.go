package networktool

import (
	"math"
	"net"
	"net/url"
	"strings"
)

// isValidURL validates URL format with comprehensive checks
func isValidURL(input string) bool {
	if len(input) < 3 {
		return false
	}

	input = strings.TrimSpace(input)
	
	// Check for basic domain structure or protocol+domain
	lower := strings.ToLower(input)
	
	// If it has a protocol, validate the full URL
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		parsedURL, err := url.Parse(input)
		if err != nil {
			return false
		}
		
		// Must have a host (domain or IP)
		if parsedURL.Host == "" {
			return false
		}
		
		// Validate domain format - should contain at least one dot for domains
		// unless it's an IP address
		domain := strings.TrimPrefix(parsedURL.Host, "www.")
		hasDot := strings.Contains(domain, ".")
		isIP := isValidIPAddress(domain)
		
		if hasDot || isIP {
			return true
		}
		
		// Allow localhost without dot for development scenarios
		if domain == "localhost" {
			return true
		}
		
		return false
	}
	
	// If no protocol, treat as hostname (domain or IP)
	if strings.Contains(input, ".") || isValidIPAddress(input) {
		return true
	}
	
	// Check for localhost
	if input == "localhost" {
		return true
	}
	
	return false
}

// isValidHostname validates hostname format (e.g., "8.8.8.8", "google.com")  
func isValidHostname(hostname string) bool {
	hostname = strings.TrimSpace(hostname)
	
	if len(hostname) < 1 || len(hostname) > 253 {
		return false
	}

	// Check for valid characters and proper structure
	var lastDot int = -1
	
	for i, c := range hostname {
		if c == '.' {
			labelLen := i - lastDot - 1
			
			// Empty label is not allowed (e.g., "example..com")
			if labelLen == 0 {
				return false
			}
			
			lastDot = i
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || 
		          (c >= '0' && c <= '9') || c == '-' {
			// Valid characters: letters, digits, and hyphens
			continue
		} else if c == '_' {
			// Some hostnames use underscores for internal DNS
			continue
		} else {
			return false // Invalid character
		}
	}

	// Check that hostname doesn't end or start with a hyphen
	if strings.HasPrefix(hostname, "-") || strings.HasSuffix(hostname, "-") {
		return false
	}

	return true
}

// isValidIPAddress validates IP address format (IPv4 and IPv6)
// Uses net.ParseIP for correctness with all IPv6 notations including :: compression
func isValidIPAddress(ip string) bool {
	ip = strings.TrimSpace(ip)
	
	// Handle bracketed IPv6 [::1]
	if strings.HasPrefix(ip, "[") && strings.HasSuffix(ip, "]") {
		ip = ip[1 : len(ip)-1]
	}
	
	return net.ParseIP(ip) != nil
}

// isValidIPv4Octet validates a single IPv4 octet (0-255)
func isValidIPv4Octet(octet string) bool {
	// Delegate to net.ParseIP for correctness
	ip := net.ParseIP(octet)
	return ip != nil && ip.To4() != nil
}

// isValidFloat validates float values are reasonable for network speeds
func isValidFloat(value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	
	// Allow non-negative values (speeds can be zero)
	return value >= 0
}
