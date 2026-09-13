package service

import (
	"context"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

// DhService answers the one secret-chats method a call needs: the group the
// two phones derive the call key in (#14). A phone asks for it before it
// places or answers a call and waits on the answer - with none, the call
// stands at "Requesting" for ever. Everything else under RPCSecretChats stays
// unimplemented: secret chats are not offered in this fork, end-to-end
// encryption is MLS.
type DhService struct {
	mtproto.UnimplementedRPCSecretChatsServer
}

func NewDhService() *DhService {
	return &DhService{}
}

// randomAtMost caps what a phone may ask for: the clients ask for 256 bytes,
// and a larger number is not a phone.
const randomAtMost = 256

// MessagesGetDhConfig names the group, with fresh randomness (#14).
//
// messages.getDhConfig version:int random_length:int = messages.DhConfig;
func (s *DhService) MessagesGetDhConfig(ctx context.Context, request *mtproto.TLMessagesGetDhConfig) (*mtproto.Messages_DhConfig, error) {
	if request.GetRandomLength() < 0 || request.GetRandomLength() > randomAtMost {
		return nil, mtproto.ErrRandomLengthInvalid
	}
	return calls.DhConfig(request.GetVersion(), request.GetRandomLength()), nil
}
