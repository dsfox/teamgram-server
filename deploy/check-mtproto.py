#!/usr/bin/env python3
"""Checks whether MTProto reaches the server from this machine.

It tells apart three states that look identical from outside ("does not
connect"):
  - the server is unreachable at all (no TCP);
  - TCP works but there is no answer, so traffic is filtered on the way;
  - the server answers, which means the app itself is at fault.

It sends the first handshake request (req_pq_multi) in the clear: no keys and no
account are needed. No third-party libraries required.

With --key N (repeatable) it also requires the answer to offer the key whose
fingerprint is N: a server can answer and still not hold the key the apps know
(ice9 #242), and then no phone gets past the next step.

Usage: python3 check-mtproto.py [address] [port] [--key fingerprint ...]
"""
import os
import socket
import struct
import sys
import time


def _arguments(argv):
    positional, keys = [], []
    rest = list(argv)
    while rest:
        word = rest.pop(0)
        if word == "--key":
            keys.append(int(rest.pop(0)))
        else:
            positional.append(word)
    return positional, keys


_POSITIONAL, EXPECTED_KEYS = _arguments(sys.argv[1:])
ADDRESS = _POSITIONAL[0] if _POSITIONAL else os.environ.get("TEAMGRAM_HOST", "127.0.0.1")
PORT = int(_POSITIONAL[1]) if len(_POSITIONAL) > 1 else 10443

REQ_PQ_MULTI = 0xBE7E8EF1
RES_PQ = 0x05162463
VECTOR = 0x1CB5C415


def offered_keys(answer: bytes) -> list:
    """The key fingerprints a resPQ lists, after nonce, server_nonce and pq."""
    at = answer.index(struct.pack("<I", RES_PQ)) + 4 + 16 + 16
    length = answer[at]
    at += (1 + length + 3) // 4 * 4
    if struct.unpack_from("<I", answer, at)[0] != VECTOR:
        return []
    count = struct.unpack_from("<i", answer, at + 4)[0]
    return [struct.unpack_from("<Q", answer, at + 8 + 8 * i)[0] for i in range(count)]


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
        offered = offered_keys(answer)
        print("keys offered: " + ", ".join(str(key) for key in offered))
        missing = [key for key in EXPECTED_KEYS if key not in offered]
        if missing:
            print("MISSING KEY: the server does not offer " + ", ".join(str(key) for key in missing))
            return 1
        return 0

    print(f"STRANGE ANSWER ({len(answer)} bytes): {answer[:32].hex()}")
    print("Either it is not our server answering, or the answer was tampered with.")
    return 1


if __name__ == "__main__":
    sys.exit(main())
