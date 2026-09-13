package core

import (
	"errors"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

// What the registry and the state machine refuse reaches the phone as the
// error code it knows, all of them 400. Seen live: "no such call" went out as
// a 500, which a phone reads as "try again" - ten acceptCalls in a row and a
// screen stuck at "Exchanging encryption keys". A 400 ends the call cleanly.
func TestRefusalsReachThePhoneAsCallErrorsItKnows(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   error
		want error
	}{
		{"no such call", calls.ErrNoCall, mtproto.ErrCallPeerInvalid},
		{"a stranger", calls.ErrWrongParty, mtproto.ErrCallPeerInvalid},
		{"a call to oneself", calls.ErrSelfCall, mtproto.ErrCallPeerInvalid},
		{"somebody is busy", calls.ErrBusy, mtproto.ErrCallOccupyFailed},
		{"the wrong moment", calls.ErrWrongState, mtproto.ErrCallAlreadyAccepted},
	} {
		if got := rpcError(tc.in, mtproto.ErrCallAlreadyAccepted); !errors.Is(got, tc.want) {
			t.Errorf("%s: %v became %v, expected %v", tc.name, tc.in, got, tc.want)
		}
	}
	// The wrong moment means different things at different steps; the
	// handler names which.
	if got := rpcError(calls.ErrWrongState, mtproto.ErrCallAlreadyDeclined); !errors.Is(got, mtproto.ErrCallAlreadyDeclined) {
		t.Errorf("the handler's own reading of a wrong moment was not kept: %v", got)
	}
	// Anything else is the server's own trouble and stays what it is.
	other := errors.New("the relay is down")
	if got := rpcError(other, mtproto.ErrCallAlreadyAccepted); !errors.Is(got, other) {
		t.Errorf("a server error was rewritten as a client one: %v", got)
	}
}

// And through a handler: a call that is gone answers CALL_PEER_INVALID.
func TestAcceptingAGoneCallIsPeerInvalid(t *testing.T) {
	s := newStand(t)
	peer := s.peer()
	peer.AccessHash++
	_, err := s.as(bob).PhoneAcceptCall(&mtproto.TLPhoneAcceptCall{Peer: peer, GB: []byte("g_b"), Protocol: protocol()})
	if !errors.Is(err, mtproto.ErrCallPeerInvalid) {
		t.Fatalf("answered %v", err)
	}
}
