package core

import (
	"github.com/teamgram/proto/mtproto"
)

// refuseSavedMessages answers anything written into Saved Messages with
// CHAT_WRITE_FORBIDDEN (#271).
//
// Saved Messages has no conversation to encrypt with, so whatever goes there
// is kept in the clear: a note, a photo, or a forward out of a chat that
// encrypts, which iOS sends as its opened words. Nothing may be (the owner's
// word of 16 October). This holds every installed build at once, the way
// drafts are held (#264); the clients hide the way in, and Saved Messages
// comes back when it encrypts for the account's own devices. A refusal a
// client shows, not a 500 it would send again.
func refuseSavedMessages(userId int64, peer *mtproto.PeerUtil) error {
	if peer.IsSelfUser(userId) {
		return mtproto.ErrChatWriteForbidden
	}
	return nil
}
