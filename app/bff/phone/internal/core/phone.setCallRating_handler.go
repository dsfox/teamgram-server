package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
)

// PhoneSetCallRating takes the stars a person gave a call that is over.
//
// phone.setCallRating flags:# user_initiative:flags.0?true peer:InputPhoneCall
//
//	rating:int comment:string = Updates;
//
// The call is gone from the registry by now, so there is no party to check
// and nothing a rating could affect; it goes to the log, which is where the
// health check reads. It is answered rather than left unimplemented because
// iOS sends it through retryRequest and would repeat an error every five
// seconds for as long as the app lives.
func (c *PhoneCore) PhoneSetCallRating(in *mtproto.TLPhoneSetCallRating) (*mtproto.Updates, error) {
	c.Logger.Infof("phone.setCallRating - %d rated call %d: %d stars, %q",
		c.MD.UserId, in.GetPeer().GetId(), in.GetRating(), in.GetComment())

	return mtproto.MakeTLUpdates(&mtproto.Updates{
		Updates: []*mtproto.Update{},
		Users:   []*mtproto.User{},
		Chats:   []*mtproto.Chat{},
		Date:    int32(time.Now().Unix()),
		Seq:     0,
	}).To_Updates(), nil
}
