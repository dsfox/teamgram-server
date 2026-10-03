package reactions

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

const (
	alice = int64(101)
	bob   = int64(102)
	carol = int64(103)
)

func TestNobodyHasReactedSoTheMessageCarriesNothing(t *testing.T) {
	if got := OnMessage(nil, alice); got != nil {
		t.Fatalf("a message nobody reacted to carries %v", got)
	}
	empty := View(nil, alice)
	if empty == nil || len(empty.Results) != 0 {
		t.Fatalf("a reaction taken back must reach the client as an empty list, got %v", empty)
	}
}

func TestEachPersonSeesTheirOwnReactionAsChosen(t *testing.T) {
	rows := []MessageReactionsDO{
		{UserId: alice, Reaction: "👍", ReactedAt: 10},
		{UserId: bob, Reaction: "❤️", ReactedAt: 20},
		{UserId: carol, Reaction: "👍", ReactedAt: 30},
	}

	forBob := View(rows, bob)
	if len(forBob.Results) != 2 {
		t.Fatalf("two kinds of reaction, got %d", len(forBob.Results))
	}
	first, second := forBob.Results[0], forBob.Results[1]
	if first.Reaction_REACTION.GetEmoticon() != "👍" || first.Count != 2 {
		t.Fatalf("the most given comes first: %v", first)
	}
	if second.Reaction_REACTION.GetEmoticon() != "❤️" || second.Count != 1 {
		t.Fatalf("then the rest: %v", second)
	}
	if first.ChosenOrder != nil || second.ChosenOrder == nil || second.ChosenOrder.Value != 0 {
		t.Fatalf("Bob gave the heart, and only the heart is his: %v / %v", first.ChosenOrder, second.ChosenOrder)
	}
	if first.Reaction_STRING != "👍" {
		t.Fatalf("clients on the older layer read the reaction as a string: %q", first.Reaction_STRING)
	}

	forDave := View(rows, 104)
	for _, count := range forDave.Results {
		if count.ChosenOrder != nil || count.Chosen {
			t.Fatalf("somebody who gave nothing has nothing chosen: %v", count)
		}
	}
}

func TestRecentReactionsAreNewestFirstAndMarkTheViewer(t *testing.T) {
	rows := []MessageReactionsDO{
		{UserId: alice, Reaction: "👍", ReactedAt: 10},
		{UserId: bob, Reaction: "🔥", ReactedAt: 30},
	}
	recent := View(rows, alice).RecentReactions
	if len(recent) != 2 || recent[0].PeerId.GetUserId() != bob || recent[1].PeerId.GetUserId() != alice {
		t.Fatalf("newest first: %v", recent)
	}
	if recent[0].My || !recent[1].My {
		t.Fatalf("only the viewer's own is marked as theirs: %v", recent)
	}
	if recent[0].Reaction_REACTION.GetEmoticon() != "🔥" || recent[0].Date != 30 {
		t.Fatalf("who, with what and when: %v", recent[0])
	}
}

func TestTiesAreOrderedTheSameWayEveryTime(t *testing.T) {
	rows := []MessageReactionsDO{
		{UserId: alice, Reaction: "😢", ReactedAt: 20},
		{UserId: bob, Reaction: "🎉", ReactedAt: 10},
	}
	for range 20 {
		results := View(rows, carol).Results
		if results[0].Reaction_REACTION.GetEmoticon() != "🎉" {
			t.Fatalf("one each: the one given first leads, every time: %v", results)
		}
	}
}

func TestALargeGroupNamesOnlyTheLatestFew(t *testing.T) {
	var rows []MessageReactionsDO
	for i := range 50 {
		rows = append(rows, MessageReactionsDO{UserId: int64(1000 + i), Reaction: "👍", ReactedAt: int32(i)})
	}
	view := View(rows, alice)
	if view.Results[0].Count != 50 {
		t.Fatalf("every reaction is counted: %d", view.Results[0].Count)
	}
	if len(view.RecentReactions) != recentLimit || view.RecentReactions[0].PeerId.GetUserId() != 1049 {
		t.Fatalf("only the latest %d are named, newest first: %d", recentLimit, len(view.RecentReactions))
	}
}

func TestAStoredMessageKeepsNoReactionsAndTheToldOneKeepsThem(t *testing.T) {
	message := &mtproto.Message{Id: 7, Message: "mls1:...", Reactions: View([]MessageReactionsDO{{UserId: alice, Reaction: "👍"}}, alice)}
	stored := Stored(message)
	if stored.Reactions != nil || stored.Message != "mls1:..." || stored.Id != 7 {
		t.Fatalf("message_data keeps the message and not the reactions: %v", stored)
	}
	if message.Reactions == nil {
		t.Fatal("the copy the editor's other phones are told of keeps what they see")
	}
	plain := &mtproto.Message{Id: 8}
	if Stored(plain) != plain {
		t.Fatal("a message without reactions is written as it is")
	}
}
