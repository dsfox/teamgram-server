package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
)

// PhoneConfirmCall closes the key exchange and hands both phones what to try.
//
// phone.confirmCall peer:InputPhoneCall g_a:bytes key_fingerprint:long
//
//	protocol:PhoneCallProtocol = phone.PhoneCall;
//
// This is where the connections come from: STUN first so the two devices find
// each other, relay only as the fallback. The key itself is theirs - we carried
// g_a_hash, g_b and now g_a, and never opened any of them.
func (c *PhoneCore) PhoneConfirmCall(in *mtproto.TLPhoneConfirmCall) (*mtproto.Phone_PhoneCall, error) {
	now := time.Now()
	call, err := c.find(in.GetPeer(), now)
	if err != nil {
		return nil, rpcError(err, mtproto.ErrCallPeerInvalid)
	}
	if err = call.Confirm(c.MD.UserId, in.GetGA(), in.GetKeyFingerprint(), now); err != nil {
		c.Logger.Errorf("phone.confirmCall - %d cannot confirm %d: %v", c.MD.UserId, call.Id, err)
		return nil, rpcError(err, mtproto.ErrCallPeerInvalid)
	}

	connections, err := c.connectionsFor(true, now)
	if err != nil {
		c.Logger.Errorf("phone.confirmCall - nothing to offer for %d: %v", call.Id, err)
		return nil, err
	}

	active := mtproto.MakeTLPhoneCall(&mtproto.PhoneCall{
		Id:             call.Id,
		AccessHash:     call.AccessHash,
		Date:           int32(now.Unix()),
		AdminId:        call.Admin,
		ParticipantId:  call.Participant,
		GAOrB:          call.GA,
		KeyFingerprint: call.KeyFingerprint,
		Protocol:       in.GetProtocol(),
		Connections:    connections,
		StartDate:      int32(now.Unix()),
	}).To_PhoneCall()

	c.tell(call.Participant, call.ParticipantKey, call.ParticipantServer, c.updatesFor(active, now))

	return mtproto.MakeTLPhonePhoneCall(&mtproto.Phone_PhoneCall{
		PhoneCall: active,
		Users:     []*mtproto.User{},
	}).To_Phone_PhoneCall(), nil
}
