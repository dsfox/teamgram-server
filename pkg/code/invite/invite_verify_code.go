// Package invite verifies sign-in codes without sending anything anywhere.
//
// What it replaces was a placeholder that compared the typed code against the
// constant "12345" and ignored the real one, which the server had generated,
// stored and delivered. Anyone who knew a phone number could sign in as its
// owner - measured, not supposed: entering 12345 for a registered number
// returned that account's session.
//
// Two things are accepted here, and nothing else:
//
//   - the code the server generated for this attempt, which reaches a person
//     who already has the app open somewhere as a service message;
//   - an invitation minted by the owner, for a phone with no session at all - a
//     new person, or a familiar one on a new device;
//   - the account's own recovery code, handed to its owner once and kept by
//     them, which is the only way back that needs nobody else awake.
//
// An invitation is bound to what it may open. One minted for a number opens
// that number and no other; one minted for nobody in particular opens only a
// number that has no account yet. Without that rule any invitation was a key to
// every account whose number somebody knew - measured the same way the first
// hole was, and closed the same day.
//
// There is no SMS. Nobody gets in because they know a number.
package invite

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/code/attempt"
	"github.com/teamgram/teamgram-server/pkg/code/conf"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/kv"
)

// Guessing is the attack this has to survive: the code is five digits, and a
// hundred thousand of them is not many. Three tries spend the attempt; asking
// for another code starts a new one, which is visible in the log.
const maxAttempts = 3

// How long a spent-attempt counter is remembered. Longer than the code itself
// lives, so that asking again does not clear the count.
const attemptsTTL = 15 * 60

const (
	attemptsPrefix   = "signin_attempts:"
	invitationPrefix = "invitation:"
)

// Store is the part of the key-value store this needs. Named here so the
// verifier can be tested with a map instead of a Redis.
type Store interface {
	GetCtx(ctx context.Context, key string) (string, error)
	SetCtx(ctx context.Context, key, value string) error
	SetexCtx(ctx context.Context, key, value string, seconds int) error
	IncrCtx(ctx context.Context, key string) (int64, error)
	DelCtx(ctx context.Context, keys ...string) (int, error)
}

// Invitation is what an invitation code stands for: who it is for, and what it
// is allowed to open.
type Invitation struct {
	// Phone, when set, is the only number this invitation opens. Empty means
	// the invitation is for somebody new, and then it opens only a number that
	// has no account.
	Phone string `json:"phone,omitempty"`

	// Note is for the person who minted it, and is never checked.
	Note string `json:"note,omitempty"`
}

type verifier struct {
	store Store
}

func New(_ *conf.SmsVerifyCodeConfig, store Store) *verifier {
	return &verifier{store: store}
}

// InvitationKey is where an invitation lives. Exported so that whatever mints
// them agrees with whatever spends them.
func InvitationKey(code string) string {
	return invitationPrefix + code
}

// Digits reduces a phone number to what can be compared. The client sends one
// spelling and the person minting an invitation types another; "+7 999 12-34"
// and "79991234" are the same number and must not fail to match over a plus.
func Digits(phone string) string {
	var out strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// Encode turns an invitation into what is stored under its code.
func Encode(inv Invitation) string {
	body, err := json.Marshal(inv)
	if err != nil {
		// The struct is two strings; this cannot fail, and if it ever did, the
		// note is the part worth losing.
		return `{"phone":"` + inv.Phone + `"}`
	}
	return string(body)
}

// Decode reads a stored invitation. Anything unparseable is treated as a note
// with no binding, which is what the first invitations written by hand were.
func Decode(value string) Invitation {
	var inv Invitation
	if err := json.Unmarshal([]byte(value), &inv); err != nil {
		return Invitation{Note: value}
	}
	return inv
}

// SendSmsVerifyCode sends nothing: there is no SMS in this service. The code
// still travels to whoever already has a session, as a service message; this
// hands it back so the caller can store it.
func (v *verifier) SendSmsVerifyCode(_ context.Context, _, code, _ string) (string, error) {
	return code, nil
}

// VerifySmsCode decides whether this attempt gets in.
func (v *verifier) VerifySmsCode(ctx context.Context, a attempt.Attempt) error {
	if a.Code == "" {
		return mtproto.ErrPhoneCodeEmpty
	}

	if v.spent(ctx, a.CodeHash) {
		logx.WithContext(ctx).Infof("sign-in refused: too many attempts on one code")
		return mtproto.ErrPhoneCodeInvalid
	}

	// The code the server generated reached a session of this very account, so
	// having it is proof enough. Constant time, because the answer is a secret
	// and how long it takes to give should not hint at how much of it was right.
	if a.Generated != "" &&
		subtle.ConstantTimeCompare([]byte(a.Code), []byte(a.Generated)) == 1 {
		return nil
	}

	if v.useInvitation(ctx, a) {
		logx.WithContext(ctx).Infof("sign-in: invitation accepted")
		return nil
	}

	if v.useRecovery(ctx, a.PhoneNumber, a.Code) {
		return nil
	}

	logx.WithContext(ctx).Infof("sign-in refused: wrong code")
	return mtproto.ErrPhoneCodeInvalid
}

// spent counts this attempt and reports whether the allowance is used up.
func (v *verifier) spent(ctx context.Context, codeHash string) bool {
	if v.store == nil || codeHash == "" {
		return false
	}

	key := attemptsPrefix + codeHash
	used, err := v.store.IncrCtx(ctx, key)
	if err != nil {
		// Not being able to count is no reason to let anybody in, and no reason
		// to lock everybody out either. It has to be visible.
		logx.WithContext(ctx).Errorf("sign-in: cannot count attempts: %v", err)
		return false
	}

	if used == 1 {
		// The counter has to expire by itself, or a code hash remembers forever.
		if err = v.store.SetexCtx(ctx, key, "1", attemptsTTL); err != nil {
			logx.WithContext(ctx).Errorf("sign-in: cannot age the attempt counter: %v", err)
		}
	}

	return used > maxAttempts
}

// useInvitation spends an invitation if the code is one this attempt may use,
// and reports whether it did.
func (v *verifier) useInvitation(ctx context.Context, a attempt.Attempt) bool {
	if v.store == nil {
		return false
	}

	key := InvitationKey(a.Code)
	value, err := v.store.GetCtx(ctx, key)
	if err != nil {
		logx.WithContext(ctx).Errorf("sign-in: cannot read the invitation: %v", err)
		return false
	}
	if value == "" {
		return false
	}

	if !allows(Decode(value), a) {
		// Left where it is: refusing must not burn somebody else's invitation,
		// or knowing a code would be enough to cancel it.
		logx.WithContext(ctx).Infof("sign-in refused: this invitation is not for this number")
		return false
	}

	// The deletion is the proof. Reading said the invitation was there; only
	// the delete says this attempt is the one that took it, which is what stops
	// two people racing the same code from both getting in.
	deleted, err := v.store.DelCtx(ctx, key)
	if err != nil {
		logx.WithContext(ctx).Errorf("sign-in: cannot spend the invitation: %v", err)
		return false
	}

	return deleted == 1
}

// allows is the rule an invitation is worth stating on its own: an invitation
// naming a number opens that number, and one naming nobody opens only a number
// that has no account behind it yet.
func allows(inv Invitation, a attempt.Attempt) bool {
	if inv.Phone != "" {
		return Digits(inv.Phone) == Digits(a.PhoneNumber)
	}
	return !a.PhoneRegistered
}

// Attempts reports how many tries a code has had, for the tool that mints
// invitations to show and for tests to read.
func Attempts(ctx context.Context, store kv.Store, codeHash string) int {
	value, err := store.GetCtx(ctx, attemptsPrefix+codeHash)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(value)
	return n
}
