package core

import (
	"strings"

	"github.com/teamgram/proto/mtproto"
)

// Saved Messages takes ciphertext and nothing else (#271, #284).
//
// It had no conversation to encrypt with, so whatever went there was kept in
// the clear: a note, a photo, or a forward out of a chat that encrypts, which
// iOS sent as its opened words. Nothing may be (the owner's word of 16
// October). The clients now keep Saved Messages in a conversation with the
// account's own devices and send it what they send any chat that encrypts:
// words as an mls1: ciphertext, and a file as the blob that ciphertext holds
// the key to - a document named "file" of type application/octet-stream,
// with no thumbnail. Anything else is CHAT_WRITE_FORBIDDEN, as every write
// was before: that holds the builds installed before this one, which send
// plain words, and a forward, which the server would copy as it is. A
// refusal a client shows, not a 500 it would send again.

const ciphertextPrefix = "mls1:"

// isCiphertext is whether text is what the clients' MLS layer writes.
func isCiphertext(text string) bool {
	return strings.HasPrefix(text, ciphertextPrefix) && len(text) > len(ciphertextPrefix)
}

// isBlob is whether media is the upload of an encrypted file, as the clients
// make it: nothing about it describes what it holds.
func isBlob(media *mtproto.InputMedia) bool {
	if media == nil || media.GetPredicateName() != mtproto.Predicate_inputMediaUploadedDocument {
		return false
	}
	if media.GetMimeType() != "application/octet-stream" || media.GetThumb() != nil {
		return false
	}
	attributes := media.GetAttributes()
	if len(attributes) != 1 || attributes[0].GetPredicateName() != mtproto.Predicate_documentAttributeFilename {
		return false
	}
	return attributes[0].GetFileName() == "file"
}

// refuseSavedMessages refuses a message into Saved Messages that is not
// ciphertext, with a file only as a blob; nil when the peer is somebody
// else. media is nil for a text.
func refuseSavedMessages(userId int64, peer *mtproto.PeerUtil, text string, media *mtproto.InputMedia) error {
	if !peer.IsSelfUser(userId) {
		return nil
	}
	if !isCiphertext(text) {
		return mtproto.ErrChatWriteForbidden
	}
	if media != nil && !isBlob(media) {
		return mtproto.ErrChatWriteForbidden
	}
	return nil
}

// refuseSavedEdit refuses an edit in Saved Messages that brings anything but
// a ciphertext and a blob. An edit carries only what changes, so each part
// it carries is held to what a new message would be.
func refuseSavedEdit(userId int64, peer *mtproto.PeerUtil, text *string, media *mtproto.InputMedia) error {
	if !peer.IsSelfUser(userId) {
		return nil
	}
	if text == nil && media == nil {
		return mtproto.ErrChatWriteForbidden
	}
	if text != nil && !isCiphertext(*text) {
		return mtproto.ErrChatWriteForbidden
	}
	if media != nil && !isBlob(media) {
		return mtproto.ErrChatWriteForbidden
	}
	return nil
}

// refuseCopyIntoSavedMessages refuses what the server would write into
// Saved Messages out of what it already holds: a forward. The clients send a
// forward into a chat that encrypts again, as a ciphertext of its own.
func refuseCopyIntoSavedMessages(userId int64, peer *mtproto.PeerUtil) error {
	if peer.IsSelfUser(userId) {
		return mtproto.ErrChatWriteForbidden
	}
	return nil
}
