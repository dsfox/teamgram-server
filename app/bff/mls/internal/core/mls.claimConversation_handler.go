package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
)

// MlsClaimConversation says which conversation a chat has, and settles it the
// first time somebody asks.
//
// Nothing settled it before: every device that wanted to send into a chat with
// no conversation started one of its own. Between two people that almost always
// lands on one; in a group where three people begin within a minute it does
// not, and they end up in conversations that cannot read each other, with no
// way back - three people did exactly that on 30 August (#135).
//
// The devices cannot settle it among themselves. Whoever loses has to be told,
// and when everybody is offline and arrives in a random order there is nobody
// to tell them. So the first claim wins here, atomically, and every device
// after it is answered with the same conversation.
//
// This tells the server which chat a conversation belongs to, which it was
// deliberately not told before. It learns nothing by it: the group id travels
// in the clear in the header of every message, so this mapping could always be
// read off any one of them.
//
// The first claim is not the last word, because it was wrong once and nothing
// could take it back. A device rebuilding a conversation on a misreading won the
// answer for one chat on the stand and nobody followed it there: everybody talks
// in another conversation, and every device that starts from nothing is sent to
// a group with nobody in it, to wait for an invitation that cannot come. Neither
// a message nor an invitation is ever compared with this answer, so no amount of
// ordinary use undoes it (#139).
//
// So a caller may also say holds_everybody: that it is itself inside this
// conversation and has just found a leaf there for every device of every member
// of the chat. That replaces what is settled, and nothing else does. It is a
// fact rather than a claim - the device made the comparison to learn it - and it
// hands nobody anything new, since a member can already take the chat into a
// conversation of their own by inviting everybody to it.
//
// mls.claimConversation peer_id:long group_id:bytes holds_everybody:Bool = mls.Conversation;
func (c *MlsCore) MlsClaimConversation(in *mtproto.TLMlsClaimConversation) (*mtproto.Mls_Conversation, error) {
	peerId := in.GetPeerId()
	claimed := in.GetGroupId()
	if peerId == 0 || len(claimed) == 0 {
		c.Logger.Errorf("mls.claimConversation - a claim needs a chat and a conversation")
		return nil, mtproto.ErrInternalServerError
	}

	var (
		held []byte
		err  error
		now  = int32(time.Now().Unix())
	)
	if in.GetHoldsEverybody() {
		held, err = c.svcCtx.Conversations.Settle(c.ctx, peerId, claimed, now)
	} else {
		held, err = c.svcCtx.Conversations.Claim(c.ctx, peerId, claimed, now)
	}
	if err != nil {
		c.Logger.Errorf("mls.claimConversation - %v", err)
		return nil, mtproto.ErrInternalServerError
	}

	// Said whichever way it goes, and said differently for the two, because the
	// question afterwards is always which of them happened.
	if in.GetHoldsEverybody() {
		c.Logger.Infof("mls.claimConversation - %d is settled on %x, which holds everybody",
			peerId, held)
	} else {
		c.Logger.Infof("mls.claimConversation - %d is held by %x (the claim was %x)",
			peerId, held, claimed)
	}
	return &mtproto.Mls_Conversation{PeerId: peerId, GroupId: held}, nil
}
