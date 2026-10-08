#!/usr/bin/env python3
"""
Network Diagnostic Tool
======================
Performs real network diagnostics including ICMP ping, DNS resolution,
TCP connectivity, and HTTP request testing. Uses only Python standard library.

Usage: python network-diagnostic.py [target_host]
"""

import socket
import struct
import time
import select
import sys
import random
import urllib.request
import http.client


# ============================================================================
# ICMP Ping Implementation
# ============================================================================

ICMP_ECHO_REQUEST = 8
ICMP_ECHO_REPLY = 0
ICMP_TIMEOUT = 3

def icmp_checksum(data: bytes) -> int:
    """Compute Internet checksum for ICMP packets."""
    if len(data) % 2 == 1:
        data = data + b'\x00'
    
    total = 0
    for i in range(0, len(data), 2):
        word = (data[i] << 8) + data[i + 1]
        total += word
    
    # Fold carries
    while total >> 16:
        total = (total & 0xFFFF) + (total >> 16)
    
    # One's complement
    return ~total & 0xFFFF


def build_icmp_echo(ident: int, seq: int, payload: bytes = b'') -> bytes:
    """
    Build an ICMP ECHO request packet.
    
    ICMP header format:
    - 1 byte type
    - 1 byte code
    - 2 bytes checksum
    - 2 bytes identifier
    - 2 bytes sequence number
    - data (optional)
    """
    header = struct.pack('!BBHHH', ICMP_ECHO_REQUEST, 0, 0, ident, seq)
    checksum = icmp_checksum(header + payload)
    header = struct.pack('!BBHHH', ICMP_ECHO_REQUEST, 0, checksum, ident, seq)
    return header + payload


def recv_icmp_response(sock, sockname, ident: int, timeout: int = ICMP_TIMEOUT) -> tuple:
    """
    Wait for an ICMP ECHO reply with matching identifier.
    
    Returns (seq, round_trip_time, packet_len) or raises TimeoutError.
    """
    end = time.monotonic() + timeout
    
    while time.monotonic() < end:
        remaining = end - time.monotonic()
        if remaining <= 0:
            break
        
        rlist, _, _ = select.select([sock], [], [], remaining)
        
        if rlist:
            packet, sender = sock.recvfrom(1024)
            if sender[0] != sockname:
                continue
            
            # Parse IP header
            ip_header = packet[0:20]
            ip_version = (ip_header[0] & 0xF0) >> 4
            if ip_version != 4:
                continue
            
            # IP header length is in 4-byte units
            ip_header_length = (ip_header[0] & 0x0F) * 4
            
            # Check protocol is ICMP (1)
            if ip_header[9] != 1:
                continue
            
            # ICMP packet starts after IP header
            icmp_header = packet[ip_header_length:ip_header_length + 8]
            type, code, checksum, packet_ident, seq = struct.unpack('!BBHHH', icmp_header)
            
            if type == ICMP_ECHO_REPLY and packet_ident == ident:
                return seq, (time.monotonic() - start), len(packet)
    
    raise TimeoutError("ICMP response timed out")


def ping(host: str, count: int = 4) -> dict:
    """
    Perform ICMP ping to host with 'count' pings.
    Returns statistics dict.
    """
    print(f"ICMP Ping: {host}")
    print("-" * 60)
    
    # Resolve host once
    try:
        addr = socket.gethostbyname(host)
        print(f"Resolved to: {addr}")
    except socket.gaierror as e:
        print(f"DNS lookup failed: {e}")
        return {"success": False, "error": str(e)}
    
    sockname = socket.gethostname()
    
    # Create raw ICMP socket
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_ICMP)
        sock.bind(('', 0))
    except (PermissionError, OSError) as e:
        print(f"Could not create ICMP socket: {e}")
        print("(Try running with elevated privileges: sudo)")
        return {"success": False, "error": str(e)}
    
    ident = random.randint(0, 65535)
    results = []
    errors = 0
    
    for seq in range(count):
        try:
            packet = build_icmp_echo(ident, seq, b'Hello from Hermes')
            start = time.monotonic()
            
            sock.sendto(packet, (addr, 0))
            seq, rtt, packet_len = recv_icmp_response(sock, sockname, ident)
            
            rtt_ms = rtt * 1000
            results.append(rtt_ms)
            print(f"  Ping {seq + 1}/{count}: {packet_len} bytes from {addr}: icmp_seq={seq} time={rtt_ms:.2f} ms")
            
        except TimeoutError:
            print(f"  Ping {seq + 1}/{count}: Request timed out")
            errors += 1
        except Exception as e:
            print(f"  Ping {seq + 1}/{count}: Error: {e}")
            errors += 1
        finally:
            time.sleep(0.5)  # Wait between pings
    
    sock.close()
    
    # Calculate stats
    success_count = len(results)
    if success_count > 0:
        avg_rtt = sum(results) / success_count
        min_rtt = min(results)
        max_rtt = max(results)
        stats = {"pings_sent": count, "pings_received": success_count, "pings_lost": errors, "average_rtt_ms": avg_rtt, "min_rtt_ms": min_rtt, "max_rtt_ms": max_rtt}
    else:
        stats = {"pings_sent": count, "pings_received": 0, "pings_lost": errors}
    
    print("-" * 60)
    print(f"--- ICMP Ping Summary ---")
    print(f"  Pings sent/received: {stats['pings_sent']}/{stats['pings_received']}")
    if 'pings_lost' in stats:
        print(f"  Pings lost: {stats['pings_lost']}")
    if 'average_rtt_ms' in stats:
        print(f"  Average round-trip time: {stats['average_rtt_ms']:.2f} ms")
        print(f"  Min round-trip time: {stats['min_rtt_ms']:.2f} ms")
        print(f"  Max round-trip time: {stats['max_rtt_ms']:.2f} ms")
    print("-" * 60)
    print()
    
    return stats


# ============================================================================
# DNS Resolution
# ============================================================================

def test_dns_resolution(host: str) -> dict:
    """Measure DNS resolution time."""
    print(f"DNS Resolution: {host}")
    print("-" * 60)
    
    start = time.monotonic()
    try:
        result = socket.getaddrinfo(host, 443, socket.AF_INET, socket.SOCK_STREAM)
        elapsed_ms = (time.monotonic() - start) * 1000
        
        addr = result[0][4][0]
        print(f"  DNS lookup: SUCCESS ({elapsed_ms:.2f} ms)")
        print(f"  Address: {addr}")
        stats = {"success": True, "time_ms": elapsed_ms, "address": addr}
    except socket.gaierror as e:
        elapsed_ms = (time.monotonic() - start) * 1000
        print(f"  DNS lookup: FAILED ({elapsed_ms:.2f} ms)")
        print(f"  Error: {e}")
        stats = {"success": False, "time_ms": elapsed_ms, "error": str(e)}
    
    print("-" * 60)
    print()
    return stats


# ============================================================================
# TCP Port Connectivity
# ============================================================================

def test_tcp_connectivity(host: str, port: int = 443) -> dict:
    """Test TCP connection to specific port."""
    print(f"TCP Connectivity: {host}:{port}")
    print("-" * 60)
    
    conn = None
    try:
        start = time.monotonic()
        
        conn = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        conn.settimeout(5.0)
        conn.connect((host, port))
        
        elapsed_ms = (time.monotonic() - start) * 1000
        print(f"  TCP connection: SUCCESS ({elapsed_ms:.2f} ms)")
        stats = {"success": True, "time_ms": elapsed_ms}
    except socket.timeout:
        elapsed_ms = (time.monotonic() - start) * 1000
        print(f"  TCP connection: TIMED OUT ({elapsed_ms:.2f} ms)")
        stats = {"success": False, "time_ms": elapsed_ms}
    except ConnectionRefusedError:
        elapsed_ms = (time.monotonic() - start) * 1000
        print(f"  TCP connection: REFUSED ({elapsed_ms:.2f} ms)")
        stats = {"success": False, "time_ms": elapsed_ms}
    except OSError as e:
        elapsed_ms = (time.monotonic() - start) * 1000
        print(f"  TCP connection: ERROR ({elapsed_ms:.2f} ms)")
        print(f"  Error: {e}")
        stats = {"success": False, "time_ms": elapsed_ms, "error": str(e)}
    finally:
        if conn:
            conn.close()
    
    print("-" * 60)
    print()
    return stats


# ============================================================================
# HTTP Testing
# ============================================================================

def test_http_request(host: str, path: str = "/", timeout: int = 10) -> dict:
    """Test HTTP request and verify status."""
    print(f"HTTP Request Test: https://{host}{path}")
    print("-" * 60)
    
    try:
        conn = http.client.HTTPSConnection(host, timeout=timeout)
        
        start = time.monotonic()
        conn.request("GET", "/")
        response = conn.getresponse()
        elapsed_ms = (time.monotonic() - start) * 1000
        
        status = response.status
        # Treat 3xx redirects as success for connectivity purposes
        print(f"  Status code: {status} ({response.reason})")
        print(f"  Response time: {elapsed_ms:.2f} ms")
        
        # Check TLS info
        if hasattr(conn, 'sock') and conn.sock:
            try:
                cert = conn.sock.getpeercert()
                if cert:
                    issuer = cert['issuer'][1][1] if len(cert['issuer']) > 1 else "Unknown"
                    subject = cert['subject'][1][1] if len(cert['subject']) > 1 else "Unknown"
                    print(f"  TLS: Certificate from {issuer}")
                    print(f"  TLS: Subject: {subject}")
            except Exception:
                pass
        
        # Treat redirects (3xx) as success for connectivity testing
        success = 200 <= status < 400
        print(f"  Test result: {'PASS' if success else 'FAIL'}")
        
        conn.close()
        stats = {"success": success, "status": status, "time_ms": elapsed_ms}
    except urllib.error.HTTPError as e:
        elapsed_ms = (time.monotonic() - start) * 1000
        print(f"  HTTP Error: {e.code}")
        print(f"  Response time: {elapsed_ms:.2f} ms")
        stats = {"success": False, "status": e.code, "time_ms": elapsed_ms}
    except urllib.error.URLError as e:
        elapsed_ms = (time.monotonic() - start) * 1000
        print(f"  URL Error: {e}")
        print(f"  Response time: {elapsed_ms:.2f} ms")
        stats = {"success": False, "time_ms": elapsed_ms}
    except Exception as e:
        elapsed_ms = (time.monotonic() - start) * 1000
        print(f"  Error: {e}")
        print(f"  Response time: {elapsed_ms:.2f} ms")
        stats = {"success": False, "time_ms": elapsed_ms}
    
    print("-" * 60)
    print()
    return stats


# ============================================================================
# Main
# ============================================================================

def run_diagnostics(host: str):
    """Run all network diagnostics and generate report."""
    print("=" * 70)
    print(f"        NETWORK DIAGNOSTIC TOOL")
    print(f"        Target: {host}")
    print(f"        Date: {time.strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 70)
    print()
    
    all_pass = True
    
    # 1. ICMP Ping
    print("\n[1/4] ICMP Ping Test")
    ping_stats = ping(host, count=4)
    
    # 2. DNS Resolution
    print("\n[2/4] DNS Resolution Test")
    dns_stats = test_dns_resolution(host)
    if not dns_stats["success"]:
        all_pass = False
    
    # 3. TCP Connectivity
    print("\n[3/4] TCP Port Connectivity Test")
    tcp_stats = test_tcp_connectivity(host, port=443)
    if not tcp_stats["success"]:
        all_pass = False
    
    # 4. HTTP Request Test
    print("\n[4/4] HTTP Request Test")
    http_stats = test_http_request(host)
    if not http_stats["success"]:
        all_pass = False
    
    # Generate final report
    print("=" * 70)
    print(f"        DIAGNOSTIC REPORT")
    print(f"        Target: {host}")
    print("=" * 70)
    print()
    
    if "average_rtt_ms" in ping_stats:
        print(f"[ICMP] Average ping: {ping_stats['average_rtt_ms']:.2f} ms")
    else:
        print(f"[ICMP] FAILED (could not reach host via ICMP)")
        all_pass = False
    
    print(f"[DNS]  Resolution: {'PASS' if dns_stats['success'] else 'FAIL'} ({dns_stats['time_ms']:.2f} ms)")
    print(f"[TCP]  443 Port: {'PASS' if tcp_stats['success'] else 'FAIL'} ({tcp_stats['time_ms']:.2f} ms)")
    print(f"[HTTP] Request: {'PASS' if http_stats['success'] else 'FAIL'} (status: {http_stats.get('status', 'N/A')}, {http_stats['time_ms']:.2f} ms)")
    print()
    
    if all_pass:
        print("FINAL RESULT: All tests PASSED")
        print("Network connectivity to target is GOOD")
    else:
        print("FINAL RESULT: Some tests FAILED")
        print("Network connectivity to target has issues")
    
    print("=" * 70)
    
    return all_pass


if __name__ == "__main__":
    target = sys.argv[1] if len(sys.argv) > 1 else "google.com"
    run_diagnostics(target)