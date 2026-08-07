/*
 * Created from 'scheme.tl' by 'mtprotoc'
 *
 * Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
 *  All rights reserved.
 *
 * Author: teamgramio (teamgram.io@gmail.com)
 */

package server

import (
	"flag"

	kafka "github.com/teamgram/marmota/pkg/mq"
	"github.com/teamgram/teamgram-server/app/messenger/sync/internal/config"
	"github.com/teamgram/teamgram-server/app/messenger/sync/internal/server/grpc"
	"github.com/teamgram/teamgram-server/app/messenger/sync/internal/server/mq"
	"github.com/teamgram/teamgram-server/app/messenger/sync/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
)

var configFile = flag.String("f", "etc/sync.yaml", "the config file")

type Server struct {
	grpcSrv *zrpc.RpcServer
	mq      *kafka.ConsumerGroup
}

func New() *Server {
	return new(Server)
}

func (s *Server) Initialize() error {
	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	logx.Infov(c)

	if err := logx.SetUp(c.Log); err != nil {
		return err
	}

	ctx := svc.NewServiceContext(c)

	// Приём работы напрямую по gRPC — так очередь перестаёт быть обязательной.
	s.grpcSrv = grpc.New(ctx, c.RpcServerConf)
	go s.grpcSrv.Start()

	// Очередь поднимаем, только если она настроена.
	if len(c.SyncConsumer.Brokers) > 0 {
		s.mq = mq.New(ctx, c.SyncConsumer)
		go s.mq.Start()
	}

	return nil
}

func (s *Server) RunLoop() {
}

func (s *Server) Destroy() {
	s.grpcSrv.Stop()
	if s.mq != nil {
		s.mq.Stop()
	}
}
