/*
 * Created from 'scheme.tl' by 'mtprotoc'
 *
 * Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
 *  All rights reserved.
 *
 * Author: teamgramio (teamgram.io@gmail.com)
 */

package dao

import (
	"fmt"
	"sync"

	"github.com/teamgram/marmota/pkg/net/rpcx"
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	sync_client "github.com/teamgram/teamgram-server/app/messenger/sync/client"
	"github.com/teamgram/teamgram-server/pkg/pushnotify"
	"github.com/teamgram/teamgram-server/pkg/queue"
	"github.com/teamgram/teamgram-server/app/messenger/sync/internal/config"
	chat_client "github.com/teamgram/teamgram-server/app/service/biz/chat/client"
	idgen_client "github.com/teamgram/teamgram-server/app/service/idgen/client"
	status_client "github.com/teamgram/teamgram-server/app/service/status/client"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/kv"
	"github.com/zeromicro/go-zero/zrpc"
)

type Dao struct {
	*Mysql
	kv               kv.Store
	conf             *config.Config
	mu               sync.RWMutex
	sessionServers   map[string]SessionPusher
	useStreamSession bool
	idgen_client.IDGenClient2
	status_client.StatusClient
	chat_client.ChatClient
	PushClient sync_client.SyncClient
	// Notifier reaches the devices where the app is closed.
	Notifier *pushnotify.Notifier
}

// refuseTheStreamingPath says why sync will not run on the streaming session
// client, or nil when it is not asked to.
//
// The path is broken and nothing measures it (#149). Its `unavailable` latch is
// only ever set: `sendLoop` and `recvLoop` return when the stream breaks, so
// afterwards nobody drains the channel, and merely clearing the latch would
// replace an honest refusal with silent loss. No scenario and no test touch it,
// and it appears in no deployed config.
//
// Upstream's file is left exactly as it is - it is theirs, and every carry would
// bring it back - so what is ours is the refusal to start on it. A switch that
// is off by default and known broken is worse than one that is not there: the
// day somebody turns it on, delivery stops in a way that looks like anything
// else.
func refuseTheStreamingPath(on bool) error {
	if !on {
		return nil
	}
	return fmt.Errorf(
		"sync: UseStreamSession is on, and that path does not work (#149) - " +
			"a broken stream ends both of its goroutines and nothing reopens " +
			"them, so pushes would be accepted and lost. Turn it off, or close " +
			"#149 first")
}

func New(c config.Config) *Dao {
	logx.Must(refuseTheStreamingPath(c.UseStreamSession))

	db := sqlx.NewMySQL(&c.Mysql)
	d := &Dao{
		Mysql:            newMysqlDao(db),
		Notifier:         pushnotify.New(db),
		kv:               kv.NewStore(c.KV),
		conf:             &c,
		sessionServers:   make(map[string]SessionPusher),
		useStreamSession: c.UseStreamSession,
		IDGenClient2:     idgen_client.NewIDGenClient2(zrpc.MustNewClient(c.IdgenClient)),
		StatusClient:     status_client.NewStatusClient(zrpc.MustNewClient(c.StatusClient)),
		ChatClient:       chat_client.NewChatClient(rpcx.GetCachedRpcClient(c.ChatClient)),
	}
	if c.PushClient != nil {
		d.PushClient = queue.NewSyncClient(c.PushClient)
	}

	go d.watch(c.SessionClient)
	return d
}
