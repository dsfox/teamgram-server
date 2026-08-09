package invite

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

// A recovery phrase is what a person types when the phone that held their
// account is gone. Nothing else can reach them: there is no SMS here, and the
// ordinary code goes to a session they no longer have. So it is handed over in
// advance, once, and kept by them - on paper, away from the phone.
//
// What is stored is a bcrypt hash of the phrase, never the phrase: reading the
// whole key-value store yields nothing that opens anything. See phrase.go for
// why six words rather than the eight digits this started as.

// recoveryDigits is the length of a sign-in code - an invitation, or the code
// the server delivers to a session. Not the recovery phrase, which is words.
// Eight because the clients draw exactly as many boxes as the server declares.
const recoveryDigits = 8

const recoveryPrefix = "recovery:"

func recoveryKey(phone string) string {
	return recoveryPrefix + Digits(phone)
}

// storedRecovery is what is kept for a number: enough to check a code, and
// whether the person was ever actually told it.
//
// The second half matters more than it looks. A code minted by hand, for
// somebody who cannot sign in at all, might never reach them - and without this
// the server would see a code, decide the account was covered and never hand
// out another. An account with a way back that nobody knows is worse than one
// with none, because nothing ever says so.
type storedRecovery struct {
	Hash      string `json:"h"`
	Delivered bool   `json:"d"`
}

func readRecovery(value string) (storedRecovery, bool) {
	if value == "" {
		return storedRecovery{}, false
	}
	var stored storedRecovery
	if err := json.Unmarshal([]byte(value), &stored); err != nil || stored.Hash == "" {
		// Written before this carried anything but the hash, and those were all
		// minted and delivered by the server itself.
		return storedRecovery{Hash: value, Delivered: true}, true
	}
	return stored, true
}

// HasRecoveryPhrase says whether this number has a code at all, delivered or not.
// Minting a second would silently replace the first, and somebody may have that
// one written down.
func HasRecoveryPhrase(ctx context.Context, store Store, phone string) bool {
	value, err := store.GetCtx(ctx, recoveryKey(phone))
	if err != nil {
		logx.WithContext(ctx).Errorf("recovery: cannot read the code: %v", err)
		return true // Not knowing is no reason to hand out a second one.
	}
	_, ok := readRecovery(value)
	return ok
}

// HasDeliveredRecoveryPhrase says whether this account has a way back that it was
// actually told about. This, not the one above, is what decides whether the
// server hands one over.
func HasDeliveredRecoveryPhrase(ctx context.Context, store Store, phone string) bool {
	value, err := store.GetCtx(ctx, recoveryKey(phone))
	if err != nil {
		logx.WithContext(ctx).Errorf("recovery: cannot read the code: %v", err)
		return true
	}
	stored, ok := readRecovery(value)
	return ok && stored.Delivered
}

// MintRecoveryPhrase makes a phrase for this number and stores what it takes to
// check it - never the phrase itself. It is returned once, to be delivered;
// after that nobody, including us, can read it back.
//
// delivered says whether it is about to reach the person by itself. The server
// sets it; the tool that mints one by hand does not, because whether it gets
// passed along is not something the tool can know.
func MintRecoveryPhrase(ctx context.Context, store Store, phone string, delivered bool) (string, error) {
	phrase, err := Phrase()
	if err != nil {
		return "", err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(phrase), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("cannot hash the recovery phrase: %w", err)
	}

	body, err := json.Marshal(storedRecovery{Hash: string(hashed), Delivered: delivered})
	if err != nil {
		return "", fmt.Errorf("cannot record the recovery code: %w", err)
	}

	// No expiry: it is worth nothing until the phone is lost, and that can
	// happen at any time.
	if err = store.SetCtx(ctx, recoveryKey(phone), string(body)); err != nil {
		return "", fmt.Errorf("cannot store the recovery phrase: %w", err)
	}

	return phrase, nil
}

// MoveRecoveryPhrase follows an account onto a new number, so that changing the
// number does not quietly leave the way back pointing at the old one.
func MoveRecoveryPhrase(ctx context.Context, store Store, from, to string) {
	value, err := store.GetCtx(ctx, recoveryKey(from))
	if err != nil || value == "" {
		return
	}
	if err = store.SetCtx(ctx, recoveryKey(to), value); err != nil {
		logx.WithContext(ctx).Errorf("recovery: cannot move the code to the new number: %v", err)
		return
	}
	if _, err = store.DelCtx(ctx, recoveryKey(from)); err != nil {
		logx.WithContext(ctx).Errorf("recovery: the code stayed on the old number: %v", err)
	}
}

// useRecovery spends this number's recovery phrase if that is what was typed.
// What arrives is normalized first: a person typing their paper back adds a
// capital or a double space, and none of that is a different phrase. A code
// from before phrases existed passes through this untouched.
//
// It is spent whether or not the person meant to use it: a code that survived
// being used would be a code that survives being read over somebody's shoulder.
// The next successful sign-in mints another and delivers it.
func (v *verifier) useRecovery(ctx context.Context, phone, code string) bool {
	if v.store == nil || phone == "" {
		return false
	}

	value, err := v.store.GetCtx(ctx, recoveryKey(phone))
	if err != nil {
		logx.WithContext(ctx).Errorf("recovery: cannot read the code: %v", err)
		return false
	}
	stored, ok := readRecovery(value)
	if !ok {
		return false
	}

	if err = bcrypt.CompareHashAndPassword(
		[]byte(stored.Hash), []byte(NormalizePhrase(code))); err != nil {
		return false
	}

	// The deletion is what makes it single use, and it has to say so: if the
	// store refuses, the code is still out there and letting this attempt in
	// would leave a second person able to use it.
	if _, err = v.store.DelCtx(ctx, recoveryKey(phone)); err != nil {
		logx.WithContext(ctx).Errorf("recovery: cannot spend the code: %v", err)
		return false
	}

	logx.WithContext(ctx).Infof("sign-in: recovery phrase accepted")
	return true
}

// Code is a sign-in code with no pattern to it, the length everything here
// uses. Exported so that whatever mints one cannot pick a length the client
// cannot type: the code field is exactly as many boxes as the server declares.
func Code() string {
	code, err := digits(recoveryDigits)
	if err != nil {
		// Falling back to something predictable would defeat the point.
		panic(err)
	}
	return code
}

// digits is a code with no pattern to it. crypto/rand and a rejection-free
// modulus: guessing is the attack this has to survive.
func digits(n int) (string, error) {
	out := strings.Builder{}
	for i := 0; i < n; i++ {
		v, err := crand.Int(crand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("no randomness available: %w", err)
		}
		out.WriteByte(byte('0' + v.Int64()))
	}
	return out.String(), nil
}
