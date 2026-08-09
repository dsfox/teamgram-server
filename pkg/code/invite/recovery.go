package invite

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

// A recovery code is what a person types when the phone that held their account
// is gone. Nothing else can reach them: there is no SMS here, and the ordinary
// code goes to a session they no longer have. So the code is handed over in
// advance, once, and kept by them.
//
// It is eight digits because it has to survive guessing without a person on the
// other end noticing: a hundred million of them, three tries per request, and
// the stored form is a bcrypt hash, so reading the whole key-value store yields
// nothing that opens anything.
const recoveryDigits = 8

const recoveryPrefix = "recovery:"

func recoveryKey(phone string) string {
	return recoveryPrefix + Digits(phone)
}

// HasRecoveryCode says whether this number already has one. Minting a second
// would silently replace the first, and somebody has that one written down.
func HasRecoveryCode(ctx context.Context, store Store, phone string) bool {
	value, err := store.GetCtx(ctx, recoveryKey(phone))
	if err != nil {
		logx.WithContext(ctx).Errorf("recovery: cannot read the code: %v", err)
		return true // Not knowing is no reason to hand out a second one.
	}
	return value != ""
}

// MintRecoveryCode makes a code for this number and stores what it takes to
// check it - never the code itself. The code is returned once, to be delivered;
// after that nobody, including us, can read it back.
func MintRecoveryCode(ctx context.Context, store Store, phone string) (string, error) {
	code, err := digits(recoveryDigits)
	if err != nil {
		return "", err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("cannot hash the recovery code: %w", err)
	}

	// No expiry: it is worth nothing until the phone is lost, and that can
	// happen at any time.
	if err = store.SetCtx(ctx, recoveryKey(phone), string(hashed)); err != nil {
		return "", fmt.Errorf("cannot store the recovery code: %w", err)
	}

	return code, nil
}

// MoveRecoveryCode follows an account onto a new number, so that changing the
// number does not quietly leave the way back pointing at the old one.
func MoveRecoveryCode(ctx context.Context, store Store, from, to string) {
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

// useRecovery spends this number's recovery code if that is what was typed.
//
// It is spent whether or not the person meant to use it: a code that survived
// being used would be a code that survives being read over somebody's shoulder.
// The next successful sign-in mints another and delivers it.
func (v *verifier) useRecovery(ctx context.Context, phone, code string) bool {
	if v.store == nil || phone == "" {
		return false
	}

	stored, err := v.store.GetCtx(ctx, recoveryKey(phone))
	if err != nil {
		logx.WithContext(ctx).Errorf("recovery: cannot read the code: %v", err)
		return false
	}
	if stored == "" {
		return false
	}

	if err = bcrypt.CompareHashAndPassword([]byte(stored), []byte(code)); err != nil {
		return false
	}

	// The deletion is what makes it single use, and it has to say so: if the
	// store refuses, the code is still out there and letting this attempt in
	// would leave a second person able to use it.
	if _, err = v.store.DelCtx(ctx, recoveryKey(phone)); err != nil {
		logx.WithContext(ctx).Errorf("recovery: cannot spend the code: %v", err)
		return false
	}

	logx.WithContext(ctx).Infof("sign-in: recovery code accepted")
	return true
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
