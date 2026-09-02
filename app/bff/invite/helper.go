// Package invite_helper mints invitations for the people who vouch for them
// (#47). It lives in the same process as the other bff services and answers
// one method: a code bound to a number, for the member who asked.
package invite_helper

import (
	"github.com/teamgram/teamgram-server/app/bff/invite/internal/config"
	"github.com/teamgram/teamgram-server/app/bff/invite/internal/server/grpc/service"
	"github.com/teamgram/teamgram-server/app/bff/invite/internal/svc"
)

type Config = config.Config

func New(c Config) *service.Service {
	return service.New(svc.NewServiceContext(c))
}
