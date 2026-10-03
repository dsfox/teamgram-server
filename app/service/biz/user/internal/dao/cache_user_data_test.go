package dao

import (
	"testing"
)

// Red is colour 0, the first of the clients' seven; "none" is -1 (#24).
func TestRedIsAColourAndMinusOneIsNone(t *testing.T) {
	red := makePeerColor(0, 0)
	if red == nil || red.Color == nil || red.Color.Value != 0 {
		t.Fatalf("colour 0 must reach the client as red, got %v", red)
	}
	if none := makePeerColor(-1, 0); none != nil {
		t.Fatalf("-1 is no colour at all, got %v", none)
	}
	onlyEmoji := makePeerColor(-1, 42)
	if onlyEmoji == nil || onlyEmoji.Color != nil || onlyEmoji.BackgroundEmojiId_FLAGINT64.GetValue() != 42 {
		t.Fatalf("an emoji without a colour keeps the emoji and no colour, got %v", onlyEmoji)
	}
	if blue := makePeerColor(5, 0); blue.Color.GetValue() != 5 {
		t.Fatalf("got %v", blue)
	}
}
