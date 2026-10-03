package core

import (
	"context"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/rpc/metadata"
	"github.com/teamgram/teamgram-server/app/bff/reactions/internal/svc"
	"github.com/teamgram/teamgram-server/pkg/reactions"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReactionsCore struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	MD *metadata.RpcMetadata
}

func New(ctx context.Context, svcCtx *svc.ServiceContext) *ReactionsCore {
	return &ReactionsCore{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
		MD:     metadata.RpcMetadataFromIncoming(ctx),
	}
}

// The conversation a box belongs to, as the box's owner names it: the other
// person of a pair, or the group.
func peerOf(box *mtproto.MessageBox) *mtproto.Peer {
	return mtproto.GetPeerByDialogId(box.UserId, mtproto.DialogID{A: box.DialogId1, B: box.DialogId2})
}

// The reactions on one copy of a message, as the copy's owner sees them and by
// the number the message has in their box.
func reactionsUpdate(box *mtproto.MessageBox, rows []reactions.MessageReactionsDO) *mtproto.Update {
	return mtproto.MakeTLUpdateMessageReactions(&mtproto.Update{
		Peer_PEER:                  peerOf(box),
		MsgId_INT32:                box.MessageId,
		Reactions_MESSAGEREACTIONS: reactions.View(rows, box.UserId),
	}).To_Update()
}

// Whether the peer a client named is the conversation the box is in. A
// message is found by the number it has in the caller's own box, so the peer
// is only a check that the client means the message it thinks it means.
func samePeer(asked *mtproto.PeerUtil, box *mtproto.MessageBox) bool {
	peer := peerOf(box)
	switch asked.PeerType {
	case mtproto.PEER_SELF, mtproto.PEER_USER:
		return peer.GetPredicateName() == mtproto.Predicate_peerUser && peer.GetUserId() == asked.PeerId
	case mtproto.PEER_CHAT:
		return peer.GetPredicateName() == mtproto.Predicate_peerChat && peer.GetChatId() == asked.PeerId
	}
	return false
}
