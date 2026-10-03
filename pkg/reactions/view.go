package reactions

import (
	"sort"

	"github.com/teamgram/proto/mtproto"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// How many of the people who reacted a message names. The clients draw at most
// three faces under a message, and a history page carries this for every
// message on it, so a group of two hundred is not spelt out two hundred times.
const recentLimit = 10

// View is what one person sees of a message's reactions: how many of each,
// which one is theirs, and who reacted with what, newest first.
//
// The same rows look different to each person - only the viewer's own reaction
// is marked as chosen - which is why this is built for every box a message is
// in rather than stored with the message. Never nil: an empty result is how a
// client learns that the last reaction was taken back.
func View(rows []MessageReactionsDO, viewer int64) *mtproto.MessageReactions {
	type tally struct {
		reaction string
		count    int32
		first    int32
	}
	tallies := map[string]*tally{}
	for _, row := range rows {
		t, ok := tallies[row.Reaction]
		if !ok {
			t = &tally{reaction: row.Reaction, first: row.ReactedAt}
			tallies[row.Reaction] = t
		}
		t.count++
		if row.ReactedAt < t.first {
			t.first = row.ReactedAt
		}
	}
	ordered := make([]*tally, 0, len(tallies))
	for _, t := range tallies {
		ordered = append(ordered, t)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].count != ordered[j].count {
			return ordered[i].count > ordered[j].count
		}
		if ordered[i].first != ordered[j].first {
			return ordered[i].first < ordered[j].first
		}
		return ordered[i].reaction < ordered[j].reaction
	})

	mine := ""
	for _, row := range rows {
		if row.UserId == viewer {
			mine = row.Reaction
		}
	}

	results := make([]*mtproto.ReactionCount, 0, len(ordered))
	for _, t := range ordered {
		count := &mtproto.ReactionCount{
			Reaction_REACTION: mtproto.FromReaction(t.reaction),
			Reaction_STRING:   t.reaction,
			Count:             t.count,
		}
		if t.reaction == mine {
			count.ChosenOrder = &wrapperspb.Int32Value{Value: 0}
			count.Chosen = true
		}
		results = append(results, mtproto.MakeTLReactionCount(count).To_ReactionCount())
	}

	recent := append([]MessageReactionsDO(nil), rows...)
	sort.Slice(recent, func(i, j int) bool {
		if recent[i].ReactedAt != recent[j].ReactedAt {
			return recent[i].ReactedAt > recent[j].ReactedAt
		}
		return recent[i].UserId < recent[j].UserId
	})
	if len(recent) > recentLimit {
		recent = recent[:recentLimit]
	}
	peers := make([]*mtproto.MessagePeerReaction, 0, len(recent))
	for _, row := range recent {
		peers = append(peers, PeerReaction(row, viewer))
	}

	return mtproto.MakeTLMessageReactions(&mtproto.MessageReactions{
		CanSeeList:      true,
		Results:         results,
		RecentReactions: peers,
	}).To_MessageReactions()
}

// PeerReaction is one row as a client reads it: who, with what, and when.
func PeerReaction(row MessageReactionsDO, viewer int64) *mtproto.MessagePeerReaction {
	return mtproto.MakeTLMessagePeerReaction(&mtproto.MessagePeerReaction{
		My:                row.UserId == viewer,
		PeerId:            mtproto.MakePeerUser(row.UserId),
		Date:              row.ReactedAt,
		Reaction_REACTION: mtproto.FromReaction(row.Reaction),
		Reaction_STRING:   row.Reaction,
	}).To_MessagePeerReaction()
}

// OnMessage is View as a message from history carries it: nothing at all when
// nobody has reacted, so a message without reactions looks as it always did.
func OnMessage(rows []MessageReactionsDO, viewer int64) *mtproto.MessageReactions {
	if len(rows) == 0 {
		return nil
	}
	return View(rows, viewer)
}

// Stored is a message as message_data keeps it: without reactions, which live
// in their own table and look different to everybody who reads them. The
// message itself is left as it was, for whoever is about to be told of it.
func Stored(m *mtproto.Message) *mtproto.Message {
	if m.GetReactions() == nil {
		return m
	}
	stored := proto.Clone(m).(*mtproto.Message)
	stored.Reactions = nil
	return stored
}
