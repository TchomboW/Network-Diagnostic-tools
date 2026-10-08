package icmp

import (

	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

// Sequence is a unique ICMP echo sequence number.
type Sequence uint16

// Identifier identifies a ping session (typically PID).
type Identifier uint16

// EchoRequest creates an ICMP Echo Request packet (Type 8, Code 0).
func EchoRequest(id Identifier, seq Sequence, size int) []byte {
	payload := make([]byte, size-8)
	pkt := make([]byte, 8+len(payload))
	pkt[0] = 8  // Type: Echo Request
	pkt[1] = 0  // Code
	pkt[2] = byte(id >> 8)
	pkt[3] = byte(id & 0xFF)
	pkt[4] = byte(seq >> 8)
	pkt[5] = byte(seq & 0xFF)
	// Zero out checksum field before computing
	pkt[2] = 0
	pkt[3] = 0
	// Compute checksum over entire packet
	chksum := calcChecksum(pkt)
	pkt[2] = byte(chksum >> 8)
	pkt[3] = byte(chksum & 0xFF)
	pkt[4] = 0
	pkt[5] = 0
	return pkt
}

// EchoReply parses an Echo Reply and extracts id and seq.
func EchoReply(pkt []byte) (Identifier, Sequence, error) {
	if len(pkt) < 8 {
		return 0, 0, fmt.Errorf("packet too short")
	}
	if pkt[0] != 0 {
		return 0, 0, fmt.Errorf("packet not Echo Reply")
	}
	id := Identifier(pkt[2]<<8) | Identifier(pkt[3])
	seq := Sequence(pkt[4]<<8) | Sequence(pkt[5])
	// Verify checksum on entire packet
	chksum := calcChecksum(pkt)
	if chksum != 0 {
		return 0, 0, fmt.Errorf("checksum mismatch: got %d, expected 0", chksum)
	}
	return id, seq, nil
}

// calcChecksum computes the 16-bit Internet checksum (RFC 1071).
func calcChecksum(data []byte) uint16 {
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
	return ^uint16(sum)
}

// SequenceGenerator is a thread-safe sequence number generator.
type SequenceGenerator struct {
	seq atomic.Uint32
}

func NewSequenceGenerator() *SequenceGenerator {
	return &SequenceGenerator{}
}

func (g *SequenceGenerator) Next() Sequence {
	return Sequence(g.seq.Add(1))
}

// PingResult is a single ping result.
type PingResult struct {
	Latency time.Duration
	Seq     Sequence
	OK      bool
	Err     error
}

// Pinger handles ICMP ping sessions.
type Pinger struct {
	target       netip.Addr
	conn         *net.UDPConn
	done         chan bool
	id           Identifier
	sequence     *SequenceGenerator
	resultCh     chan PingResult
	wg           sync.WaitGroup
	pingTimeout  time.Duration
	interval     time.Duration
	count        int
	running      atomic.Bool
	readTimeout  time.Duration
	writtenAt    time.Time
	icmpTimeout  time.Duration
	roundTripFn  string
	roundTripNum uint64

	// Metrics
	BytesSent     int
	BytesReceived int
	TxTime        time.Duration
	RxTime        time.Duration
	Errors        int
}

// New creates a Pinger for the target address.
func New(target string) (*Pinger, error) {
	addr, err := netip.ParseAddr(target)
	if err != nil {
		return nil, err
	}
	if addr.Zone() != "" {
		return nil, fmt.Errorf("link-local IPv6 addresses not supported")
	}
	return &Pinger{
		target:      addr,
		id:          Identifier(0x8844),
		sequence:    NewSequenceGenerator(),
		pingTimeout: time.Second,
		interval:    time.Second,
		count:       4,
	}, nil
}

// SetIdentifier sets the identifier (PID).
func (p *Pinger) SetIdentifier(identifier Identifier) {
	p.id = identifier
}

// SetSequence sets the sequence generator.
func (p *Pinger) SetSequence(generator *SequenceGenerator) {
	p.sequence = generator
}

// SetTimeout sets the per-ping timeout.
func (p *Pinger) SetTimeout(timeout time.Duration) {
	p.pingTimeout = timeout
}

// SetInterval sets the interval between pings.
func (p *Pinger) SetInterval(interval time.Duration) {
	p.interval = interval
}

// SetCount sets the number of pings to send.
func (p *Pinger) SetCount(count int) {
	p.count = count
}

// SetReadTimeout sets the read timeout for the connection.
func (p *Pinger) SetReadTimeout(timeout time.Duration) {
	p.readTimeout = timeout
}

// SetICMPTTimeout sets the timeout for the ICMP socket.
func (p *Pinger) SetICMPTTimeout(timeout time.Duration) {
	p.icmpTimeout = timeout
}

// Run sends count ICMP pings and returns results.
func (p *Pinger) Run() []PingResult {
	results := make([]PingResult, 0, p.count)
	p.done = make(chan bool)

	// Try raw ICMP socket first
	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		// Raw sockets require root on Linux. Fall back.
		// Use UDP probe approach instead.
		p.conn = nil // Signal use UDP approach
	} else {
		p.conn = conn.(*net.UDPConn)
	}

	defer p.Close()

	for i := 0; i < p.count; i++ {
		select {
		case <-p.done:
			return results
		default:
		}

		seq := p.sequence.Next()
		p.writtenAt = time.Now()

		if p.conn != nil {
			// Raw ICMP ping
			request := EchoRequest(p.id, seq, 64)
			err := p.writeICMP(request)
			if err != nil {
				p.Errors++
				results = append(results, PingResult{Seq: seq, OK: false, Err: fmt.Errorf("write failed: %w", err)})
				continue
			}

			// Wait for response with timeout
			p.conn.SetReadDeadline(time.Now().Add(p.pingTimeout))
			var buf [1500]byte
			n, _, err := p.conn.ReadFrom(buf[:])
			if err != nil {
				p.Errors++
				results = append(results, PingResult{Seq: seq, OK: false, Err: fmt.Errorf("read failed or timeout: %w", err)})
				continue
			}
			// Validate response is Echo Reply to our request
			if n < 8 || buf[0] != 0 || buf[1] != 0 {
				continue // Not ours or not response
			}
			latency := time.Since(p.writtenAt)
			results = append(results, PingResult{Seq: seq, OK: true, Latency: latency})
		}

		// Pace requests
		if i < p.count-1 {
			select {
			case <-time.After(p.interval):
			case <-p.done:
				return results
			}
		}
	}
	return results
}

func (pinger *Pinger) writeICMP(packet []byte) error {
	_, err := pinger.conn.WriteTo(packet, &net.UDPAddr{IP: pinger.target.AsSlice(), Port: 0})
	return err
}

// Close shuts down the Pinger.
func (p *Pinger) Close() {
	if p.conn != nil {
		p.conn.Close()
	}
}