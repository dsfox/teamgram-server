package gnet

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// A server of one's own has a key of its own, and an app given the server's
// code fetches that key with a plain `GET /key` on the MTProto port, then
// checks it against the code before the first handshake (#244). Only this one
// request is answered; every other HTTP request is closed as before.

type keyAsk int

const (
	notKeyRequest keyAsk = iota
	keyPartial
	keyWhole
	keyTooLong
)

var keyRequestLine = []byte("GET /key HTTP/1.")

const keyRequestLimit = 2048

// askedForKey tells the first bytes of a connection apart: not the key
// request (the codec decides as before), part of it so far, all of it (the
// headers end within the limit), or too long to be it.
func askedForKey(buf []byte) keyAsk {
	if len(buf) == 0 {
		return notKeyRequest
	}
	n := len(buf)
	if n > len(keyRequestLine) {
		n = len(keyRequestLine)
	}
	if !bytes.Equal(buf[:n], keyRequestLine[:n]) {
		return notKeyRequest
	}
	if len(buf) < len(keyRequestLine) {
		return keyPartial
	}
	if end := bytes.Index(buf, []byte("\r\n\r\n")); end >= 0 && end+4 <= keyRequestLimit {
		return keyWhole
	}
	if len(buf) >= keyRequestLimit {
		return keyTooLong
	}
	return keyPartial
}

// keyAnswerFor is the whole HTTP answer to `GET /key`, built once at start: the
// server's own public key, or a 404 when it has none (the stock key is never
// handed out - pinning a key anybody holds protects nothing).
func keyAnswerFor(own *rsa.PublicKey) []byte {
	status, body := "404 Not Found", []byte("this server has no key of its own\n")
	if own != nil {
		status = "200 OK"
		body = pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(own)})
	}
	head := fmt.Sprintf("HTTP/1.1 %s\r\nContent-Type: text/plain; charset=us-ascii\r\nContent-Length: %d\r\nCache-Control: no-store\r\nConnection: close\r\n\r\n",
		status, len(body))
	return append([]byte(head), body...)
}
