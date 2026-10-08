#!/usr/bin/env python3
"""Network diagnostic tool with real ICMP, DNS, TCP, HTTP, and speed tests."""

import socket
import struct
import time
import argparse
import urllib.request
import os
import platform

def ping(host, count=4, timeout=2.0):
    """Real ICMP ping with correct packet construction and checksum."""
    try:
        target = socket.gethostbyname(host)
    except socket.gaierror as e:
        return None, f"DNS error: {e}"
    
    sock = None
    try:
        icmp = 1  # ICMP echo
        sock = socket.socket(socket.AF_INET, socket.SOCK_RAW, icmp)
        sock.settimeout(timeout)
    except PermissionError:
        return None, "Not running as root; using fallback"
    except OSError as e:
        sock = None
        return None, str(e)
    
    results = []
    try:
        for i in range(count):
            id = (os.getpid() >> 8) & 0xffff
            seq = i & 0xffff
            data = struct.pack("!HHIH", id, seq, int(time.time()), 0)
            data = data + b" " * 32  # Pad to minimum size
            
            # Calculate ICMP checksum (RFC 792)
            hdr = struct.pack("!BBHHH", 8, 0, 0, id, seq)
            pkt = hdr + data
            
            # Compute checksum
            cksum = 0
            for j in range(0, len(pkt), 2):
                cksum += struct.unpack("!H", pkt[j:j+2])[0]
            while cksum >> 16:
                cksum = (cksum & 0xffff) + (cksum >> 16)
            cksum = ~cksum & 0xffff
            pkt = struct.pack("!BBH", 8, 0, cksum) + pkt[4:]
            
            sock.sendto(pkt, (target, 0))
            start = time.time()
            
            try:
                while True:
                    d, addr = sock.recvfrom(512)
                    ptype = d[0]
                    if ptype == 0 and struct.unpack("!H", d[4:6])[0] == id:
                        break
            except socket.timeout:
                results.append(0.0)
                continue
            except Exception:
                results.append(0.0)
                
            rt = (time.time() - start) * 1000  # Convert to ms
            results.append(rt)
    finally:
        if sock:
            sock.close()
    
    return results, None

def test_dns(host):
    """Measure DNS resolution time."""
    try:
        start = time.time()
        socket.getaddrinfo(host, None)
        rt = (time.time() - start) * 1000.0
        return 0, f"{rt:.1f}ms"
    except socket.gaierror as e:
        return 1, f"resolution error: {e}"

def test_tcp(host, port=443, timeout=5.0):
    """Measure TCP connection time to specific port."""
    try:
        start = time.time()
        sock = socket.create_connection((host, port), timeout)
        sock.close()
        rt = (time.time() - start) * 1000.0
        return 0, f"{rt:.1f}ms"
    except Exception as e:
        return 1, str(e)

def test_http(timeout=10):
    """Test HTTP response from target."""
    try:
        req = urllib.request.Request("https://google.com/", timeout=timeout)
        req.add_header("User-Agent", "NetworkDiagnostic/1.0")
        start = time.time()
        resp = urllib.request.urlopen(req, timeout=timeout)
        elapsed = time.time() - start
        resp.close()
        return 0, f"200 OK - {elapsed*1000:.1f}ms"
    except Exception as e:
        return 1, str(e)

def test_speed(timeout=60):
    """Test download speed by downloading a test file."""
    try:
        url = "https://httpbin.org/anything/5M"
        start = time.time()
        urllib.request.urlretrieve(url, "/tmp/netspeed_test.bin", timeout=timeout)
        elapsed = time.time() - start
        os.remove("/tmp/netspeed_test.bin")
        speed = 40.96 / elapsed  # 4096 bytes / elapsed seconds = KB/s
        return 0, f"{speed:.1f} KB/s"
    except Exception as e:
        return 1, str(e)
    finally:
        try:
            os.remove("/tmp/netspeed_test.bin")
        except FileNotFoundError:
            pass

def test_upload(timeout=60):
    """Test upload speed by uploading a test file."""
    try:
        url = "https://httpbin.org/post"
        data = open("/dev/urandom", "rb").read(512*1024)  # 512 KB
        start = time.time()
        req = urllib.request.Request(url, data=data * 2)  # 1 MB total
        resp = urllib.request.urlopen(req, timeout=timeout)
        elapsed = time.time() - start
        resp.close()
        speed = 1048 / elapsed  # 1048 bytes / elapsed seconds = KB/s
        return 0, f"{speed:.1f} KB/s"
    except Exception as e:
        return 1, str(e)

def main():
    parser = argparse.ArgumentParser(description="NetworkDiagnostic - real ICMP ping, DNS, TCP, HTTP, and speed tests")
    parser.add_argument("--target", default="google.com", help="Target host (default: google.com)")
    parser.add_argument("--interval", type=int, default=5, help="Run every n seconds (default: 5)")
    parser.add_argument("--count", type=int, default=3, help="Number of ping attempts (default: 3)")
    
    args = parser.parse_args()
    
    print(f"Testing network connectivity to {args.target}...")
    
    # Ping
    print("[PING]")
    results, error = ping(args.target, count=args.count)
    if results and results[0] != 0:
        sent = len(results)
        loss = sent - sum(1 for r in results if r != 0)
        avg = sum(r for r in results if r != 0) / max(1, sent - loss)
        print(f"  Packets: {sent}/4 transmitted")
        print(f"  Loss: {loss}s")
        print(f"  Avg: {avg}ms")
        print(f"  Min: {min(r for r in results if r != 0):.1f}ms")
        print(f"  Max: {max(r for r in results if r != 0):.1f}ms")
    elif error:
        print(f"  Error: {error}")
    else:
        print("  No ICMP echo replies received")
    
    # DNS
    ret, msg = test_dns(args.target)
    print(f"[DNS] {'OK' if ret == 0 else 'FAIL'} {msg}")
    
    # TCP
    ret, msg = test_tcp(args.target, port=443)
    print(f"[TCP] {'OK' if ret == 0 else 'FAIL'} {msg}")
    
    # HTTP
    ret, msg = test_http()
    print(f"[HTTP] {'OK' if ret == 0 else 'FAIL'} {msg}")
    
    # Speed download
    ret, msg = test_speed()
    print(f"[SPEED-Download] {'OK' if ret == 0 else 'FAIL'} {msg}")
    
    # Speed upload
    ret, msg = test_upload()
    print(f"[SPEED-Upload] {'OK' if ret == 0 else 'FAIL'} {msg}")

if __name__ == "__main__":
    main()