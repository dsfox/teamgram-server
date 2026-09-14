package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
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

	// Direct unless either of the two hides their IP (the P2P privacy
	// setting, off by default). The flag travels with the connections,
	// because both engines read it before gathering a single candidate -
	// without it they wait for a relay, and with none configured the call
	// fails.
	p2pAllowed := c.directAllowed(call.Admin, call.Participant)
	connections, err := c.connectionsFor(p2pAllowed, now)
	if err != nil {
		c.Logger.Errorf("phone.confirmCall - nothing to offer for %d: %v", call.Id, err)
		return nil, err
	}

	active := mtproto.MakeTLPhoneCall(&mtproto.PhoneCall{
		P2PAllowed:     p2pAllowed,
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
		// What the caller asked for, on every object the caller is shown:
		// Android reads the flag back from here and ran a video call as
		// voice while it was missing.
		Video: call.Video,
	}).To_PhoneCall()

	c.tell(call.Participant, call.ParticipantKey, call.ParticipantServer, c.updatesFor(active, now))

	return mtproto.MakeTLPhonePhoneCall(&mtproto.Phone_PhoneCall{
		PhoneCall: active,
		Users:     []*mtproto.User{},
	}).To_Phone_PhoneCall(), nil
}

// directAllowed is whether the two may learn each other's address: my IP is
// hidden from you when I say so, and yours from me when you do, so both are
// asked. An answer that cannot be had counts as hidden - the relay is the
// safe side of that mistake, a leaked address is not.
func (c *PhoneCore) directAllowed(admin, participant int64) bool {
	return c.allowsDirect(admin, participant) && c.allowsDirect(participant, admin)
}

func (c *PhoneCore) allowsDirect(owner, peer int64) bool {
	allowed, err := c.svcCtx.Users.UserCheckPrivacy(c.ctx, &userpb.TLUserCheckPrivacy{
		UserId:  owner,
		KeyType: mtproto.PHONE_P2P,
		PeerId:  peer,
	})
	if err != nil {
		c.Logger.Errorf("phone: cannot ask whether %d lets %d connect directly, so no: %v", owner, peer, err)
		return false
	}
	return mtproto.FromBool(allowed)
}
