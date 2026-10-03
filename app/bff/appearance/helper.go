// Package appearance_helper answers chat themes and name colours (#23, #24).
//
// The themes are the colours in teamgramd/appearance - Android's own palettes,
// with no file behind any of them - and a theme is set for a chat between two:
// both sides keep it and both are told with a service message, the way
// Telegram tells them. A person's name and profile colour are kept on their
// account and travel with their user object.
//
// The lists and their shape live in pkg/appearance; this is how they are
// offered to a phone.
package appearance_helper

import (
	"github.com/teamgram/teamgram-server/app/bff/appearance/internal/config"
	"github.com/teamgram/teamgram-server/app/bff/appearance/internal/server/grpc/service"
	"github.com/teamgram/teamgram-server/app/bff/appearance/internal/svc"
)

type Config = config.Config

func New(c Config) *service.Service {
	return service.New(svc.NewServiceContext(c))
}
