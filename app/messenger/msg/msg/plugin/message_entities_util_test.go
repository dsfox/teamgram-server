package plugin

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// A client that marks its own @mention used to get it marked twice: its entity
// travelled through untouched, and the server then scanned the text and added
// one of its own beside it. Two marks over one word, and the second a character
// longer or shorter whenever the two disagreed about whether the "@" counts.
//
// Found by the scenario that sends every kind of text and reads back what
// arrived - which is the whole reason that scenario exists.

func aMention(offset, length int32) *mtproto.MessageEntity {
	return mtproto.MakeTLMessageEntityMention(&mtproto.MessageEntity{
		Offset: offset,
		Length: length,
	}).To_MessageEntity()
}

func bold(offset, length int32) *mtproto.MessageEntity {
	return mtproto.MakeTLMessageEntityBold(&mtproto.MessageEntity{
		Offset: offset,
		Length: length,
	}).To_MessageEntity()
}

func TestOneMentionIsMarkedOnce(t *testing.T) {
	already := mtproto.MessageEntitySlice{aMention(6, 17)}
	if !markedAlready(already, 6) {
		t.Fatal("a mention at this offset was not noticed, so a second one gets added beside it")
	}
}

func TestAnotherWordIsStillMarked(t *testing.T) {
	already := mtproto.MessageEntitySlice{aMention(6, 17)}
	if markedAlready(already, 40) {
		t.Fatal("a mention elsewhere in the line counted as this one, so it would go unmarked")
	}
}

// Two marks of different kinds over one word are ordinary - somebody writing a
// mention in bold - and only a second mention over the first means nothing.
func TestBoldOverAWordIsNotAMention(t *testing.T) {
	already := mtproto.MessageEntitySlice{bold(6, 17)}
	if markedAlready(already, 6) {
		t.Fatal("bold at this offset was taken for a mention, so the mention would be lost")
	}
}

func TestNothingMarkedYet(t *testing.T) {
	if markedAlready(mtproto.MessageEntitySlice{}, 0) {
		t.Fatal("an empty list reported a mark in it")
	}
}
