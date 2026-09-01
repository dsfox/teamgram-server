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
// A caller used to be able to say holds_everybody as well: that it was itself
// inside this conversation and had just found a leaf there for every device of
// every member of the chat. It existed because nobody else could answer that
// question. The roster answers it now, and a conversation nobody is left in
// stops holding a chat at once rather than after a fortnight, so the vouch is
// gone (#147).
//
// mls.claimConversation peer_id:long group_id:bytes holds:Vector<bytes> = mls.Conversation;
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
	held, err = c.svcCtx.Conversations.Claim(c.ctx, peerId, claimed, now)
	if err != nil {
		c.Logger.Errorf("mls.claimConversation - %v", err)
		return nil, mtproto.ErrInternalServerError
	}

	// And who the caller says this conversation holds. A new group's membership
	// arrives here and nowhere else: its creator accepts its own commit locally
	// and never posts it, there being nobody to have raced with, so no commit
	// will ever carry it (#147).
	//
	// Recorded whoever won the claim, because the roster is about the group the
	// caller named rather than about the chat - a conversation that lost the
	// claim still holds the people it holds. An empty list says nothing and is
	// ignored; a failure here does not fail the claim, which is the thing the
	// caller is waiting for.
	if len(in.GetHolds()) > 0 {
		// Epoch zero: a claim carries no epoch, and a new group is at its
		// first. Any commit afterwards names a higher one and takes over.
		if err := c.svcCtx.Members.Record(c.ctx, claimed, 0, in.GetHolds(), now); err != nil {
			c.Logger.Errorf("mls.claimConversation - the roster of %x was not written: %v", claimed, err)
		} else {
			c.Logger.Infof("mls.claimConversation - %x holds %d leaf/leaves",
				claimed, len(in.GetHolds()))
		}
	}

	// Said whichever way it goes, and the claim is named beside the answer,
	// because the question afterwards is always whether this caller won.
	c.Logger.Infof("mls.claimConversation - %d is held by %x (the claim was %x)",
		peerId, held, claimed)
	return &mtproto.Mls_Conversation{PeerId: peerId, GroupId: held}, nil
}
