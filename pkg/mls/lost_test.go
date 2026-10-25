package mls

import "testing"

// The owner's iPhone and Edik's phone on 24 October (#291): the roster holds
// both, the iPhone claims another conversation for the chat.
func TestAClaimFromInsideTheRosterIsALoss(t *testing.T) {
	iphone := deviceKey{user: 1, key: 11}
	edik := deviceKey{user: 2, key: 21}
	held := []memberRow{leaf("1/a", 1), leaf("2/a", 2)}
	behind := map[string]deviceKey{"1/a": iphone, "2/a": edik}

	if !claimShowsLoss(held, behind, iphone, nil, 2, at) {
		t.Fatal("a device the roster holds claimed another conversation and nothing was written down")
	}
	if claimShowsLoss(held, behind, deviceKey{user: 1, key: 12}, nil, 2, at) {
		t.Fatal("a device outside the conversation is waiting to be let in, not lost")
	}
	if claimShowsLoss(held, map[string]deviceKey{"2/a": edik}, iphone, nil, 2, at) {
		t.Fatal("a leaf nobody can match was taken for the claiming device")
	}
}

func TestAWaitingInvitationIsTheWayInNotALoss(t *testing.T) {
	iphone := deviceKey{user: 1, key: 11}
	held := []memberRow{leaf("1/a", 1)}
	behind := map[string]deviceKey{"1/a": iphone}

	if claimShowsLoss(held, behind, iphone, []Welcome{{PeerId: 2}}, 2, at) {
		t.Fatal("an invitation to this chat waits and the leaf was called lost")
	}
	if claimShowsLoss(held, behind, iphone, []Welcome{{PeerId: 0}}, 2, at) {
		t.Fatal("an invitation that names no chat may be for this one")
	}
	if !claimShowsLoss(held, behind, iphone, []Welcome{{PeerId: 3}}, 2, at) {
		t.Fatal("an invitation to another chat kept a lost leaf alive")
	}
}

func TestALeafJoinedThisSecondIsNotLost(t *testing.T) {
	iphone := deviceKey{user: 1, key: 11}
	held := []memberRow{{Leaf: []byte("1/a"), UserId: 1, JoinedAt: at}}
	if claimShowsLoss(held, map[string]deviceKey{"1/a": iphone}, iphone, nil, 2, at) {
		t.Fatal("a leaf let in this very second was called lost")
	}
}
