package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIPv4Validation verifies valid IPv4 addresses are accepted
func TestIPv4Validation(t *testing.T) {
	t.Run("valid IPv4", func(t *testing.T) {
		assert.True(t, isValidIPAddress("192.168.1.1"))
		assert.True(t, isValidIPAddress("10.0.0.1"))
		assert.True(t, isValidIPAddress("255.255.255.255"))
		assert.True(t, isValidIPAddress("0.0.0.0"))
	})
	
	t.Run("IPv4 with leading zeros", func(t *testing.T) {
		assert.True(t, isValidIPAddress("192.168.001.001")) // Should be accepted by net.ParseIP
	})
}

// TestInvalidIPv4Validation verifies malformed IPv4 addresses are rejected
func TestInvalidIPv4Validation(t *testing.T) {
	t.Run("incomplete IPv4", func(t *testing.T) {
		assert.False(t, isValidIPAddress("192.168.1"))      // Missing last octet
		assert.False(t, isValidIPAddress("192.168."))       // Empty parts
	})
	
	t.Run("invalid IPv4 format", func(t *testing.T) {
		assert.False(t, isValidIPAddress("300.168.1.1"))    // Invalid octet value
		assert.False(t, isValidIPAddress("192.168.1.256"))  // Out of range
		assert.False(t, isValidIPAddress("192.168.1.-1"))   // Negative value
	})
	
	t.Run("IPv4 with non-numeric characters", func(t *testing.T) {
		assert.False(t, isValidIPAddress("192.168.a.1"))    // Non-numeric octet
		assert.False(t, isValidIPAddress("192.168.1.x"))    // Non-numeric last octet
	})
	
	t.Run("IPv4 with too many parts", func(t *testing.T) {
		assert.False(t, isValidIPAddress("192.168.1.1.1"))  // Too many octets
		assert.False(t, isValidIPAddress("192.168.1.1.1.1"))// Way too many
	})
}

// TestIPv6Validation verifies valid IPv6 addresses are accepted
func TestIPv6Validation(t *testing.T) {
	t.Run("full IPv6 format", func(t *testing.T) {
		assert.True(t, isValidIPAddress("2001:0db8:85a3:0000:0000:8a2e:0370:7334"))
	})
	
	t.Run("IPv6 with leading zeros omitted", func(t *testing.T) {
		assert.True(t, isValidIPAddress("2001:db8:85a3::8a2e:370:7334"))
	})
	
	t.Run("IPv6 with :: compression at end", func(t *testing.T) {
		assert.True(t, isValidIPAddress("2001:db8::"))
	})
	
	t.Run("IPv6 with :: compression at start", func(t *testing.T) {
		assert.True(t, isValidIPAddress("::1"))  // Loopback
		assert.True(t, isValidIPAddress("::ffff:192.168.1.1"))  // IPv4-mapped
	})
	
	t.Run("IPv6 with :: compression in middle", func(t *testing.T) {
		assert.True(t, isValidIPAddress("2001:db8::8a2e:370:7334"))
	})
	
	t.Run("IPv6 loopback", func(t *testing.T) {
		assert.True(t, isValidIPAddress("::1"))
	})
}

// TestInvalidIPv6Validation verifies malformed IPv6 addresses are rejected (including the bug fix case)
func TestInvalidIPv6Validation(t *testing.T) {
	t.Run("malformed IPv6 - incomplete format", func(t *testing.T) {
		assert.False(t, isValidIPAddress("192.168.1"))       // This is the specific bug that was fixed
		assert.False(t, isValidIPAddress("192.168."))        // Missing parts after colon
		assert.False(t, isValidIPAddress(":168:1"))          // Invalid start
	})
	
	t.Run("IPv6 with invalid hex characters", func(t *testing.T) {
		assert.False(t, isValidIPAddress("2001:0db8:85a3:ghjk:0000:8a2e:0370:7334"))  // Invalid chars
		assert.False(t, isValidIPAddress("2001:0db8:85a3:0000:0000:8a2z:0370:7334"))  // z is not hex
	})
	
	t.Run("IPv6 with parts too long", func(t *testing.T) {
		assert.False(t, isValidIPAddress("2001:0db8:85a3:00000:0000:8a2e:0370:7334")) // 5-digit part (max is 4)
		assert.False(t, isValidIPAddress("gggg:0db8:85a3:0000:0000:8a2e:0370:7334")) // g is not hex and too long
	})
	
	t.Run("IPv6 with parts that are empty", func(t *testing.T) {
		assert.False(t, isValidIPAddress("2001:0db8:::85a3:0000:0000:8a2e:0370:7334")) // Empty part after double colon (except at boundaries)
		assert.False(t, isValidIPAddress(":0db8:85a3:0000:0000:8a2e:0370:7334"))       // Empty start
	})
	
	t.Run("IPv6 with too many colons", func(t *testing.T) {
		assert.False(t, isValidIPAddress("2001::db8::85a3:0000:0000:8a2e:0370:7334")) // Too many :: (only one allowed)
		assert.False(t, isValidIPAddress("2001:0db8:85a3:0000:0000:8a2e:0370:7334:1")) // 9 parts - too many groups
	})
	
	t.Run("IPv6 with invalid characters in parts", func(t *testing.T) {
		assert.False(t, isValidIPAddress("2001:0db8:85a3:ghjk:0000:8a2e:0370:7334"))  // Non-hex chars
		assert.False(t, isValidIPAddress("2001:0db8:85a3:0000:0000:8a2e:037g:7334"))  // g is not hex
	})
	
	t.Run("IPv6 with negative or out-of-range values", func(t *testing.T) {
		assert.False(t, isValidIPAddress("2001:-db8:85a3:0000:0000:8a2e:0370:7334")) // Negative value not allowed in IPv6 hex notation
	})
	
	t.Run("IPv6 with leading zeros that are too long", func(t *testing.T) {
		assert.False(t, isValidIPAddress("2001:0db8:85a3:00000:0000:8a2e:0370:7334")) // 5-digit hex part (max is 4 per RFC 4291)
	})
}

// TestLocalhostValidation verifies localhost and loopback addresses are accepted
func TestLocalhostValidation(t *testing.T) {
	t.Run("localhost", func(t *testing.T) {
		assert.True(t, isValidIPAddress("localhost"))
	})
	
	t.Run("127.0.0.1", func(t *testing.T) {
		assert.True(t, isValidIPAddress("127.0.0.1"))
	})
}

// TestEmptyAndWhitespaceValidation verifies empty strings and whitespace are rejected
func TestEmptyAndWhitespaceValidation(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		assert.False(t, isValidIPAddress(""))
	})
	
	t.Run("whitespace only", func(t *testing.T) {
		assert.False(t, isValidIPAddress("   "))
		assert.False(t, isValidIPAddress("\t"))
		assert.False(t, isValidIPAddress("\n"))
	})
	
	t.Run("leading/trailing whitespace that becomes empty after trim", func(t *testing.T) {
		assert.False(t, isValidIPAddress("  \t\n"))
	})
}

// TestMixedIPv4IPv6Validation verifies IPv4-mapped IPv6 addresses are handled correctly
func TestMixedIPv4IPv6Validation(t *testing.T) {
	t.Run("IPv4-mapped IPv6 address", func(t *testing.T) {
		assert.True(t, isValidIPAddress("::ffff:192.168.1.1")) // Valid IPv4-mapped IPv6
	})
}

// TestEdgeCaseIPv6Validation verifies various edge cases for IPv6 format validation
func TestEdgeCaseIPv6Validation(t *testing.T) {
	t.Run("IPv6 with :: in the middle but without proper structure", func(t *testing.T) {
		assert.False(t, isValidIPAddress("2001::db8:85a3:0000:0000:8a2e:0370")) // Too many groups after ::
	})
	
	t.Run("IPv6 with leading zeros that should be valid", func(t *testing.T) {
		assert.True(t, isValidIPAddress("2001:0db8:85a3:0000:0000:8a2e:0370:7334")) // All zeros are valid
	})
	
	t.Run("IPv6 with uppercase and lowercase mixed", func(t *testing.T) {
		assert.True(t, isValidIPAddress("2001:DB8:85A3:0:0:8a2e:370:7334")) // Uppercase is valid
		assert.True(t, isValidIPAddress("2001:db8:85a3:0:0:8a2e:370:7334")) // Lowercase is valid
		assert.True(t, isValidIPAddress("2001:Db8:85A3:0:0:8a2E:370:7334")) // Mixed case is valid
	})
	
	t.Run("IPv6 with :: at very start", func(t *testing.T) {
		assert.True(t, isValidIPAddress("::1"))      // Loopback
		assert.True(t, isValidIPAddress("::"))       // All zeros
	})
	
	t.Run("IPv6 with :: at very end", func(t *testing.T) {
		assert.True(t, isValidIPAddress("2001:db8::"))  // Compressed trailing zeros
	})
}

// TestComprehensiveIPv4Validation verifies various IPv4 formats are correctly accepted/rejected
func TestComprehensiveIPv4Validation(t *testing.T) {
	t.Run("valid private addresses", func(t *testing.T) {
		assert.True(t, isValidIPAddress("10.0.0.1"))
		assert.True(t, isValidIPAddress("172.16.0.1"))
		assert.True(t, isValidIPAddress("192.168.1.1"))
	})
	
	t.Run("valid public addresses", func(t *testing.T) {
		assert.True(t, isValidIPAddress("8.8.8.8"))       // Google DNS
		assert.True(t, isValidIPAddress("1.1.1.1"))       // Cloudflare DNS
		assert.True(t, isValidIPAddress("208.67.222.222"))// OpenDNS
	})
	
	t.Run("valid broadcast address", func(t *testing.T) {
		assert.True(t, isValidIPAddress("255.255.255.255"))
	})
	
	t.Run("invalid IPv4 - out of range octets", func(t *testing.T) {
		assert.False(t, isValidIPAddress("256.1.1.1"))    // First octet too large
		assert.False(t, isValidIPAddress("192.300.1.1"))  // Second octet out of range
		assert.False(t, isValidIPAddress("192.168.256.1"))// Third octet out of range
		assert.False(t, isValidIPAddress("192.168.1.300"))// Fourth octet out of range
	})
	
	t.Run("invalid IPv4 - negative values", func(t *testing.T) {
		assert.False(t, isValidIPAddress("-1.1.1.1"))     // Negative first octet
		assert.False(t, isValidIPAddress("192.-1.1.1"))   // Negative second octet
	})
	
	t.Run("invalid IPv4 - non-numeric characters", func(t *testing.T) {
		assert.False(t, isValidIPAddress("1a.1.1.1"))     // Non-digit in first part
		assert.False(t, isValidIPAddress("192.a.1.1"))    // Non-digit in second part
		assert.False(t, isValidIPAddress("192.168.x.1"))  // Non-digit in third part
	})
}
