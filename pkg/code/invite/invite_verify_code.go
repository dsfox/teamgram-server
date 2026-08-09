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
//     new person, or a familiar one on a new device.
//
// There is no SMS. Nobody gets in because they know a number.
package invite

import (
	"context"
	"crypto/subtle"
	"strconv"

	"github.com/teamgram/proto/mtproto"
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
	SetexCtx(ctx context.Context, key, value string, seconds int) error
	IncrCtx(ctx context.Context, key string) (int64, error)
	DelCtx(ctx context.Context, keys ...string) (int, error)
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

// SendSmsVerifyCode sends nothing: there is no SMS in this service. The code
// still travels to whoever already has a session, as a service message; this
// hands it back so the caller can store it.
func (v *verifier) SendSmsVerifyCode(_ context.Context, _, code, _ string) (string, error) {
	return code, nil
}

// VerifySmsCode decides whether this attempt gets in.
//
// extraData is the code the server generated for this attempt. Comparing
// against it - rather than against a constant - is the whole point.
func (v *verifier) VerifySmsCode(ctx context.Context, codeHash, code, extraData string) error {
	if code == "" {
		return mtproto.ErrPhoneCodeEmpty
	}

	if v.spent(ctx, codeHash) {
		logx.WithContext(ctx).Infof("sign-in refused: too many attempts on one code")
		return mtproto.ErrPhoneCodeInvalid
	}

	// Constant time, because the answer is a secret and how long it takes to
	// give should not hint at how much of it was right.
	if extraData != "" && subtle.ConstantTimeCompare([]byte(code), []byte(extraData)) == 1 {
		return nil
	}

	if v.useInvitation(ctx, code) {
		logx.WithContext(ctx).Infof("sign-in: invitation accepted")
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

// useInvitation spends an invitation if the code is one, and reports whether it
// did. An invitation works once: the deletion is what proves this attempt got
// it, so two people racing the same code cannot both be let in.
func (v *verifier) useInvitation(ctx context.Context, code string) bool {
	if v.store == nil {
		return false
	}

	key := InvitationKey(code)
	if _, err := v.store.GetCtx(ctx, key); err != nil {
		return false
	}

	deleted, err := v.store.DelCtx(ctx, key)
	if err != nil {
		logx.WithContext(ctx).Errorf("sign-in: cannot spend the invitation: %v", err)
		return false
	}

	return deleted == 1
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
