package invite

import (
	"context"
	"strconv"

	"github.com/teamgram/proto/mtproto"

	"github.com/zeromicro/go-zero/core/logx"
)

// Three tries per code request looked like a limit and was not one. A code
// request costs nothing, and a fresh connection gets a fresh request with three
// fresh tries - measured against the stand, thirty-two guesses went through in
// two seconds and nothing stopped them. At that rate a hundred million codes
// take about ten weeks in one thread, and a day in a hundred.
//
// So the count that matters follows the number, not the request. Ten wrong
// codes in an hour and that number stops answering until the hour is out: a
// person who mistypes has room, and a hundred million codes now take a lifetime.
//
// The cost is that somebody can make a number wait an hour by guessing at it
// badly. That is a nuisance and it is visible in the log; the alternative is an
// account that can be opened by a patient stranger, which is not.
const (
	maxFailuresPerPhone = 10
	failureWindow       = 60 * 60
)

const failuresPrefix = "signin_failures:"

func failuresKey(phone string) string {
	return failuresPrefix + Digits(phone)
}

// guessedAtTooOften reports whether this number has to wait, and for how long.
// It is asked before anything is compared, so that guessing costs the guesser
// a refusal rather than costing us the work of checking.
func (v *verifier) guessedAtTooOften(ctx context.Context, phone string) (bool, int) {
	if v.store == nil || phone == "" {
		return false, 0
	}

	value, err := v.store.GetCtx(ctx, failuresKey(phone))
	if err != nil {
		// Not being able to count is no reason to let anybody guess freely, but
		// it is no reason to lock everybody out either. It has to be visible.
		logx.WithContext(ctx).Errorf("sign-in: cannot read the failure count: %v", err)
		return false, 0
	}

	failures, _ := strconv.Atoi(value)
	if failures < maxFailuresPerPhone {
		return false, 0
	}

	return true, failureWindow
}

// recordFailure counts a wrong code against the number it was aimed at.
func (v *verifier) recordFailure(ctx context.Context, phone string) {
	if v.store == nil || phone == "" {
		return
	}

	key := failuresKey(phone)
	failures, err := v.store.IncrCtx(ctx, key)
	if err != nil {
		logx.WithContext(ctx).Errorf("sign-in: cannot count the failure: %v", err)
		return
	}

	if failures == 1 {
		// The window starts at the first wrong code and is not extended by the
		// ones after it, so an hour of silence always clears it.
		if err = v.store.SetexCtx(ctx, key, "1", failureWindow); err != nil {
			logx.WithContext(ctx).Errorf("sign-in: cannot age the failure count: %v", err)
		}
	}

	if failures >= maxFailuresPerPhone {
		logx.WithContext(ctx).Errorf(
			"sign-in: %d wrong codes for one number, it waits an hour now", failures)
	}
}

// forgetFailures clears the count once somebody proves they belong here.
func (v *verifier) forgetFailures(ctx context.Context, phone string) {
	if v.store == nil || phone == "" {
		return
	}
	if _, err := v.store.DelCtx(ctx, failuresKey(phone)); err != nil {
		logx.WithContext(ctx).Errorf("sign-in: cannot clear the failure count: %v", err)
	}
}

// errWait is what a number that has been guessed at too often gets back. It
// says what is wrong and what fixes it, which "invalid code" does not: both
// clients show the wait and count it down.
func errWait(seconds int) error {
	return mtproto.NewErrFloodWaitX(int32(seconds))
}
