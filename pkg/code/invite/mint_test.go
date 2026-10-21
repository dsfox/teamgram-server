package invite

import (
	"context"
	"testing"

	"github.com/teamgram/teamgram-server/pkg/code/attempt"
)

func TestMintWritesABoundInvitation(t *testing.T) {
	store := &mapStore{data: map[string]string{}}
	code, err := Mint(context.Background(), store, 3600, Invitation{Phone: "+79991234567", Note: "42"})
	if err != nil {
		t.Fatalf("cannot mint: %v", err)
	}
	if len(code) != recoveryDigits {
		t.Fatalf("a code of %d digits, want %d", len(code), recoveryDigits)
	}
	inv := Decode(store.data[InvitationKey(code)])
	if inv.Phone != "+79991234567" || inv.Note != "42" {
		t.Fatalf("stored %+v", inv)
	}
	if !contains(outstandingOf(t, store), code) {
		t.Fatal("the code is not listed as outstanding")
	}
}

func TestRevokeForgetsAnInvitation(t *testing.T) {
	store := &mapStore{data: map[string]string{}}
	code, _ := Mint(context.Background(), store, 3600, Invitation{Phone: "+79991234567"})
	if err := Revoke(context.Background(), store, code); err != nil {
		t.Fatalf("cannot revoke: %v", err)
	}
	if store.data[InvitationKey(code)] != "" {
		t.Fatal("the invitation is still there")
	}
}

// The reviewer's code (#214): it opens the demo account on every review, and
// that number only.
const reviewerPhone = "+1" + "555" + "0000158"

func TestALastingInvitationOpensItsNumberEveryTime(t *testing.T) {
	ctx := context.Background()
	store := &mapStore{data: map[string]string{}}
	code, err := Mint(ctx, store, 0, Invitation{Phone: reviewerPhone, Lasting: true})
	if err != nil {
		t.Fatalf("cannot mint: %v", err)
	}
	if _, expiring := store.expiring[InvitationKey(code)]; expiring {
		t.Fatal("a lasting invitation was written with an expiry")
	}
	v := New(nil, store)
	for i, hash := range []string{"r-1", "r-2", "r-3"} {
		if err := v.VerifySmsCode(ctx, attempt.Attempt{CodeHash: hash, Code: code,
			PhoneNumber: reviewerPhone, PhoneRegistered: i > 0}); err != nil {
			t.Fatalf("sign-in %d with the lasting code was refused: %v", i+1, err)
		}
	}
	if err := v.VerifySmsCode(ctx, attempt.Attempt{CodeHash: "r-4", Code: code,
		PhoneNumber: "+799****0001"}); err == nil {
		t.Fatal("the lasting code opened a number it does not name")
	}
}

func TestALastingInvitationNeedsANumber(t *testing.T) {
	store := &mapStore{data: map[string]string{}}
	if code, err := Mint(context.Background(), store, 0, Invitation{Lasting: true}); err == nil {
		t.Fatalf("minted a lasting invitation for anybody: %s", code)
	}
	// And one written by hand without a number opens nothing.
	store.data[InvitationKey("424242")] = Encode(Invitation{Lasting: true})
	if err := New(nil, store).VerifySmsCode(context.Background(), attempt.Attempt{
		CodeHash: "r-5", Code: "424242", PhoneNumber: "+799****0002"}); err == nil {
		t.Fatal("a lasting invitation naming nobody let somebody new in")
	}
}

func TestRevokeForKeepsOnlyTheNewCode(t *testing.T) {
	ctx := context.Background()
	store := &mapStore{data: map[string]string{}}
	old1, _ := Mint(ctx, store, 3600, Invitation{Phone: reviewerPhone})
	old2, _ := Mint(ctx, store, 3600, Invitation{Phone: "+1 555 000 0158"})
	other, _ := Mint(ctx, store, 3600, Invitation{Phone: "+799****4567"})
	fresh, _ := Mint(ctx, store, 0, Invitation{Phone: reviewerPhone, Lasting: true})
	gone, err := RevokeFor(ctx, store, reviewerPhone, fresh)
	if err != nil || gone != 2 {
		t.Fatalf("revoked %d (%v), want the 2 earlier codes", gone, err)
	}
	for _, code := range []string{old1, old2} {
		if store.data[InvitationKey(code)] != "" {
			t.Fatalf("an earlier reviewer code %s still opens the account", code)
		}
	}
	if store.data[InvitationKey(fresh)] == "" || store.data[InvitationKey(other)] == "" {
		t.Fatal("revoking the reviewer's earlier codes took the new one or somebody else's")
	}
}

func contains(codes []string, code string) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

func outstandingOf(t *testing.T, store Store) []string {
	t.Helper()
	var codes []string
	for _, o := range Outstanding(context.Background(), store) {
		codes = append(codes, o.Code)
	}
	return codes
}
