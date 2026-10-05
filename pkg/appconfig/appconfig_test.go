package appconfig

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// iOS hides its AI button only when ios_disable_ai_chat is the number 1; a
// bool, or no key at all, leaves a button the server cannot answer.
func TestTheAIButtonIsSwitchedOffAsANumber(t *testing.T) {
	found := false
	for _, value := range Value().Value_VECTORJSONOBJECTVALUE {
		if value.Key != "ios_disable_ai_chat" {
			continue
		}
		found = true
		if value.Value.GetPredicateName() != mtproto.Predicate_jsonNumber || value.Value.Value_FLOAT64 != 1 {
			t.Fatalf("ios_disable_ai_chat is %s %v, iOS wants the number 1", value.Value.GetPredicateName(), value.Value)
		}
	}
	if !found {
		t.Fatal("no ios_disable_ai_chat: the AI button stays")
	}
	x := mtproto.NewEncodeBuf(512)
	if err := Value().Encode(x, 228); err != nil {
		t.Fatalf("the settings do not encode: %v", err)
	}
}
