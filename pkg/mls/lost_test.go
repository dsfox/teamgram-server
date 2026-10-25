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

// The owner's iPhone, marked lost, and Edik's old build that never lets it in.
func TestAChatNobodyLetTheLostDeviceBackIntoIsHandedOver(t *testing.T) {
	iphone := deviceKey{user: 1, key: 11}
	edik := deviceKey{user: 2, key: 21}
	held := []memberRow{leaf("1/a", 1), leaf("2/a", 2)}
	behind := map[string]deviceKey{"1/a": iphone, "2/a": edik}
	answering := map[int64]map[int64]bool{1: {11: true}, 2: {21: true}}
	long := map[deviceKey]int32{iphone: at - LostWait - 1}

	if !lostLongEnough(held, behind, answering, long, iphone, at) {
		t.Fatal("lost half an hour ago, nobody let it in, and the chat stays where nobody of its owner is")
	}
	if lostLongEnough(held, behind, answering, map[deviceKey]int32{iphone: at - 60}, iphone, at) {
		t.Fatal("handed over a minute after the loss, racing a member that is about to let it in")
	}
	if lostLongEnough(held, behind, answering, nil, iphone, at) {
		t.Fatal("handed over with no loss written down")
	}
	touched := []memberRow{leaf("1/a", 1), {Leaf: []byte("2/a"), UserId: 2, JoinedAt: at - 86400, SeenAt: at - 10}}
	if lostLongEnough(touched, behind, answering, long, iphone, at) {
		t.Fatal("somebody wrote the roster after the loss and the chat was taken from them")
	}
	tablet := deviceKey{user: 1, key: 12}
	withTablet := append([]memberRow{leaf("1/b", 1)}, held...)
	behindTablet := map[string]deviceKey{"1/a": iphone, "1/b": tablet, "2/a": edik}
	answeringTablet := map[int64]map[int64]bool{1: {11: true, 12: true}, 2: {21: true}}
	if lostLongEnough(withTablet, behindTablet, answeringTablet, long, iphone, at) {
		t.Fatal("the owner's tablet is still in the conversation and the chat was moved away from it")
	}
}

func TestAFirstLossIsNotMovedByTheNextClaim(t *testing.T) {
	iphone := deviceKey{user: 1, key: 11}
	held := []memberRow{leaf("1/a", 1)}
	behind := map[string]deviceKey{"1/a": iphone}
	if !alreadyMarked(held, behind, iphone, map[deviceKey]int32{iphone: at}) {
		t.Fatal("a mark after the leaf joined was not taken for a loss already written")
	}
	if alreadyMarked(held, behind, iphone, map[deviceKey]int32{iphone: at - 2*86400}) {
		t.Fatal("a mark from before the leaf joined was taken for this loss")
	}
}

func TestALeafJoinedThisSecondIsNotLost(t *testing.T) {
	iphone := deviceKey{user: 1, key: 11}
	held := []memberRow{{Leaf: []byte("1/a"), UserId: 1, JoinedAt: at}}
	if claimShowsLoss(held, map[string]deviceKey{"1/a": iphone}, iphone, nil, 2, at) {
		t.Fatal("a leaf let in this very second was called lost")
	}
}
