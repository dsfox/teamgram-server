package appconfig

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// iOS hides its AI buttons only when these are the number 1; a bool, or no key
// at all, leaves a button the server cannot answer. The message field reads
// one key, a caption in the legacy gallery the other (#181, #227).
func TestTheAIButtonIsSwitchedOffAsANumber(t *testing.T) {
	for _, key := range []string{"ios_disable_ai_chat", "ios_disable_ai_attach"} {
		found := false
		for _, value := range Value().Value_VECTORJSONOBJECTVALUE {
			if value.Key != key {
				continue
			}
			found = true
			if value.Value.GetPredicateName() != mtproto.Predicate_jsonNumber || value.Value.Value_FLOAT64 != 1 {
				t.Fatalf("%s is %s %v, iOS wants the number 1", key, value.Value.GetPredicateName(), value.Value)
			}
		}
		if !found {
			t.Fatalf("no %s: the AI button stays", key)
		}
	}
	x := mtproto.NewEncodeBuf(512)
	if err := Value().Encode(x, 228); err != nil {
		t.Fatalf("the settings do not encode: %v", err)
	}
}
