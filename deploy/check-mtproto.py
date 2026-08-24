#!/usr/bin/env python3
"""Checks whether MTProto reaches the server from this machine.

It tells apart three states that look identical from outside ("does not
connect"):
  - the server is unreachable at all (no TCP);
  - TCP works but there is no answer, so traffic is filtered on the way;
  - the server answers, which means the app itself is at fault.

It sends the first handshake request (req_pq_multi) in the clear: no keys and no
account are needed. No third-party libraries required.

Usage: python3 check-mtproto.py [address] [port]
"""
import os
import socket
import struct
import sys
import time

ADDRESS = sys.argv[1] if len(sys.argv) > 1 else os.environ.get("TEAMGRAM_HOST", "127.0.0.1")
PORT = int(sys.argv[2]) if len(sys.argv) > 2 else 10443

REQ_PQ_MULTI = 0xBE7E8EF1
RES_PQ = 0x05162463


def build_request() -> bytes:
    """An unencrypted MTProto message carrying req_pq_multi.

    The transport is the full one (length, number, body, checksum): every server
    build understands it, while the abridged variants are not supported
    everywhere.
    """
    import zlib

    nonce = os.urandom(16)
    payload = struct.pack("<I", REQ_PQ_MULTI) + nonce

    # auth_key_id = 0 marks an unencrypted message, which is how every handshake
    # begins
    msg_id = int(time.time()) << 32
    body = struct.pack("<qqi", 0, msg_id, len(payload)) + payload

    length = len(body) + 12
    packet = struct.pack("<ii", length, 0) + body

    return packet + struct.pack("<I", zlib.crc32(packet)), nonce


def main() -> int:
    print(f"checking {ADDRESS}:{PORT}")

    request, nonce = build_request()
    started = time.monotonic()

    try:
        s = socket.create_connection((ADDRESS, PORT), timeout=15)
    except OSError as e:
        print(f"NO CONNECTION: cannot establish a connection ({e})")
        print("The server is down, the port is firewalled, or the address is unreachable.")
        return 1

    print(f"TCP established in {time.monotonic() - started:.2f}s")

    try:
        s.sendall(request)
        s.settimeout(15)
        answer = s.recv(512)
    except OSError as e:
        print(f"NO ANSWER: {e}")
        print("TCP goes through but the handshake does not. Looks like traffic filtering on the way.")
        return 1
    finally:
        s.close()

    if not answer:
        print("NO ANSWER: the server closed the connection silently.")
        print("TCP goes through but the handshake does not. Looks like traffic filtering on the way.")
        return 1

    # Look for the res_pq constructor and our nonce in the answer: that shows our
    # server replied rather than something on the way
    if struct.pack("<I", RES_PQ) in answer and nonce in answer:
        print(f"SERVER ANSWERED in {time.monotonic() - started:.2f}s - MTProto goes through")
        return 0

    print(f"STRANGE ANSWER ({len(answer)} bytes): {answer[:32].hex()}")
    print("Either it is not our server answering, or the answer was tampered with.")
    return 1


if __name__ == "__main__":
    sys.exit(main())
