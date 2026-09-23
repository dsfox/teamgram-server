package core

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/calls"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// The objects a phone is shown about a call, every one built from the call
// through view(): id, access hash, date, the two people, the caller's
// protocol and whether it is a video call ride on all of them, and a view
// adds only what its step brings. Two of these used to be assembled by hand
// in their handlers and lost the video flag; Android runs a video call as
// voice then. One place, one test (views_test.go).

// view is what every object of the call carries.
func view(call *calls.Call) *mtproto.PhoneCall {
	return &mtproto.PhoneCall{
		Id:            call.Id,
		AccessHash:    call.AccessHash,
		Date:          int32(call.Created.Unix()),
		AdminId:       call.Admin,
		ParticipantId: call.Participant,
		Protocol:      call.Protocol,
		Video:         call.Video,
	}
}

// requested is the call as the callee first sees it. Both phones start
// ringing on this constructor and on no other - a waiting call with no session
// behind it is dropped by either client - and g_a_hash travels in it: the
// callee checks g_a against it at the end of the exchange.
func (c *PhoneCore) requested(call *calls.Call) *mtproto.PhoneCall {
	pc := view(call)
	pc.GAHash = call.GAHash
	return mtproto.MakeTLPhoneCallRequested(pc).To_PhoneCall()
}

// waiting is the call as the caller sees it before anyone picks up: the same
// object at requestCall and again, with receive_date, once the callee's phone
// says it is ringing - and what the callee is answered with at acceptCall,
// still waiting for the caller's confirm.
func (c *PhoneCore) waiting(call *calls.Call, now time.Time) *mtproto.PhoneCall {
	pc := view(call)
	if !call.Received.IsZero() {
		pc.ReceiveDate = mtproto.MakeFlagsInt32(int32(call.Received.Unix()))
	}
	return mtproto.MakeTLPhoneCallWaiting(pc).To_PhoneCall()
}

// accepted is the answer as the caller sees it: g_b, and the callee's
// protocol, which the caller reads to settle the layer the two speak.
func (c *PhoneCore) accepted(call *calls.Call) *mtproto.PhoneCall {
	pc := view(call)
	pc.GB = call.GB
	if call.ParticipantProtocol != nil {
		pc.Protocol = call.ParticipantProtocol
	}
	return mtproto.MakeTLPhoneCallAccepted(pc).To_PhoneCall()
}

// active is the confirmed call: g_a and the fingerprint the two compare, the
// connections to try, whether a direct path is allowed, and the one version
// of the call library both phones will run.
func (c *PhoneCore) active(call *calls.Call, connections []*mtproto.PhoneConnection, p2pAllowed bool, now time.Time) *mtproto.PhoneCall {
	pc := view(call)
	pc.Protocol = agreedProtocol(call.Protocol, call.ParticipantProtocol)
	pc.GAOrB = call.GA
	pc.KeyFingerprint = call.KeyFingerprint
	pc.Connections = connections
	pc.P2PAllowed = p2pAllowed
	pc.StartDate = int32(now.Unix())
	return mtproto.MakeTLPhoneCall(pc).To_PhoneCall()
}

// newestAgreedVersion is the newest call library version the server settles
// on. Both phones run versions[0] of the confirmed call's protocol, whatever
// else they list. Every call measured on real phones so far ran 9.0.0, and
// Android compares the agreed version with "2.7.7" as text, so from "10.0.0"
// up it reads as older and the caller's camera is switched off (#186).
// Raising this is a step to measure on phones, not a default.
const newestAgreedVersion = "9.0.0"

// agreedProtocol is the caller's protocol with one version left in it: the
// newest both phones list, not above newestAgreedVersion when they share one
// that is not. Phones that share nothing get the caller's list as it was.
func agreedProtocol(caller, callee *mtproto.PhoneCallProtocol) *mtproto.PhoneCallProtocol {
	newest, newestAllowed := "", ""
	for _, version := range caller.GetLibraryVersions() {
		if !slices.Contains(callee.GetLibraryVersions(), version) {
			continue
		}
		if newest == "" || newerVersion(version, newest) {
			newest = version
		}
		if !newerVersion(version, newestAgreedVersion) && (newestAllowed == "" || newerVersion(version, newestAllowed)) {
			newestAllowed = version
		}
	}
	if newestAllowed != "" {
		newest = newestAllowed
	}
	if newest == "" {
		return caller
	}
	agreed := proto.Clone(caller).(*mtproto.PhoneCallProtocol)
	agreed.LibraryVersions = []string{newest}
	return agreed
}

// newerVersion compares dotted versions by number, "10.0.0" after "9.0.0".
func newerVersion(a, b string) bool {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(left), len(right)); i++ {
		var x, y int
		if i < len(left) {
			x, _ = strconv.Atoi(left[i])
		}
		if i < len(right) {
			y, _ = strconv.Atoi(right[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// discarded is the call over: the reason, the duration once it was spoken,
// whether it was video, and need_debug - both phones send their stats log,
// the one thing that says whether the media went direct or through the relay.
func (c *PhoneCore) discarded(call *calls.Call, reason *mtproto.PhoneCallDiscardReason, duration *wrapperspb.Int32Value) *mtproto.PhoneCall {
	return mtproto.MakeTLPhoneCallDiscarded(&mtproto.PhoneCall{
		Id:        call.Id,
		Reason:    reason,
		Duration:  duration,
		Video:     call.Video,
		NeedDebug: true,
	}).To_PhoneCall()
}
