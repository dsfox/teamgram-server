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
// mls.claimConversation peer_id:long group_id:bytes = mls.Conversation;
func (c *MlsCore) MlsClaimConversation(in *mtproto.TLMlsClaimConversation) (*mtproto.Mls_Conversation, error) {
	peerId := in.GetPeerId()
	claimed := in.GetGroupId()
	if peerId == 0 || len(claimed) == 0 {
		c.Logger.Errorf("mls.claimConversation - a claim needs a chat and a conversation")
		return nil, mtproto.ErrInternalServerError
	}

	held, err := c.svcCtx.Conversations.Claim(c.ctx, peerId, claimed, int32(time.Now().Unix()))
	if err != nil {
		c.Logger.Errorf("mls.claimConversation - %v", err)
		return nil, mtproto.ErrInternalServerError
	}

	c.Logger.Infof("mls.claimConversation - %d is held by %x (the claim was %x)",
		peerId, held, claimed)
	return &mtproto.Mls_Conversation{PeerId: peerId, GroupId: held}, nil
}
