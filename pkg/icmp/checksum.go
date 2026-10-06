package icmp

// Checksum computes the 16-bit Internet checksum over the given data.
// The data passed should have bytes 2 and 3 (the checksum field) set to zero.
func Checksum(data []byte) uint16 {
	var sum uint32 = 0
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(data[i])<<8 | uint32(data[i+1])
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}

	for sum >> 16 != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	return uint16(^sum)
}
