package networktool

import (
	"fmt"
	"math"
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
func isValidIPAddress(ip string) bool {
	ip = strings.TrimSpace(ip)
	
	// Check for IPv6 format with brackets [::1]
	if strings.HasPrefix(ip, "[") && strings.HasSuffix(ip, "]") {
		internalIP := ip[1 : len(ip)-1]
		return isIPv6Internal(internalIP) || isIPv4Internal(internalIP)
	}

	// Try IPv4 first
	if isIPv4Internal(ip) {
		return true
	}

	// Then try IPv6 (no brackets)
	return isIPv6Internal(ip)
}

func isIPv4Internal(ip string) bool {
	parts := strings.Split(ip, ".")
	
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		if !isValidIPv4Octet(part) {
			return false
		}
	}

	return true
}

func isIPv6Internal(ip string) bool {
	// Check for IPv6 format
	parts := strings.Split(ip, ":")
	
	if len(parts) < 2 || len(parts) > 8 {
		return false
	}

	for _, part := range parts {
		if part == "" {
			continue // Empty segments are allowed in IPv6 (:: notation)
		}
		
		if !isValidIPv6Segment(part) {
			return false
		}
	}

	return true
}

func isValidIPv4Octet(octet string) bool {
	// Check if it's a valid number between 0-255
	num, err := parseUnsignedInt(octet)
	if err != nil {
		return false
	}

	// Remove leading zeros (except for "0" itself)
	trimmedOctet := strings.TrimLeft(num, "0")
	if trimmedOctet == "" {
		trimmedOctet = "0"
	}

	// Re-parse to get the actual numeric value
	value := 0.0
	fmt.Sscanf(trimmedOctet, "%f", &value)
	return value >= 0 && value <= 255
}

func isValidIPv6Segment(segment string) bool {
	// IPv6 segments are hexadecimal
	if len(segment) == 0 || len(segment) > 4 {
		return false
	}

	for _, c := range segment {
		if !((c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || 
		     (c >= '0' && c <= '9')) {
			return false
		}
	}

	return true
}

func parseUnsignedInt(s string) (string, error) {
	if len(s) == 0 {
		return "", fmt.Errorf("empty string")
	}

	var result []byte
	
	for _, c := range s {
		switch c {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			result = append(result, byte(c))
		default:
			return "", fmt.Errorf("invalid character")
		}
	}

	return string(result), nil
}

// isValidFloat validates float values are reasonable for network speeds
func isValidFloat(value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	
	// Allow non-negative values (speeds can be zero)
	return value >= 0
}
