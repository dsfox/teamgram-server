package core

import (
	"github.com/teamgram/proto/mtproto"
)

// maxStoredMessage is what messages.message holds: TEXT, 65535 bytes
// (deploy/sql-patches/18-a-ciphertext-fits-its-column.sql). In a chat that
// encrypts the text is a ciphertext nearly three times the length of its
// Cyrillic words, so the limit is the column's, not the 4096 characters a
// person can type (#269).
const maxStoredMessage = 65535

// checkStoredLength answers a text the column cannot hold with MESSAGE_TOO_LONG,
// which a client shows, rather than the database's refusal, which reaches the
// client as a 500 and is sent again for good.
func checkStoredLength(text string) error {
	if len(text) > maxStoredMessage {
		return mtproto.ErrMessageTooLong
	}
	return nil
}
