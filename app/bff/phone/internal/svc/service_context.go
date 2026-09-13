package svc

import (
	"github.com/teamgram/teamgram-server/app/bff/phone/internal/config"
	sync_client "github.com/teamgram/teamgram-server/app/messenger/sync/client"
	"github.com/teamgram/teamgram-server/pkg/calls"
	"github.com/teamgram/teamgram-server/pkg/queue"
)

// ServiceContext is deliberately thin: the calls in the air, and the one way
// out - telling the other phone. There is no media path here to hold.
type ServiceContext struct {
	Config config.Config
	// Says "you are being called" to the devices of a person.
	SyncClient sync_client.SyncClient
	// The calls currently in the air.
	Registry *calls.Registry
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:     c,
		SyncClient: queue.NewSyncClient(c.SyncClient),
		Registry:   calls.NewRegistry(),
	}
}
