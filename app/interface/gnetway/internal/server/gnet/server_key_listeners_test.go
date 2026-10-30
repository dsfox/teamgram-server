package gnet

import (
	"bytes"
	"testing"

	"github.com/panjf2000/gnet/v2"
	"github.com/teamgram/teamgram-server/app/interface/gnetway/internal/config"
	httpcodec "github.com/teamgram/teamgram-server/app/interface/gnetway/internal/server/gnet/http"
	"github.com/teamgram/teamgram-server/app/interface/gnetway/internal/server/gnet/ws"
)

// The WebSocket listener reads with Read rather than Peek.
func (f *fakeConn) InboundBuffered() int { return len(f.in) }
func (f *fakeConn) Read(b []byte) (int, error) {
	n := copy(b, f.in)
	f.in = f.in[n:]
	return n, nil
}

// GET /key is answered on every listener, not only the MTProto TCP ports: a
// server put up behind a proxy that speaks only HTTP or WebSocket hands out
// its key the same way (#245).
func TestEveryListenerHandsOutTheKey(t *testing.T) {
	own, _ := keyFile(t, 2048)
	s := &Server{handshake: mustNewHandshake([]config.RSAKey{{KeyFile: own}})}
	request := []byte("GET /key HTTP/1.1\r\nHost: x\r\n\r\n")

	conn := &fakeConn{in: append([]byte(nil), request...)}
	if action := s.onHttpData(&connContext{httpCodec: new(httpcodec.HttpCodec)}, conn); action != gnet.Close {
		t.Fatalf("HTTP: action %v, not Close", action)
	}
	if !bytes.Equal(conn.written.Bytes(), s.handshake.keyAnswer) {
		t.Fatalf("HTTP answered %q", conn.written.Bytes())
	}

	conn = &fakeConn{in: append([]byte(nil), request...)}
	if action := s.onWebsocketData(&connContext{wsCodec: new(ws.WsCodec)}, conn); action != gnet.Close {
		t.Fatalf("WebSocket: action %v, not Close", action)
	}
	if !bytes.Equal(conn.written.Bytes(), s.handshake.keyAnswer) {
		t.Fatalf("WebSocket answered %q", conn.written.Bytes())
	}

	// Half a request waits for the rest, on WebSocket as on TCP.
	conn = &fakeConn{in: []byte("GET /key HTTP/1.1\r\nHo")}
	if action := s.onWebsocketData(&connContext{wsCodec: new(ws.WsCodec)}, conn); action != gnet.None || conn.written.Len() != 0 {
		t.Fatalf("WebSocket, half a request: action %v, wrote %q", action, conn.written.Bytes())
	}
}
