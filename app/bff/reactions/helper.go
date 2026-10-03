// Package reactions_helper answers reactions to messages (#18).
//
// The set is the owner's nine emoji, drawn by the pictures in
// teamgramd/reactions; one reaction per person per message, kept in the open
// beside the encrypted message it answers. What one person gives reaches
// everybody else in the conversation at once, as each of them sees it, and a
// phone that was away reads it with the history or asks for the messages it
// is showing - as Telegram's own clients do, without a place in the update
// sequence.
//
// The rules - who sees what, the order, the pictures - live in pkg/reactions;
// this is how they are offered to a phone.
package reactions_helper

import (
	"github.com/teamgram/teamgram-server/app/bff/reactions/internal/config"
	"github.com/teamgram/teamgram-server/app/bff/reactions/internal/server/grpc/service"
	"github.com/teamgram/teamgram-server/app/bff/reactions/internal/svc"
)

type Config = config.Config

func New(c Config) *service.Service {
	return service.New(svc.NewServiceContext(c))
}
