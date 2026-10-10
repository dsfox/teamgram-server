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

With --code it fetches the server's own key the way an app does (GET /key on
the same port), refuses it unless it is the one the handshake offers first and
is shaped as the apps require, and prints the server's code (ice9 #244) - the
24 digits a person types beside the address so the app knows it reached this
server and not one posing as it. install.sh prints it from here.

This file is also the reference for the server code: the clients' ServerCode
and tests/test_the_server_code_is_one_rule.py hold to it, through the vectors
in deploy/keys/server_code_vectors.json.

Usage: python3 check-mtproto.py [address] [port] [--key fingerprint ...] [--code]
"""
import base64
import hashlib
import os
import re
import socket
import struct
import sys
import time

REQ_PQ_MULTI = 0xBE7E8EF1
RES_PQ = 0x05162463
VECTOR = 0x1CB5C415

# teamgram's stock key: its private half is public, so no app pins it.
STOCK_FINGERPRINT = 12240908862933197005

# The one shape a server's own key may have: the canonical PKCS#1 DER of a
# 2048-bit modulus with e = 65537. Anything looser would let somebody vary the
# encoding, rather than the key, to hit a code.
CANONICAL_HEAD = bytes.fromhex("3082010a0282010100")
CANONICAL_TAIL = bytes.fromhex("0203010001")
CANONICAL_LENGTH = 270
PEM_BEGIN = "-----BEGIN RSA PUBLIC KEY-----"
PEM_END = "-----END RSA PUBLIC KEY-----"


class KeyRefused(Exception):
    """A key no app would pin, and why."""


def check_own_key(der: bytes) -> int:
    """The modulus of a key shaped as the apps require, or KeyRefused."""
    if (len(der) != CANONICAL_LENGTH or not der.startswith(CANONICAL_HEAD)
            or not der.endswith(CANONICAL_TAIL) or not der[9] & 0x80):
        raise KeyRefused("not the canonical encoding of an RSA-2048 key with e 65537")
    modulus = int.from_bytes(der[9:265], "big")
    if fingerprint(modulus, 65537) == STOCK_FINGERPRINT:
        raise KeyRefused("the stock teamgram key, whose private half is public")
    return modulus


def der_of(text: str) -> bytes:
    """The DER of the one RSA PUBLIC KEY block in a PEM text, strictly."""
    if text.count(PEM_BEGIN) != 1 or text.count(PEM_END) != 1:
        raise KeyRefused("not exactly one RSA PUBLIC KEY block")
    body = text.split(PEM_BEGIN, 1)[1].split(PEM_END, 1)[0]
    body = re.sub(r"[ \t\r\n]", "", body)
    if not re.fullmatch(r"[A-Za-z0-9+/]{360}", body):
        raise KeyRefused("the block is not the 360 base64 characters of an RSA-2048 key")
    der = base64.b64decode(body, validate=True)
    check_own_key(der)
    return der


def _tl_bytes(value: int) -> bytes:
    raw = value.to_bytes((value.bit_length() + 7) // 8, "big")
    head = bytes([len(raw)]) if len(raw) < 254 else bytes([254]) + len(raw).to_bytes(3, "little")
    body = head + raw
    return body + b"\x00" * (-len(body) % 4)


def fingerprint(modulus: int, exponent: int) -> int:
    """The number a handshake names a key by (tools/rsa_fingerprint.py)."""
    digest = hashlib.sha1(_tl_bytes(modulus) + _tl_bytes(exponent)).digest()
    return struct.unpack("<Q", digest[-8:])[0]


def server_code(der: bytes) -> str:
    """SHA-256 of the DER, its first 10 bytes as a big-endian number, mod 10^24."""
    value = int.from_bytes(hashlib.sha256(der).digest()[:10], "big") % 10 ** 24
    return f"{value:024d}"


def shown(code: str) -> str:
    return " ".join(code[i:i + 4] for i in range(0, 24, 4))


def digits(typed: str):
    """What a person typed as a code: None if it is not one, "" if nothing."""
    if not re.fullmatch(r"[0-9 \-]*", typed):
        return None
    bare = re.sub(r"[ \-]", "", typed)
    if bare == "":
        return ""
    return bare if len(bare) == 24 else None


LINK_PREFIXES = ("https://i.ice9.app/server#", "http://i.ice9.app/server#", "tg2://server#")


def parse_link(url: str):
    """(host, port, code or "") for a server link, "malformed" for a server link
    that cannot be read, None for any other link."""
    prefix = next((p for p in LINK_PREFIXES if url[:len(p)].lower() == p), None)
    if prefix is None:
        return None
    rest = url[len(prefix):]
    match = re.fullmatch(r"([A-Za-z0-9.\-]+):([0-9]{1,5})(?:/([0-9]{24}))?", rest)
    if "%" in rest or not match or not 1 <= int(match.group(2)) <= 65535:
        return "malformed"
    return match.group(1), int(match.group(2)), match.group(3) or ""


def fetch_key(address: str, port: int, timeout: float = 10) -> bytes:
    """What the server says to GET /key, read to its end (at most 8 KiB)."""
    with socket.create_connection((address, port), timeout=timeout) as s:
        s.settimeout(timeout)
        s.sendall(f"GET /key HTTP/1.1\r\nHost: {address}\r\nConnection: close\r\n\r\n".encode())
        answer = b""
        while len(answer) < 8192:
            chunk = s.recv(8192 - len(answer))
            if not chunk:
                break
            answer += chunk
        return answer


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


def _arguments(argv):
    positional, keys, code = [], [], False
    rest = list(argv)
    while rest:
        word = rest.pop(0)
        if word == "--key":
            keys.append(int(rest.pop(0)))
        elif word == "--code":
            code = True
        else:
            positional.append(word)
    address = positional[0] if positional else os.environ.get("TEAMGRAM_HOST", "127.0.0.1")
    port = int(positional[1]) if len(positional) > 1 else 10443
    return address, port, keys, code


def say_code(address: str, port: int, offered: list) -> int:
    try:
        answer = fetch_key(address, port)
    except OSError as e:
        print(f"NO KEY ANSWER: GET /key went nowhere ({e})")
        return 1
    if not answer:
        print("NO KEY ANSWER: the server closed GET /key without a word - its image is older than server codes (#244)")
        return 1
    status = answer.split(b"\r\n", 1)[0].decode("latin-1")
    if " 404 " in status + " ":
        print("NO KEY OF ITS OWN: GET /key answered 404 - run install.sh again to make the server its key")
        return 1
    if " 200 " not in status + " " or b"\r\n\r\n" not in answer:
        print(f"NO KEY ANSWER: GET /key answered {status!r}")
        return 1
    try:
        der = der_of(answer.split(b"\r\n\r\n", 1)[1].decode("ascii"))
    except (KeyRefused, UnicodeDecodeError, ValueError) as refused:
        print(f"REFUSED KEY: {refused}")
        return 1
    served = fingerprint(check_own_key(der), 65537)
    if not offered or offered[0] != served:
        print(f"WRONG KEY: GET /key hands out {served}, but the handshake offers {offered[0] if offered else 'nothing'} first")
        return 1
    print(f"served key: {served} - RSA-2048, e 65537, offered first")
    print(f"server code: {shown(server_code(der))}")
    return 0


def main(argv=None) -> int:
    address, port, expected, code = _arguments(sys.argv[1:] if argv is None else argv)
    print(f"checking {address}:{port}")

    request, nonce = build_request()
    started = time.monotonic()

    try:
        s = socket.create_connection((address, port), timeout=15)
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
        missing = [key for key in expected if key not in offered]
        if missing:
            print("MISSING KEY: the server does not offer " + ", ".join(str(key) for key in missing))
            return 1
        return say_code(address, port, offered) if code else 0

    print(f"STRANGE ANSWER ({len(answer)} bytes): {answer[:32].hex()}")
    print("Either it is not our server answering, or the answer was tampered with.")
    return 1


if __name__ == "__main__":
    sys.exit(main())
