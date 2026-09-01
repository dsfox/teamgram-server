// Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
//  All rights reserved.
//
// Author: teamgramio (teamgram.io@gmail.com)
//

package dao

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/teamgram/teamgram-server/app/interface/gnetway/internal/config"
	sessionclient "github.com/teamgram/teamgram-server/app/interface/session/client"
	"github.com/teamgram/teamgram-server/pkg/discovery"

	"github.com/zeromicro/go-zero/core/hash"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stringx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxNodeFailures = 3 // consecutive failures before removing node from ring
)

// nodeReturnsAfter is how long a node stays out of the ring before it is tried
// again.
//
// It used to stay out for ever, and nothing anywhere put it back. With direct
// addresses and no etcd - which is how this server runs - `discovery.Watch`
// calls `update` once at startup and returns; the whole function is an if and a
// call, so the only path that adds a node never runs a second time. On a
// one-node install the ring is then empty, `dispatcher.Get` finds nothing, and
// every request is answered `not found session` until somebody restarts the
// container: nobody can sign in, and nobody can write (#150).
//
// Taking a failing node out is still right - with several of them the traffic
// should go round it - so it goes out, and comes back to be tried. Five
// seconds, which is what sync waits before trusting a session again (#146).
//
// A var rather than a const so a test can make the pause short enough to watch.
var nodeReturnsAfter = 5 * time.Second

var (
	ErrSessionNotFound = errors.New("not found session")
)

type ShardingSessionClient struct {
	mu           sync.RWMutex
	gatewayId    string
	dispatcher   *hash.ConsistentHash
	sessions     map[string]sessionclient.SessionClient
	failCounters map[string]int
	// When each node that has been taken out of the ring may be tried again.
	// The client itself stays in `sessions`: zrpc reconnects underneath, so
	// what is being paused is the routing, not the connection.
	sidelined map[string]time.Time
}

func NewShardingSessionClient(c config.Config) *ShardingSessionClient {
	sess := &ShardingSessionClient{
		dispatcher:   hash.NewConsistentHash(),
		sessions:     make(map[string]sessionclient.SessionClient),
		failCounters: make(map[string]int),
		sidelined:    make(map[string]time.Time),
	}
	sess.watch(c.Session)

	return sess
}

func (sess *ShardingSessionClient) watch(c zrpc.RpcClientConf) {
	update := func(values []string) {
		var (
			addClis    []string
			removeClis []string
		)

		sess.mu.Lock()
		defer sess.mu.Unlock()

		sessions := map[string]sessionclient.SessionClient{}
		for _, v := range values {
			if old, ok := sess.sessions[v]; ok {
				sessions[v] = old
				continue
			}
			c.Endpoints = []string{v}
			cli, err := zrpc.NewClient(
				c,
				zrpc.WithDialOption(grpc.WithReadBufferSize(16*1024*1024)),
				zrpc.WithDialOption(grpc.WithWriteBufferSize(16*1024*1024)),
			)
			if err != nil {
				logx.Errorf("watchSession NewClient(%v) error: %v", v, err)
				continue
			}
			sessionCli := sessionclient.NewSessionClient(cli)
			sessions[v] = sessionCli

			addClis = append(addClis, v)
		}

		for key, _ := range sess.sessions {
			if !stringx.Contains(values, key) {
				removeClis = append(removeClis, key)
			}
		}

		for _, n := range addClis {
			sess.dispatcher.Add(n)
		}

		for _, n := range removeClis {
			sess.dispatcher.Remove(n)
			delete(sess.failCounters, n)
		}

		sess.sessions = sessions
	}

	if err := discovery.Watch(c.Etcd, c.Endpoints, update); err != nil {
		logx.Errorf("watchSession(%+v) error: %v", c, err)
	}
}

// bringBackWhatIsDue puts a sidelined node into the ring once its pause is over.
//
// Asked before every dispatch rather than on a timer of its own: there is no
// other moment: the one thing that used to add nodes runs at startup and never
// again, which is why a node taken out stayed out for ever (#150).
//
// The read is done under the read lock and the write only when there is
// something to write, so the ordinary call - nothing sidelined - costs a map
// length and no contention.
func (sess *ShardingSessionClient) bringBackWhatIsDue() {
	now := time.Now()

	sess.mu.RLock()
	due := false
	for _, when := range sess.sidelined {
		if !now.Before(when) {
			due = true
			break
		}
	}
	sess.mu.RUnlock()
	if !due {
		return
	}

	sess.mu.Lock()
	for node, when := range sess.sidelined {
		if now.Before(when) {
			continue
		}
		delete(sess.sidelined, node)
		if _, ok := sess.sessions[node]; !ok {
			// Taken away by an update rather than by a failure. Nothing to
			// bring back, and adding it would put a name in the ring with no
			// client behind it.
			continue
		}
		sess.dispatcher.Add(node)
		logx.Infof("session node %s is back in the ring after %s", node, nodeReturnsAfter)
	}
	sess.mu.Unlock()
}

func (sess *ShardingSessionClient) InvokeByKey(key string, cb func(client sessionclient.SessionClient) (err error)) error {
	if cb == nil {
		return nil
	}

	sess.bringBackWhatIsDue()

	sess.mu.RLock()
	val, ok := sess.dispatcher.Get(key)
	if !ok {
		sess.mu.RUnlock()
		return ErrSessionNotFound
	}

	node := val.(string)
	cli, ok := sess.sessions[node]
	sess.mu.RUnlock()

	if !ok {
		return ErrSessionNotFound
	}

	err := cb(cli)
	if err == nil {
		// reset failure counter on success
		sess.mu.Lock()
		delete(sess.failCounters, node)
		sess.mu.Unlock()
		return nil
	}

	// redirect 错误：session 告知正确节点，直接重路由
	if target, ok := parseRedirectError(err); ok {
		logx.Infof("session node %s redirected to %s for key %s", node, target, key)

		sess.mu.RLock()
		redirectCli, ok := sess.sessions[target]
		sess.mu.RUnlock()

		if ok {
			return cb(redirectCli)
		}
		// 目标节点不在已知列表中，按连接错误处理走 fallback
	}

	// 非连接错误直接返回
	if !isConnError(err) {
		return err
	}

	// 连接错误：增加失败计数，达到阈值才从哈希环移除
	sess.mu.Lock()
	sess.failCounters[node]++
	failCount := sess.failCounters[node]
	if failCount >= maxNodeFailures {
		logx.Errorf("session node %s unreachable (%d consecutive failures), out of the ring for %s", node, failCount, nodeReturnsAfter)
		sess.dispatcher.Remove(node)
		delete(sess.failCounters, node)
		// The client stays: it is what makes putting the node back possible,
		// and zrpc reconnects behind it on its own.
		sess.sidelined[node] = time.Now().Add(nodeReturnsAfter)
	} else {
		logx.Errorf("session node %s connection error (%d/%d), retrying on another node", node, failCount, maxNodeFailures)
	}
	sess.mu.Unlock()

	sess.mu.RLock()
	newVal, ok := sess.dispatcher.Get(key)
	if !ok {
		sess.mu.RUnlock()
		return ErrSessionNotFound
	}
	newNode := newVal.(string)
	newCli, ok := sess.sessions[newNode]
	sess.mu.RUnlock()

	if !ok {
		return ErrSessionNotFound
	}

	return cb(newCli)
}

// parseRedirectError 解析 session 返回的 redirect 错误，提取目标节点地址
// 错误格式：gRPC code 700, message "REDIRECT_TO_{server_addr}"
func parseRedirectError(err error) (string, bool) {
	s, ok := status.FromError(err)
	if !ok {
		return "", false
	}
	if s.Code() != codes.Code(700) {
		return "", false
	}
	msg := s.Message()
	const prefix = "REDIRECT_TO_"
	if strings.HasPrefix(msg, prefix) {
		target := msg[len(prefix):]
		if target != "" {
			return target, true
		}
	}
	return "", false
}

// isConnError 判断是否为连接级错误（节点不可达）
func isConnError(err error) bool {
	s, ok := status.FromError(err)
	if !ok {
		return false
	}
	switch s.Code() {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	}
	return false
}
