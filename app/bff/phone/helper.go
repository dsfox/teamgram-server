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
)

type Config = config.Config

func New(c Config) *service.Service {
	return service.New(svc.NewServiceContext(c))
}

// Calls is the section of the bff config this service reads: the STUN and
// relay endpoints a phone is told to try, and the secret behind the
// short-lived relay credentials.
type Calls = config.Calls
