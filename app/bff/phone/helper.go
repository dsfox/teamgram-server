// Package phone_helper matches two phones for a call and gets out of the way
// (#14). It lives in the same process as the other bff services.
//
// The server is the matchmaker, not the wire: it carries the key exchange
// between the two devices and tells them which STUN to try, and the media goes
// device to device. A relay is the fallback for the one case a direct path
// cannot win, and it only ever forwards ciphertext.
package phone_helper

import (
	"github.com/teamgram/teamgram-server/app/bff/phone/internal/config"
	"github.com/teamgram/teamgram-server/app/bff/phone/internal/server/grpc/service"
	"github.com/teamgram/teamgram-server/app/bff/phone/internal/svc"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

type Config = config.Config

// New builds the service around one registry of the calls in the air. The
// registry is made by whoever runs the process and shared with the updates
// service, which rings a device that comes back while a call still rings.
func New(c Config, registry *calls.Registry) *service.Service {
	return service.New(svc.NewServiceContext(c, registry))
}

// NewDhService answers messages.getDhConfig, the group a phone asks for before
// a call; it is registered as the RPCSecretChats server, that being where the
// schema keeps the method.
func NewDhService() *service.DhService {
	return service.NewDhService()
}

// NewRegistry is the registry of calls in the air, one per process.
func NewRegistry() *calls.Registry {
	return calls.NewRegistry()
}

// Calls is the section of the bff config this service reads: the STUN and
// relay endpoints a phone is told to try, and the secret behind the
// short-lived relay credentials.
type Calls = config.Calls
