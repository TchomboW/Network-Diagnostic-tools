#!/usr/bin/env python3
"""Network Diagnostic CLI - Pure Standard Library."""
import argparse
import socket
import struct
import time
import os
import sys
import subprocess

ICMP_TYPE_ECHO_REQUEST = 8
ICMP_TYPE_ECHO_REPLY = 0
DEFAULT_TIMEOUT = 5
DEFAULT_COUNT = 5
DEFAULT_INTERVAL = 2

def calculate_checksum(data):
    if len(data) % 2 == 1:
        data += b'\x00'
    s = 0
    for i in range(0, len(data), 2):
        w = (data[i] << 8) + data[i+1]
        s += w
    s = (s >> 16) + (s & 0xffff)
    s += s >> 16
    return ~s & 0xffff

def ping(host, count=DEFAULT_COUNT, interval=DEFAULT_INTERVAL):
    print(f"\n{'='*30} PING {'='*30}")
    try:
        ip_addr = socket.gethostbyname(host)
    except socket.gaierror as e:
        print(f"DNS resolution failed for {host}: {e}")
        return False

    print(f"Pinging {host} [{ip_addr}]...")

    use_raw_icmp = False
    icmp_sock = None
    # Attempt raw ICMP (requires root/sudo on most systems)
    try:
        icmp_sock = socket.socket(socket.AF_INET, socket.SOCK_RAW, socket.getprotobyname("icmp"))
        use_raw_icmp = True
    except PermissionError:
        pass
    except Exception:
        pass

    if not use_raw_icmp:
        print("[INFO] Running as TCP probe (raw ICMP requires root)")
    
    successful = 0
    for i in range(count):
        start = time.time()
        if use_raw_icmp and icmp_sock:
            # Construct ICMP packet
            header = struct.pack("!BBHHH", ICMP_TYPE_ECHO_REQUEST, 0, 0, 1, 1)
            body = b'\x41' * 64
            checksum = calculate_checksum(header + body)
            header = struct.pack("!BBHHH", ICMP_TYPE_ECHO_REQUEST, 0, checksum, 1, 1)
            packet = header + body
            icmp_sock.settimeout(DEFAULT_TIMEOUT)
            icmp_sock.sendto(packet, (ip_addr, 7))
            
            elapsed = 0
            try:
                _, _ = icmp_sock.recvfrom(32)
                elapsed = (time.time() - start) * 1000
                print(f"Reply: {elapsed:.2f} ms")
                successful += 1
            except socket.timeout:
                print("Request timed out")
        else:
            # TCP fallback
            try:
                sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                sock.settimeout(DEFAULT_TIMEOUT)
                sock.connect((ip_addr, 443))
                elapsed = (time.time() - start) * 1000
                print(f"TCP 443 OK: {elapsed:.2f} ms")
                sock.close()
                successful += 1
            except Exception:
                print("TCP 443 Failed")
        
        if i < count - 1:
            time.sleep(interval)
    
    print(f"{'='*30} STATS {'='*30}")
    print(f"Sent: {count}, Successful: {successful}, Failed: {count - successful}")
    return successful > 0

def check_dns(host):
    print(f"\n{'='*30} DNS CHECK {'='*30}")
    start = time.time()
    try:
        infos = socket.getaddrinfo(host, 443)
        elapsed = (time.time() - start) * 1000
        dns_ip = infos[0][4][0] if infos else "Unknown"
        print(f"Resolution for {host}: {dns_ip} ({elapsed:.2f} ms)")
        return True
    except Exception as e:
        print(f"DNS check failed: {e}")
        return False

def check_https(host):
    print(f"\n{'='*30} HTTPS CHECK {'='*30}")
    import ssl
    try:
        context = ssl.create_default_context()
        with socket.create_connection((host, 443), timeout=5) as sock:
            with context.wrap_socket(sock, server_hostname=host) as ssock:
                cert = ssock.getpeercert()
                if not cert:
                    print("TLS Handshake OK, but no certificate details found.")
                    return True
                subject = cert.get('subject', [])
                if subject:
                    # Find commonName or CN
                    for rdn in subject:
                        for attr_type, attr_value in rdn:
                            if attr_type == 'commonName' or attr_type == 'CN':
                                print(f"TLS Handshake OK. Cert CN: {attr_value}")
                                return True
                print(f"TLS Handshake OK. Cert details: {cert}")
                return True
    except Exception as e:
        print(f"HTTPS check failed: {e}")
        return False

def main():
    parser = argparse.ArgumentParser(description="Network Diagnostic Tool")
    parser.add_argument("target", nargs="?", default="google.com", help="Target host to diagnose")
    parser.add_argument("--interval", type=int, default=DEFAULT_INTERVAL, help="Interval between pings in seconds")
    parser.add_argument("--count", type=int, default=DEFAULT_COUNT, help="Number of pings/probes")
    args = parser.parse_args()

    print(f"\nNetwork Diagnostics for: {args.target}")
    
    ping(args.target, count=args.count, interval=args.interval)
    check_dns(args.target)
    check_https(args.target)

if __name__ == "__main__":
    main()
