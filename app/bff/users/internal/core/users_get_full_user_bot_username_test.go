package core

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// ice9: a bot's username survives user.ToUnsafeUser, which getFullUser hands out as is

func TestUnsafeUserKeepsBotUsername(t *testing.T) {
	me := mtproto.MakeTLImmutableUser(&mtproto.ImmutableUser{
		User: mtproto.MakeTLUserData(&mtproto.UserData{
			Id:        1,
			FirstName: "me",
		}).To_UserData(),
	}).To_ImmutableUser()

	bot := mtproto.MakeTLImmutableUser(&mtproto.ImmutableUser{
		User: mtproto.MakeTLUserData(&mtproto.UserData{
			Id:        6,
			FirstName: "bot",
			Username:  "bot_username",
			Bot: mtproto.MakeTLBotData(&mtproto.BotData{
				BotInfoVersion: 1,
			}).To_BotData(),
		}).To_UserData(),
	}).To_ImmutableUser()

	unsafeBot := bot.ToUnsafeUser(me)

	if unsafeBot.GetUsername().GetValue() != "bot_username" {
		t.Fatalf("expected username %q, got %v", "bot_username", unsafeBot.GetUsername())
	}
	if len(unsafeBot.GetUsernames()) != 0 {
		t.Fatalf("expected no usernames vector for a single username, got %d", len(unsafeBot.GetUsernames()))
	}
}

func TestUnsafeUserKeepsBotUsernamesVector(t *testing.T) {
	me := mtproto.MakeTLImmutableUser(&mtproto.ImmutableUser{
		User: mtproto.MakeTLUserData(&mtproto.UserData{
			Id:        1,
			FirstName: "me",
		}).To_UserData(),
	}).To_ImmutableUser()

	bot := mtproto.MakeTLImmutableUser(&mtproto.ImmutableUser{
		User: mtproto.MakeTLUserData(&mtproto.UserData{
			Id:        6,
			FirstName: "bot",
			Username:  "primary_should_not_be_used",
			Usernames: []*mtproto.Username{
				mtproto.MakeTLUsername(&mtproto.Username{Username: "bot_one", Active: true}).To_Username(),
				mtproto.MakeTLUsername(&mtproto.Username{Username: "bot_two", Active: false}).To_Username(),
			},
			Bot: mtproto.MakeTLBotData(&mtproto.BotData{
				BotInfoVersion: 1,
			}).To_BotData(),
		}).To_UserData(),
	}).To_ImmutableUser()

	unsafeBot := bot.ToUnsafeUser(me)

	if unsafeBot.GetUsername() != nil {
		t.Fatalf("expected username field to stay nil when usernames vector exists, got %q", unsafeBot.GetUsername().GetValue())
	}
	if len(unsafeBot.GetUsernames()) != 2 {
		t.Fatalf("expected 2 usernames, got %d", len(unsafeBot.GetUsernames()))
	}
	if unsafeBot.GetUsernames()[0].GetUsername() != "bot_one" {
		t.Fatalf("expected first username kept, got %q", unsafeBot.GetUsernames()[0].GetUsername())
	}
}
