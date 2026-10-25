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
// every member's client - builds from before this too - takes it out and lets
// the device in afresh, and the new leaf joins later than the mark, which then
// lets go by itself.
//
// Not while an invitation to this chat waits for the device: a phone that has
// not read its welcome yet claims as well, and the welcome is its way in.

// JustJoinedFor is how long after a device confirms a welcome its claims are
// not taken as a loss.
//
// A claim the device began before it joined arrives after: on 25 October the
// owner's iPhone asked for Edik's key packages at 13:15:25.738, confirmed the
// welcome into the chat's conversation at .881, and its claim reached the
// server at .965. The claim was read as "lost", and Edik's phone took the
// freshly joined leaf out and let it in a second time. Two minutes covers any
// claim in flight; a device that really loses a conversation that soon claims
// again on its next send.
const JustJoinedFor = 120

// JustJoinedKey is where a device's last confirmed welcome is kept, in the KV
// store, for JustJoinedFor seconds.
func JustJoinedKey(userId, authKeyId int64) string {
	return fmt.Sprintf("mls_just_joined#%d_%d", userId, authKeyId)
}

// LostByClaim writes down that this device lost groupId, when the roster holds
// a leaf of it there and no invitation to this chat waits for it. peerId is the
// chat as the claiming device names it, which is how its welcomes name it too.
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
	if !claimShowsLoss(held, behind, deviceKey{user: userId, key: authKeyId}, waiting, peerId, now) {
		return false, nil
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
