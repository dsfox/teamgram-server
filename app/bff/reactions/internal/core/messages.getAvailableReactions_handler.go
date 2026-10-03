package core

import (
	"github.com/teamgram/proto/mtproto"
)

// MessagesGetAvailableReactions
// messages.getAvailableReactions
//
// "Not modified" only for the hash this list has. A client asking with zero
// holds nothing, and telling it nothing changed is how one phone asked 431
// times in a row (fake_rpc_result.go).
func (c *ReactionsCore) MessagesGetAvailableReactions(in *mtproto.TLMessagesGetAvailableReactions) (*mtproto.Messages_AvailableReactions, error) {
	catalog := c.svcCtx.Catalog
	if catalog == nil {
		return mtproto.MakeTLMessagesAvailableReactions(&mtproto.Messages_AvailableReactions{
			Hash:      0,
			Reactions: []*mtproto.AvailableReaction{},
		}).To_Messages_AvailableReactions(), nil
	}
	if in.Hash != 0 && in.Hash == catalog.Hash() {
		return mtproto.MakeTLMessagesAvailableReactionsNotModified(nil).To_Messages_AvailableReactions(), nil
	}
	return mtproto.MakeTLMessagesAvailableReactions(&mtproto.Messages_AvailableReactions{
		Hash:      catalog.Hash(),
		Reactions: catalog.Available(),
	}).To_Messages_AvailableReactions(), nil
}
