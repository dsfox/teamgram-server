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

package config

import (
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/pkg/queue"
	"github.com/teamgram/teamgram-server/pkg/code/conf"
	"github.com/zeromicro/go-zero/core/stores/kv"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	KV kv.KvConf
	// The only place where bff touches the database directly: device tokens for
	// notifications. There is nowhere else to put them — the app reports the
	// token via account.registerDevice, which lands right here, and the build has
	// no separate notification service.
	Mysql                     sqlx.Config `json:",optional"`
	Code                      *conf.SmsVerifyCodeConfig
	BizServiceClient          zrpc.RpcClientConf
	AuthSessionClient         zrpc.RpcClientConf
	MediaClient               zrpc.RpcClientConf
	IdgenClient               zrpc.RpcClientConf
	MsgClient                 zrpc.RpcClientConf
	SyncClient                *queue.Conf
	DfsClient                 zrpc.RpcClientConf
	StatusClient              zrpc.RpcClientConf
	SignInServiceNotification []conf.MessageEntityConfig `json:",optional"`
	SignInMessage             []conf.MessageEntityConfig `json:",optional"`
}
