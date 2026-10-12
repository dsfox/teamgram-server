package core

import (
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/service/biz/dialog/dialog"
)

// keepsNoDraft says a draft stays on the phone (#264).
//
// The checklist's rule is that a draft stays on the device: syncing it would
// put the words on the server before anybody decided to send them. iOS kept
// that for encrypted chats only, Android for none, and messages.saveDraft
// stored whatever came, in dialogs.draft_message_data, and handed it to the
// account's other devices. Installed builds go on sending drafts, so this is
// where the rule holds for all of them: the request is answered, nothing is
// kept, and nobody is told - an empty draft sent to the other devices would
// erase the drafts they keep themselves.
const keepsNoDraft = true

// forgetDraft answers a saveDraft and keeps nothing. A draft this chat had
// from before the rule is cleared on the way, so the leftovers go by
// themselves.
func (c *DraftsCore) forgetDraft(peer *mtproto.PeerUtil) (*mtproto.Bool, error) {
	if _, err := c.svcCtx.Dao.DialogClient.DialogClearDraftMessage(c.ctx, &dialog.TLDialogClearDraftMessage{
		UserId:   c.MD.UserId,
		PeerType: peer.PeerType,
		PeerId:   peer.PeerId,
	}); err != nil {
		// Said and not acted on: the draft the client sent is kept nowhere
		// either way, and refusing the request would only make it ask again.
		c.Logger.Errorf("messages.saveDraft - cannot clear the draft kept for %d/%d: %v",
			peer.PeerType, peer.PeerId, err)
	}
	return mtproto.BoolTrue, nil
}
