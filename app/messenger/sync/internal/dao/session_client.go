// Copyright 2022 Teamgram Authors
//  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author: teamgramio (teamgram.io@gmail.com)
//

package dao

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	sessionclient "github.com/teamgram/teamgram-server/app/interface/session/client"
	"github.com/teamgram/teamgram-server/pkg/discovery"
	"github.com/teamgram/teamgram-server/app/interface/session/session"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
)

type sessionDataCtx struct {
	ctx     context.Context
	updates any
}

// SessionOptions comet options.
type SessionOptions struct {
	RoutineSize uint64
	RoutineChan uint64
}

// Session is a gateway.
type Session struct {
	serverId       string
	client         sessionclient.SessionClient
	sessionChan    []chan sessionDataCtx
	sessionChanNum uint64
	options        SessionOptions
	ctx            context.Context
	cancel         context.CancelFunc
	unavailable    atomic.Bool  // set when connection errors detected
	markedAt       atomic.Int64 // when it was set, so that it can be unset
}

// How long a session that has just failed is left alone before one push is let
// through to find out whether it is answering again.
//
// The mark this clears used to be permanent: one connection error and every
// push to that session was refused for the life of the process, with nothing
// anywhere that could take the mark off. On 31 August that is what 192
// undelivered updates were - the session server was back within seconds and
// nothing here ever asked again (#146). A restart of the whole container was
// the only cure.
//
// Long enough that a session which is really down is not asked on every push,
// short enough that a person waiting for a message does not notice.
const sessionRetryAfter = 5 * time.Second

// pushCallDeadline is how long the call itself may take once it leaves the
// queue.
//
// #144 put a deadline on the push and it never reached the call. `deliver` in
// the core wraps `PushUpdates`, and `PushUpdates` only puts the work on a
// channel and returns - the gRPC call is made later, by `process`, with the
// context that travelled in the channel. That context comes from `detach`,
// which is `context.WithoutCancel`, and by its contract such a context carries
// no deadline at all. So the three seconds bounded the enqueue, which never
// takes any time, and the call was left unbounded.
//
// That is the shape of what happened on 31 August: the session client stopped
// answering, every call waited on it for ever, and 192 updates never reached
// anybody. A refused connection answers at once and is not this; a server that
// accepts and then says nothing is, and it is the harder half.
//
// The detaching itself stays and is right: the request that produced the push
// has answered and cancelled its context by the time the queue gets to it, and
// with the original context 30 deliveries in 35 died as "context canceled".
// What was missing is a deadline of the call's own.
//
// A var so a test can shorten it.
var pushCallDeadline = 3 * time.Second

// markUnavailable records that this session failed, and when.
func (c *Session) markUnavailable() {
	c.markedAt.Store(time.Now().UnixNano())
	if !c.unavailable.Swap(true) {
		logx.Errorf("session(%s) marked unavailable due to conn error", c.serverId)
	}
}

// refusing says whether to drop this push without trying.
//
// It goes false once the pause is over, while the session is still marked:
// that one push is what finds out the server is back, and getting through
// clears the mark. Without it there is no way back at all.
func (c *Session) refusing() bool {
	if !c.unavailable.Load() {
		return false
	}
	return time.Since(time.Unix(0, c.markedAt.Load())) < sessionRetryAfter
}

// alive records a push that got through, which is the one thing that can take
// the mark off. Said out loud, because "delivery came back by itself" is the
// sentence this whole issue is about.
func (c *Session) alive() {
	if c.unavailable.Swap(false) {
		logx.Infof("session(%s) is answering again", c.serverId)
	}
}

func isSessionConnError(err error) bool {
	s, ok := grpcStatus.FromError(err)
	if !ok {
		return false
	}
	switch s.Code() {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	}
	return false
}

// process
func (c *Session) process(sessionChan chan sessionDataCtx) {
	var err error
	for {
		select {
		case sessionData, ok := <-sessionChan:
			if !ok {
				logx.Errorf("process error")
				return
			}

			if c.refusing() {
				logx.Errorf("session(%s) unavailable, dropping push", c.serverId)
				continue
			}

			// The call's own deadline. What arrives in the channel carries none:
			// see pushCallDeadline.
			pushCtx, done := context.WithTimeout(sessionData.ctx, pushCallDeadline)

			switch r := sessionData.updates.(type) {
			case *session.TLSessionPushSessionUpdatesData:
				_, err = c.client.SessionPushSessionUpdatesData(pushCtx, r)
				if err != nil {
					logx.Errorf("c.client.PushSessionUpdates(%s, %v, reply) serverId:%s error(%v)", r, c.serverId, c.serverId, err)
					if isSessionConnError(err) {
						c.markUnavailable()
					}
				} else {
					c.alive()
				}
			case *session.TLSessionPushUpdatesData:
				_, err = c.client.SessionPushUpdatesData(pushCtx, r)
				if err != nil {
					logx.Errorf("c.client.PushUpdates(%s, %v, reply) serverId:%s error(%v)", r, c.serverId, c.serverId, err)
					if isSessionConnError(err) {
						c.markUnavailable()
					}
				} else {
					c.alive()
				}
			case *session.TLSessionPushRpcResultData:
				_, err = c.client.SessionPushRpcResultData(pushCtx, r)
				if err != nil {
					logx.Errorf("c.client.PushRpcResult(%s, %v, reply) serverId:%s error(%v)", r, c.serverId, c.serverId, err)
					if isSessionConnError(err) {
						c.markUnavailable()
					}
				} else {
					c.alive()
				}
			default:
				logx.Errorf("invalid type: %#v", r)
			}
			done()
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Session) Close() (err error) {
	finish := make(chan bool)
	go func() {
		for {
			n := len(c.sessionChan)
			for _, ch := range c.sessionChan {
				n += len(ch)
			}
			if n == 0 {
				finish <- true
				return
			}
			time.Sleep(time.Second)
		}
	}()
	select {
	case <-finish:
		logx.Info("close session client finish")
	case <-time.After(5 * time.Second):
		err = fmt.Errorf("close session(server:%s push:%d) timeout", c.serverId, len(c.sessionChan))
	}
	c.cancel()
	return
}

// detach keeps the values of a request context (tracing, metadata) but drops its
// cancellation.
//
// The push below is handled by another goroutine, long after the request that
// produced it has answered and cancelled its context. With the original context
// the delivery died with "context canceled" before it even started: measured 30
// failures out of 35 on a live server. The update then never reached the phone
// over its connection and the person saw the message only after the client asked
// for it again — a notification arrived instantly while the message itself took
// tens of seconds.
func detach(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}

func (c *Session) PushUpdates(ctx context.Context, msg *session.TLSessionPushUpdatesData) (err error) {
	if c.refusing() {
		return fmt.Errorf("session(%s) unavailable", c.serverId)
	}
	idx := atomic.AddUint64(&c.sessionChanNum, 1) % c.options.RoutineSize
	c.sessionChan[idx] <- sessionDataCtx{ctx: detach(ctx), updates: msg}
	return
}

func (c *Session) PushSessionUpdates(ctx context.Context, msg *session.TLSessionPushSessionUpdatesData) (err error) {
	if c.refusing() {
		return fmt.Errorf("session(%s) unavailable", c.serverId)
	}
	idx := atomic.AddUint64(&c.sessionChanNum, 1) % c.options.RoutineSize
	c.sessionChan[idx] <- sessionDataCtx{ctx: detach(ctx), updates: msg}
	return
}

func (c *Session) PushRpcResult(ctx context.Context, msg *session.TLSessionPushRpcResultData) (err error) {
	if c.refusing() {
		return fmt.Errorf("session(%s) unavailable", c.serverId)
	}
	idx := atomic.AddUint64(&c.sessionChanNum, 1) % c.options.RoutineSize
	c.sessionChan[idx] <- sessionDataCtx{ctx: detach(ctx), updates: msg}
	return
}

// NewSession new a comet.
func NewSession(c zrpc.RpcClientConf, options SessionOptions) (*Session, error) {
	sess := &Session{
		serverId:    c.Endpoints[0],
		sessionChan: make([]chan sessionDataCtx, options.RoutineSize),
		options:     options,
	}

	cli, err := zrpc.NewClient(c)
	if err != nil {
		logx.Errorf("watchComet NewClient(%+v) error(%v)", c, err)
		return nil, err
	}
	sess.client = sessionclient.NewSessionClient(cli)
	sess.ctx, sess.cancel = context.WithCancel(context.Background())

	for i := uint64(0); i < options.RoutineSize; i++ {
		sess.sessionChan[i] = make(chan sessionDataCtx, options.RoutineChan)
		go sess.process(sess.sessionChan[i])
	}
	return sess, nil
}

func (d *Dao) watch(c zrpc.RpcClientConf) {
	update := func(values []string) {
		if len(values) == 0 {
			return
		}

		d.mu.Lock()

		sessions := map[string]SessionPusher{}
		for _, v := range values {
			if old, ok := d.sessionServers[v]; ok {
				sessions[v] = old
				continue
			}

			var (
				cli SessionPusher
				err error
			)
			if d.useStreamSession {
				cli, err = NewStreamingSession(v)
			} else {
				c.Endpoints = []string{v}
				cli, err = NewSession(c, SessionOptions{
					RoutineSize: d.conf.Routine.Size,
					RoutineChan: d.conf.Routine.Chan,
				})
			}
			if err != nil {
				d.mu.Unlock()
				logx.Errorf("watchComet NewClient(%+v) error(%v)", values, err)
				return
			}
			sessions[v] = cli
		}

		var removed []SessionPusher
		for key, old := range d.sessionServers {
			if _, ok := sessions[key]; !ok {
				removed = append(removed, old)
				logx.Infof("watchComet DelComet:%s", key)
			}
		}

		d.sessionServers = sessions
		d.mu.Unlock()

		// close removed sessions outside of lock
		for _, old := range removed {
			old.Close()
		}
	}

	if err := discovery.Watch(c.Etcd, c.Endpoints, update); err != nil {
		logx.Errorf("watch sessions(%+v) error: %v", c, err)
	}
}

func (d *Dao) PushUpdatesToSession(ctx context.Context, serverId string, msg *session.TLSessionPushUpdatesData) (err error) {
	d.mu.RLock()
	pusher, ok := d.sessionServers[serverId]
	d.mu.RUnlock()

	if ok {
		return pusher.PushUpdates(ctx, msg)
	}
	logx.WithContext(ctx).Errorf("PushUpdatesToSession - stale gateway, serverId %s not in active sessions (permAuthKeyId:%d)",
		serverId, msg.PermAuthKeyId)
	return fmt.Errorf("stale gateway %s", serverId)
}

func (d *Dao) PushSessionUpdatesToSession(ctx context.Context, serverId string, msg *session.TLSessionPushSessionUpdatesData) (err error) {
	d.mu.RLock()
	pusher, ok := d.sessionServers[serverId]
	d.mu.RUnlock()

	if ok {
		return pusher.PushSessionUpdates(ctx, msg)
	}
	logx.WithContext(ctx).Errorf("PushSessionUpdatesToSession - stale gateway, serverId %s not in active sessions (permAuthKeyId:%d)",
		serverId, msg.PermAuthKeyId)
	return fmt.Errorf("stale gateway %s", serverId)
}

func (d *Dao) PushRpcResultToSession(ctx context.Context, serverId string, msg *session.TLSessionPushRpcResultData) (err error) {
	d.mu.RLock()
	pusher, ok := d.sessionServers[serverId]
	d.mu.RUnlock()

	if ok {
		return pusher.PushRpcResult(ctx, msg)
	}
	logx.WithContext(ctx).Errorf("PushRpcResultToSession - stale gateway, serverId %s not in active sessions (permAuthKeyId:%d)",
		serverId, msg.PermAuthKeyId)
	return fmt.Errorf("stale gateway %s", serverId)
}
