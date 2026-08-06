/*
 * WARNING! All changes made in this file will be lost!
 * Created from 'scheme.tl' by 'mtprotoc'
 *
 * Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
 *  All rights reserved.
 *
 * Author: teamgramio (teamgram.io@gmail.com)
 */

package inbox_helper

import (
	kafka "github.com/teamgram/marmota/pkg/mq"
	"github.com/teamgram/teamgram-server/app/messenger/msg/inbox/internal/config"
	"github.com/teamgram/teamgram-server/app/messenger/msg/inbox/internal/core"
	"github.com/teamgram/teamgram-server/app/messenger/msg/inbox/inbox"
	"github.com/teamgram/teamgram-server/app/messenger/msg/inbox/internal/server/grpc/service"
	"github.com/teamgram/teamgram-server/app/messenger/msg/inbox/internal/server/mq"
	"github.com/teamgram/teamgram-server/app/messenger/msg/inbox/internal/svc"

	"google.golang.org/grpc"
)

type (
	Config         = config.Config
	ServiceContext = svc.ServiceContext
	InboxCore      = core.InboxCore
)

var (
	NewServiceContext = svc.NewServiceContext
	NewInboxCore      = core.New
)

func New(c Config) *kafka.ConsumerGroup {
	return mq.New(svc.NewServiceContext(c), c.InboxConsumer)
}

// RegisterServer поднимает приём работы inbox по gRPC — альтернатива очереди.
// Регистрируется на общем сервере процесса msg: они всё равно живут вместе.
func RegisterServer(grpcServer *grpc.Server, c Config) {
	inbox.RegisterRPCInboxServer(grpcServer, service.New(svc.NewServiceContext(c)))
}
