/*
 * Created from 'scheme.tl' by 'mtprotoc'
 *
 * Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
 *  All rights reserved.
 *
 * Author: teamgramio (teamgram.io@gmail.com)
 */

package core

import (
	"context"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/rpc/metadata"
	"github.com/teamgram/teamgram-server/app/interface/session/session"
	"github.com/teamgram/teamgram-server/app/messenger/sync/internal/svc"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
	"github.com/teamgram/teamgram-server/app/service/status/status"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type SyncType int

const (
	syncTypeUser      SyncType = 1 // 该用户所有设备
	syncTypeUserNotMe SyncType = 2 // 该用户除了某个设备
	syncTypeUserMe    SyncType = 3 // 该用户指定某个设备
)

type SyncCore struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	MD *metadata.RpcMetadata
}

func New(ctx context.Context, svcCtx *svc.ServiceContext) *SyncCore {
	return &SyncCore{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
		MD:     metadata.RpcMetadataFromIncoming(ctx),
	}
}

func (c *SyncCore) processUpdates(syncType SyncType, userId int64, isBot bool, ups *mtproto.Updates) (needPush bool, err error) {
	mtproto.VisitUpdates(userId, ups, map[string]mtproto.UpdateVisitedFunc{
		mtproto.Predicate_updateNewMessage: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			// c.svcCtx.Dao.AddToPtsQueue(c.ctx, userId, update.Pts_INT32, update.PtsCount, update)
			// removed in pure push mode
			needPush = true
		},
		mtproto.Predicate_updateDeleteMessages: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			// c.svcCtx.Dao.AddToPtsQueue(c.ctx, userId, update.Pts_INT32, update.PtsCount, update)
			// removed in pure push mode
			needPush = true
		},
		mtproto.Predicate_updateReadHistoryInbox: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			// c.svcCtx.Dao.AddToPtsQueue(c.ctx, userId, update.Pts_INT32, update.PtsCount, update)
			// removed in pure push mode
			needPush = true
		},
		mtproto.Predicate_updateReadHistoryOutbox: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			// c.svcCtx.Dao.AddToPtsQueue(c.ctx, userId, update.Pts_INT32, update.PtsCount, update)
			// removed in pure push mode
			needPush = true
		},
		mtproto.Predicate_updateWebPage: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			// c.svcCtx.Dao.AddToPtsQueue(c.ctx, userId, update.Pts_INT32, update.PtsCount, update)
			// removed in pure push mode
			needPush = true
		},
		mtproto.Predicate_updateReadMessagesContents: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			// c.svcCtx.Dao.AddToPtsQueue(c.ctx, userId, update.Pts_INT32, update.PtsCount, update)
			// removed in pure push mode
			needPush = true
		},
		mtproto.Predicate_updateEditMessage: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			// c.svcCtx.Dao.AddToPtsQueue(c.ctx, userId, update.Pts_INT32, update.PtsCount, update)
			// removed in pure push mode
			needPush = true
		},
		mtproto.Predicate_updateFolderPeers: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			if syncType == syncTypeUserNotMe {
				// c.svcCtx.Dao.AddToPtsQueue(c.ctx, userId, update.Pts_INT32, update.PtsCount, update)
				// removed in pure push mode
			}
		},
		mtproto.Predicate_updatePinnedMessages: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			if syncType == syncTypeUserNotMe {
				// c.svcCtx.Dao.AddToPtsQueue(c.ctx, userId, update.Pts_INT32, update.PtsCount, update)
				// removed in pure push mode
			}
		},
		mtproto.Predicate_updatePhoneCall: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			if update.GetPhoneCall().GetPredicateName() == mtproto.Predicate_phoneCallRequested {
				// log.Debugf("recv phoneCallRequested")
				needPush = true
			}
		},
		mtproto.Predicate_updatePeerSettings: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			needPush = true
		},
	})

	return needPush, nil
}

// How long a push to a session may take before it is called lost, and said so.
//
// Bounded because it was not. On 31 August the live server worked out 193
// deliveries for sessions it believed were online and 192 of them vanished
// without a line in the log: the session client had stopped answering, every
// call waited on it for ever, and the error was thrown away by `_ =`. Clients
// then saw only what they asked for - which is how the first thing said in a
// new group, the one message nobody has asked about yet, became invisible for
// good (#144).
//
// Three seconds is long for a call to a service on the same machine and short
// enough that nothing is held behind it.
const pushDeadline = 3 * time.Second

// Sends one push and says when it does not arrive.
//
// The wording is not free either: "dropping push" is what
// deploy/check-health.py already watches for every fifteen minutes, and it
// never fired because a call that hangs writes nothing at all. A delivery that
// fails silently is one nobody hears about until somebody reports a message
// that never came.
func (c *SyncCore) deliver(what string, keyId int64, serverId string, push func(context.Context) error) {
	ctx, cancel := context.WithTimeout(c.ctx, pushDeadline)
	defer cancel()
	if err := push(ctx); err != nil {
		c.Logger.Errorf("dropping push: %s to %d on %s: %v", what, keyId, serverId, err)
	}
}

func (c *SyncCore) pushUpdatesToSession(syncType SyncType, userId, permAuthKeyId int64, hasServerId *wrapperspb.StringValue, authKeyId, sessionId *wrapperspb.Int64Value, pushData *mtproto.Updates, notification bool) {
	if syncType == syncTypeUserMe && hasServerId != nil {
		c.Logger.Debugf("pushUpdatesToSession - pushData: {server_id: %v, auth_key_id: %v}", hasServerId, authKeyId)
		if sessionId != nil {
			c.deliver("session updates", permAuthKeyId, hasServerId.GetValue(),
				func(ctx context.Context) error {
					return c.svcCtx.Dao.PushSessionUpdatesToSession(
						ctx,
						hasServerId.GetValue(),
						&session.TLSessionPushSessionUpdatesData{
							PermAuthKeyId: permAuthKeyId,
							AuthKeyId:     authKeyId.GetValue(),
							SessionId:     sessionId.GetValue(),
							Updates:       pushData,
						})
				})
		} else {
			c.deliver("updates", permAuthKeyId, hasServerId.GetValue(),
				func(ctx context.Context) error {
					return c.svcCtx.Dao.PushUpdatesToSession(
						ctx,
						hasServerId.GetValue(),
						&session.TLSessionPushUpdatesData{
							PermAuthKeyId: permAuthKeyId,
							Notification:  notification,
							Updates:       pushData,
						})
				})
		}
	} else {
		var (
			pushExcludeList   = make([]int64, 0)
			serverIdKeyIdList = make(map[string][]int64)
			// Kept apart from pushExcludeList: that one lists sessions worth a
			// delivery attempt over the connection, this one only those where the
			// app is certainly open. The difference matters for notifications,
			// see below.
			activeKeyIdList = make([]int64, 0)
			now         = time.Now().Unix()
		)

		statusList, _ := c.svcCtx.Dao.StatusClient.StatusGetUserOnlineSessions(c.ctx, &status.TLStatusGetUserOnlineSessions{
			UserId: userId,
		})
		c.Logger.Debugf("statusList - #%v", statusList)
		for _, sess := range statusList.GetUserSessions() {
			if syncType == syncTypeUserNotMe && sess.AuthKeyId == permAuthKeyId {
				continue
			}
			pushExcludeList = append(pushExcludeList, sess.PermAuthKeyId)
			// The session list comes without regard to expiry, and expiry is what
			// decides here: a backgrounded iOS app keeps the connection for a
			// while yet shows no messages. Treating such a session as live means
			// never sending a notification at all.
			if sess.Expired > now {
				activeKeyIdList = append(activeKeyIdList, sess.PermAuthKeyId)
			}
			if keyIdList, ok := serverIdKeyIdList[sess.Gateway]; ok {
				keyIdList = append(keyIdList, sess.AuthKeyId)
				serverIdKeyIdList[sess.Gateway] = keyIdList
			} else {
				serverIdKeyIdList[sess.Gateway] = []int64{sess.AuthKeyId}
			}
		}

		c.Logger.Debugf("serverIdKeyIdList - #%v", serverIdKeyIdList)
		for serverId, keyIdList := range serverIdKeyIdList {
			for _, keyId := range keyIdList {
				// log.Debugf("serverIdKeyIdList - #%v", serverIdKeyIdList)
				c.deliver("updates", keyId, serverId, func(ctx context.Context) error {
					return c.svcCtx.Dao.PushUpdatesToSession(
						ctx,
						serverId,
						&session.TLSessionPushUpdatesData{
							PermAuthKeyId: keyId,
							Notification:  notification,
							Updates:       pushData,
						})
				})
			}
		}

		if syncType == syncTypeUser {
			if c.svcCtx.Dao.PushClient != nil {
				c.Logger.Debugf("push PushClient...")
				// Said out loud for the same reason as the rest: the fourth
				// discarded error is the one that hides while the other three
				// are watched.
				if _, err := c.svcCtx.Dao.PushClient.SyncPushUpdatesIfNot(c.ctx, &sync.TLSyncPushUpdatesIfNot{
					UserId:   userId,
					Excludes: pushExcludeList,
					Updates:  pushData,
				}); err != nil {
					c.Logger.Errorf("dropping push: notification for %d: %v", userId, err)
				}
			}

			// The notification goes to devices where the app is not open. Only a
			// session that has not expired counts as open.
			c.notifyOfflineDevices(userId, activeKeyIdList, pushData)
		}
	}
}
