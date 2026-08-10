// Package mls_helper is the directory of MLS key packages, as the server offers
// it to a phone.
//
// It lives in the same process as the other bff services and answers two
// methods: leave a supply of key packages, and take one per device of a person.
// The rules it enforces - used once, an empty supply falls back rather than
// refusing, one device cannot fill the table - are in pkg/mls, away from both
// the database and the protocol, where they can be read and tested on their own.
package mls_helper

import (
	"github.com/teamgram/teamgram-server/app/bff/mls/internal/config"
	"github.com/teamgram/teamgram-server/app/bff/mls/internal/server/grpc/service"
	"github.com/teamgram/teamgram-server/app/bff/mls/internal/svc"
)

type Config = config.Config

func New(c Config) *service.Service {
	return service.New(svc.NewServiceContext(c))
}
