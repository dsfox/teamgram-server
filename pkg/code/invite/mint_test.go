package invite

import (
	"context"
	"testing"
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
