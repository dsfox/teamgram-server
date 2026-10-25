package mls

import (
	"context"
	"fmt"
)

// A device the roster puts inside a chat's conversation, asking for another
// conversation for that chat, has lost the one it is in (#291).
//
// A device inside a conversation encrypts with it and never claims; both
// clients claim only when they have no conversation for the chat. So the claim
// is the device's own word that the roster is wrong about it - and nothing else
// could ever say so. Its leaf answers and missed no commit, so it was called
// alive, its owner was not short of anybody, and no member had a reason to let
// it in again. On 24 October the owner's iPhone waited like that with a message
// to Edik for good, while the roster said it was in.
//
// Written down the way a missed commit is (#211): the leaf is dead from now,
// every member's client takes it out and lets the device in afresh, and the new
// leaf joins later than the mark, which then lets go by itself.
//
// Not while an invitation to this chat waits for the device: a phone that has
// not read its welcome yet claims as well, and the welcome is its way in.

// LostWait is how long a lost device waits to be let in before a chat between
// two is handed to the conversation it asks for.
//
// A member lets it in at its next comparison - builds since #247 compare a
// chat between two on every key package round, a few minutes - and an old
// build compares only when its owner writes into that chat. Edik's phone, a
// build of 9 October, came online on 25 October and left without writing, and
// the owner's message went on waiting. Half an hour is well past any round of
// a current build, so a hand-over after it races nobody's invitation.
const LostWait = int32(30 * 60)

// LostByClaim writes down that this device lost groupId, when the roster holds
// a leaf of it there and no invitation to this chat waits for it. peerId is the
// chat as the claiming device names it, which is how its welcomes name it too.
//
// A mark that already outdates the leaf is kept as it is, so it says when the
// loss was first seen: LostWait counts from there.
func (m *MysqlMembers) LostByClaim(ctx context.Context, welcomes WelcomeStore, commits CommitStore,
	groupId []byte, userId, authKeyId, peerId int64, now int32) (bool, error) {
	held, err := m.leavesOf(ctx, groupId)
	if err != nil || len(held) == 0 {
		return false, err
	}
	names := make([][]byte, 0, len(held))
	for _, row := range held {
		names = append(names, row.Leaf)
	}
	behind, err := m.devicesBehind(ctx, names)
	if err != nil {
		return false, err
	}
	waiting, err := welcomes.Waiting(ctx, userId, authKeyId)
	if err != nil {
		return false, fmt.Errorf("cannot read what waits for the device: %w", err)
	}
	device := deviceKey{user: userId, key: authKeyId}
	if !claimShowsLoss(held, behind, device, waiting, peerId, now) {
		return false, nil
	}
	missed, err := m.missedIn(ctx, groupId)
	if err != nil {
		return false, err
	}
	if alreadyMarked(held, behind, device, missed) {
		return true, nil
	}
	if err := commits.Missed(ctx, []Missed{{GroupId: groupId, UserId: userId, AuthKeyId: authKeyId, At: now}}); err != nil {
		return false, err
	}
	return true, nil
}

// claimShowsLoss is the decision, apart from the reads.
func claimShowsLoss(held []memberRow, behind map[string]deviceKey, device deviceKey,
	waiting []Welcome, peerId int64, now int32) bool {
	inside := false
	for _, row := range held {
		// Joined this very second is a leaf the mark could not outdate anyway.
		if known, ok := behind[string(row.Leaf)]; ok && known == device && row.JoinedAt < now {
			inside = true
			break
		}
	}
	if !inside {
		return false
	}
	for _, w := range waiting {
		// Zero is a welcome from before welcomes named their chat: it may be
		// for this one.
		if w.PeerId == peerId || w.PeerId == 0 {
			return false
		}
	}
	return true
}

// Whether the device's mark already outdates its leaf here.
func alreadyMarked(held []memberRow, behind map[string]deviceKey, device deviceKey, missed map[deviceKey]int32) bool {
	mark, ok := missed[device]
	if !ok {
		return false
	}
	for _, row := range held {
		if known, matched := behind[string(row.Leaf)]; matched && known == device && mark > row.JoinedAt {
			return true
		}
	}
	return false
}

// LostLongEnough says whether a chat between two may be handed from groupId to
// the conversation this lost device asks for: its loss was seen LostWait ago,
// nobody has touched the roster since, and its owner has no live leaf left
// there - nobody of theirs is in it, and nobody let them back in. The other
// person is then invited into the new conversation by the device that made it,
// and every build joins a welcome by itself.
func (m *MysqlMembers) LostLongEnough(ctx context.Context, groupId []byte, userId, authKeyId int64, now int32) (bool, error) {
	held, err := m.leavesOf(ctx, groupId)
	if err != nil || len(held) == 0 {
		return false, err
	}
	names := make([][]byte, 0, len(held))
	people := map[int64]bool{}
	for _, row := range held {
		names = append(names, row.Leaf)
		people[row.UserId] = true
	}
	behind, err := m.devicesBehind(ctx, names)
	if err != nil {
		return false, err
	}
	answering, err := m.answering(ctx, people, now-DeviceLife)
	if err != nil {
		return false, err
	}
	missed, err := m.missedIn(ctx, groupId)
	if err != nil {
		return false, err
	}
	return lostLongEnough(held, behind, answering, missed, deviceKey{user: userId, key: authKeyId}, now), nil
}

// lostLongEnough is the decision, apart from the reads.
func lostLongEnough(held []memberRow, behind map[string]deviceKey, answering map[int64]map[int64]bool,
	missed map[deviceKey]int32, device deviceKey, now int32) bool {
	mark, ok := missed[device]
	if !ok || mark > now-LostWait {
		return false
	}
	lostHere := false
	for _, row := range held {
		// A roster written since the loss is somebody acting on it.
		if row.SeenAt > mark {
			return false
		}
		known, matched := behind[string(row.Leaf)]
		if matched && known == device && row.JoinedAt < mark {
			lostHere = true
		}
		if row.UserId != device.user {
			continue
		}
		// A leaf nobody can match counts as alive, as everywhere (Holding).
		alive := !matched || (answering[known.user][known.key] && missed[known] <= row.JoinedAt)
		if alive {
			return false
		}
	}
	return lostHere
}
