package core

import (
	"bytes"
	"errors"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

func signalling(peer *mtproto.InputPhoneCall, data []byte) *mtproto.TLPhoneSendSignalingData {
	return &mtproto.TLPhoneSendSignalingData{Peer: peer, Data: data}
}

// The ICE-candidate channel: a blob from one leg reaches the other leg, whole,
// under the call it belongs to, and nowhere else.
func TestSignalingDataReachesTheOtherLeg(t *testing.T) {
	candidate := []byte("candidate:1 1 udp 2130706431 192.0.2.1 54321 typ host")

	for _, tc := range []struct {
		name     string
		from, to int64
	}{
		{"from the caller to the callee", alice, bob},
		{"from the callee to the caller", bob, alice},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStand(t)

			ok, err := s.as(tc.from).PhoneSendSignalingData(signalling(s.peer(), candidate))
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !mtproto.FromBool(ok) {
				t.Fatal("answered false to a forwarded blob")
			}

			if len(s.sync.pushes) != 1 {
				t.Fatalf("%d pushes went out, expected exactly one", len(s.sync.pushes))
			}
			push := s.sync.pushes[0]
			if push.GetUserId() != tc.to {
				t.Fatalf("the blob went to %d, expected %d", push.GetUserId(), tc.to)
			}
			update := onlyUpdate(t, push)
			if update.GetPredicateName() != mtproto.Predicate_updatePhoneCallSignalingData {
				t.Fatalf("the other leg got %s", update.GetPredicateName())
			}
			if update.GetPhoneCallId() != s.call.Id {
				t.Errorf("under call %d, expected %d", update.GetPhoneCallId(), s.call.Id)
			}
			if !bytes.Equal(update.GetData_FLAGBYTES(), candidate) {
				t.Errorf("the candidate arrived as %q", update.GetData_FLAGBYTES())
			}
		})
	}
}

// Nobody outside the call may inject a candidate into it, and a wrong access
// hash is the same as no call at all.
func TestSignalingDataFromOutsideTheCallGoesNowhere(t *testing.T) {
	t.Run("a stranger", func(t *testing.T) {
		s := newStand(t)

		_, err := s.as(carol).PhoneSendSignalingData(signalling(s.peer(), []byte("x")))
		if !errors.Is(err, calls.ErrWrongParty) {
			t.Fatalf("a stranger was answered with %v", err)
		}
		if len(s.sync.pushes) != 0 {
			t.Fatalf("%d pushes went out on a stranger's blob", len(s.sync.pushes))
		}
	})

	t.Run("a wrong access hash", func(t *testing.T) {
		s := newStand(t)
		peer := s.peer()
		peer.AccessHash++

		_, err := s.as(alice).PhoneSendSignalingData(signalling(peer, []byte("x")))
		if !errors.Is(err, calls.ErrNoCall) {
			t.Fatalf("a wrong hash was answered with %v", err)
		}
		if len(s.sync.pushes) != 0 {
			t.Fatalf("%d pushes went out on a wrong hash", len(s.sync.pushes))
		}
	})

	t.Run("a call that was hung up", func(t *testing.T) {
		s := newStand(t)
		if _, err := s.as(bob).PhoneDiscardCall(&mtproto.TLPhoneDiscardCall{Peer: s.peer()}); err != nil {
			t.Fatalf("cannot hang up: %v", err)
		}
		s.sync.pushes = nil

		_, err := s.as(alice).PhoneSendSignalingData(signalling(s.peer(), []byte("x")))
		if !errors.Is(err, calls.ErrNoCall) {
			t.Fatalf("a dead call was answered with %v", err)
		}
		if len(s.sync.pushes) != 0 {
			t.Fatalf("%d pushes went out on a dead call", len(s.sync.pushes))
		}
	})
}
