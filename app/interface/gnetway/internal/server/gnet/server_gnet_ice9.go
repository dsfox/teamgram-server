package gnet

import (
	"context"
	"math/rand"
	"time"

	"github.com/teamgram/teamgram-server/app/interface/gnetway/internal/dao"
	"github.com/teamgram/teamgram-server/app/interface/session/session"

	"github.com/panjf2000/gnet/v2"
	"github.com/zeromicro/go-zero/core/logx"
)

// What ice9 adds to upstream's server_gnet.go, which keeps a one-line call to
// each of these (#198).

// How long a packet the session service did not take is tried again before
// its connection is closed, and how often (#228). Longer than a restart of the
// session service or a pause like the one on 31 August that ended by itself.
const (
	undeliveredPatience = 15 * time.Second
	undeliveredRetry    = 500 * time.Millisecond
)

// trafficIn is the start of OnTraffic. Every byte that reaches this process is
// recorded here, before any codec can swallow it: together with the client
// logging its local port on each write, a packet can be followed from one side
// to the other without guesswork. The connection has spoken, and its idle
// deadline moves.
func (s *Server) trafficIn(c gnet.Conn, ctx *connContext) {
	logx.Infof("traffic in: conn(%s) %d bytes buffered", c, c.InboundBuffered())
	ctx.spoke = true
	s.extendIdleDeadline(c, ctx)
}

// logClosedByTimeout says why the reaper closed a connection. The two cases
// mean very different things and must not share a counter. A connection that
// never spoke is the client's own pool churn - measured at 34 abandoned out of
// 38 opened - and reaping it is the point. A connection that went quiet after
// carrying traffic is one the client may still believe in, and that is worth
// an alarm.
func logClosedByTimeout(c gnet.Conn, ctx *connContext) {
	if ctx.spoke {
		logx.Errorf("close conn(%s) by idle timeout after traffic", c)
	} else {
		logx.Infof("close conn(%s) by timeout, never spoke", c)
	}
}

// sendToSessionPatiently hands a client's packet to the session service. When
// the service does not take it - paused, restarting, or only just back - the
// packet is not dropped: dropped, it would leave its client waiting for an
// answer that never comes on a connection that still looks alive. A listener's
// getDifference was lost so on 5 October and its update loop stood still
// (#228). So it is tried again for as long as an outage that ends by itself
// lasts.
func (s *Server) sendToSessionPatiently(c gnet.Conn, ctx context.Context, permAuthKeyId int64, data *session.TLSessionSendDataToSession) error {
	err := s.svcCtx.Dao.SessionDispatcher.SendData(ctx, permAuthKeyId, data)
	if dao.Undelivered(err) {
		first := time.Now()
		logx.Errorf("conn(%s): the session did not take a packet (%v); trying again for %s", c, err, undeliveredPatience)
		for dao.Undelivered(err) && time.Since(first) < undeliveredPatience {
			time.Sleep(undeliveredRetry)
			err = s.svcCtx.Dao.SessionDispatcher.SendData(ctx, permAuthKeyId, data)
		}
		if err == nil {
			logx.Infof("conn(%s): the packet reached the session %s late", c, time.Since(first).Round(time.Millisecond))
		}
	}
	return err
}

// closedUndelivered closes a connection whose packet reached no session even
// after sendToSessionPatiently's patience. Closed, the connection makes the
// client send what it is waiting on again over the next one, instead of
// waiting for ever (#228).
func closedUndelivered(c gnet.Conn, err error) bool {
	if !dao.Undelivered(err) {
		return false
	}
	logx.Errorf("conn(%s): nothing reached the session (%v); closing so the client sends it again", c, err)
	_ = c.Close()
	return true
}

// extendIdleDeadline pushes back the moment the reaper may close this
// connection. Traffic in either direction proves the peer is there: a
// connection that only carries updates and file chunks down to the client is
// not idle, and reaping it left the client writing requests into a socket the
// server had already closed - it noticed only when its own timeout fired,
// which is the pause the user sees as "Updating".
func (s *Server) extendIdleDeadline(c gnet.Conn, ctx *connContext) {
	oldCloseDate := ctx.closeDate
	if ctx.http {
		ctx.closeDate = s.CachedNow() + kHttpIdleTimeout + rand.Int63()%10
	} else {
		ctx.closeDate = s.CachedNow() + kIdleTimeout + rand.Int63()%10
	}
	s.timeoutWheel.Move(c.ConnId(), oldCloseDate, ctx.closeDate)
}

// UnThreadSafeWrite is upstream's UnThreadSafeWrite after moving the idle
// deadline: a write is traffic too (see extendIdleDeadline). Upstream's
// function says every failure itself, so its callers lose nothing by `_ =`.
func (s *Server) UnThreadSafeWrite(c gnet.Conn, msg interface{}) error {
	s.extendIdleDeadline(c, c.Context().(*connContext))
	return UnThreadSafeWrite(c, msg)
}
