package core

import (
	"encoding/json"
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// Both phones poll this every twelve hours and hand the text to their engines,
// which read every key with a default. The one thing that would hurt is text
// that is not a JSON object.
func TestCallConfigIsAJSONObject(t *testing.T) {
	s := newStand(t)

	reply, err := s.as(alice).PhoneGetCallConfig(&mtproto.TLPhoneGetCallConfig{})
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if reply.GetPredicateName() != mtproto.Predicate_dataJSON {
		t.Fatalf("answered with %s", reply.GetPredicateName())
	}

	var object map[string]any
	if err := json.Unmarshal([]byte(reply.GetData()), &object); err != nil {
		t.Fatalf("the config %q is not a JSON object: %v", reply.GetData(), err)
	}
	if len(s.sync.pushes) != 0 {
		t.Fatalf("asking for the config pushed %d updates", len(s.sync.pushes))
	}
}
