package core

import (
	"github.com/teamgram/proto/mtproto"
)

// MessagesGetTopReactions
// messages.getTopReactions
//
// iOS builds the menu of a chat between two from this list alone, so it is the
// whole set, in the owner's order.
func (c *ReactionsCore) MessagesGetTopReactions(in *mtproto.TLMessagesGetTopReactions) (*mtproto.Messages_Reactions, error) {
	return c.theSet(in.Hash), nil
}

// MessagesGetRecentReactions
// messages.getRecentReactions
//
// Nobody's recent reactions are kept; the set stands in for them.
func (c *ReactionsCore) MessagesGetRecentReactions(in *mtproto.TLMessagesGetRecentReactions) (*mtproto.Messages_Reactions, error) {
	return c.theSet(in.Hash), nil
}

// Whole, whatever the limit: the clients ask for 32, 50 or 100, and nine fits.
func (c *ReactionsCore) theSet(hash int64) *mtproto.Messages_Reactions {
	catalog := c.svcCtx.Catalog
	if catalog == nil {
		return mtproto.MakeTLMessagesReactions(&mtproto.Messages_Reactions{
			Hash:      0,
			Reactions: []*mtproto.Reaction{},
		}).To_Messages_Reactions()
	}
	if hash != 0 && hash == int64(catalog.Hash()) {
		return mtproto.MakeTLMessagesReactionsNotModified(nil).To_Messages_Reactions()
	}
	return mtproto.MakeTLMessagesReactions(&mtproto.Messages_Reactions{
		Hash:      int64(catalog.Hash()),
		Reactions: catalog.Reactions(),
	}).To_Messages_Reactions()
}
