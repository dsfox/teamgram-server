package invite

import "testing"

// A code minted from a group remembers it (#164); a code minted before the
// field existed decodes as it always did - into ice9 alone.
func TestAnInvitationRemembersItsGroup(t *testing.T) {
	back := Decode(Encode(Invitation{Phone: "79991234567", Note: "1", Chat: 120090}))
	if back.Chat != 120090 || back.Phone != "79991234567" {
		t.Fatalf("came back as %+v", back)
	}
	old := Decode(`{"phone":"79991234567","note":"1"}`)
	if old.Chat != 0 {
		t.Fatalf("an old code claims chat %d", old.Chat)
	}
}
