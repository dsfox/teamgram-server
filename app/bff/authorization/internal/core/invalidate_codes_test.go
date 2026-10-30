package core

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// iOS asks the server to forget any login code it sees in a chat, and it reads
// a build number in a service message as one (29 October: 428880, 427380,
// 70719, 425940). The upstream stub refused with a licence error, which the
// health check counts as an alarm. Codes here die by themselves within three
// minutes and after one use, so the honest answer is yes (#294).
func TestForgettingCodesIsAnsweredYes(t *testing.T) {
	c := &AuthorizationCore{}
	answer, err := c.AccountInvalidateSignInCodes(&mtproto.TLAccountInvalidateSignInCodes{
		Codes: []string{"428880", "427380", "70719", "425940"},
	})
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if answer != mtproto.BoolTrue {
		t.Fatalf("answered %v, want true", answer)
	}
}
