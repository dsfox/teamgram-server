package invite

import (
	"context"
	"strconv"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/code/attempt"

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
// The same count follows the address a run of wrong codes comes from, because
// working on one account is not the only shape this takes: somebody can try a
// few codes each against a thousand numbers and never trip a per-number limit.
//
// It is deliberately generous and deliberately short. An address is not a
// person - whole regions of a mobile operator share one, so a threshold tight
// enough to be clever would shut out bystanders - and it counts only failures,
// which a person who types their phrase correctly never produces. What it must
// never become is the thing it was nearly built as: a rule that refuses correct
// codes for a day. That hands anybody a way to keep somebody locked out
// forever, three wrong tries at a time, from an address of their choosing.
const (
	maxFailuresPerPhone = 10
	failureWindow       = 60 * 60

	maxFailuresPerAddr = 60
	addrWindow         = 15 * 60
)

const (
	failuresPrefix     = "signin_failures:"
	addrFailuresPrefix = "signin_failures_addr:"
)

func failuresKey(phone string) string {
	return failuresPrefix + Digits(phone)
}

func addrFailuresKey(addr string) string {
	return addrFailuresPrefix + addr
}

// guessedAtTooOften reports whether this attempt has to wait, and for how long.
// It is asked before anything is compared, so that guessing costs the guesser
// a refusal rather than costing us the work of checking.
func (v *verifier) guessedAtTooOften(ctx context.Context, a attempt.Attempt) (bool, int) {
	if v.store == nil {
		return false, 0
	}

	if v.failures(ctx, failuresKey(a.PhoneNumber), a.PhoneNumber != "") >= maxFailuresPerPhone {
		logx.WithContext(ctx).Infof("sign-in refused: this number is waiting out its guesses")
		return true, failureWindow
	}

	if v.failures(ctx, addrFailuresKey(a.ClientAddr), a.ClientAddr != "") >= maxFailuresPerAddr {
		logx.WithContext(ctx).Errorf(
			"sign-in refused: %s has been wrong about too many numbers", a.ClientAddr)
		return true, addrWindow
	}

	return false, 0
}

// failures reads one of the counters. Not being able to count is no reason to
// let anybody guess freely, and no reason to lock everybody out either - so it
// reads as zero and says so in the log.
func (v *verifier) failures(ctx context.Context, key string, known bool) int {
	if !known {
		return 0
	}

	value, err := v.store.GetCtx(ctx, key)
	if err != nil {
		logx.WithContext(ctx).Errorf("sign-in: cannot read the failure count: %v", err)
		return 0
	}

	failures, _ := strconv.Atoi(value)
	return failures
}

// recordFailure counts a wrong code against the number it was aimed at and the
// address it came from.
func (v *verifier) recordFailure(ctx context.Context, a attempt.Attempt) {
	if v.store == nil {
		return
	}

	if a.PhoneNumber != "" {
		if n := v.countFailure(ctx, failuresKey(a.PhoneNumber), failureWindow); n >= maxFailuresPerPhone {
			logx.WithContext(ctx).Errorf(
				"sign-in: %d wrong codes for one number, it waits an hour now", n)
		}
	}

	if a.ClientAddr != "" {
		if n := v.countFailure(ctx, addrFailuresKey(a.ClientAddr), addrWindow); n >= maxFailuresPerAddr {
			logx.WithContext(ctx).Errorf(
				"sign-in: %d wrong codes from %s, it waits now", n, a.ClientAddr)
		}
	}
}

// countFailure adds one to a counter and gives it a life of its own. The window
// starts at the first wrong code and is not extended by the ones after it, so
// silence always clears it.
func (v *verifier) countFailure(ctx context.Context, key string, window int) int {
	failures, err := v.store.IncrCtx(ctx, key)
	if err != nil {
		logx.WithContext(ctx).Errorf("sign-in: cannot count the failure: %v", err)
		return 0
	}

	if failures == 1 {
		if err = v.store.SetexCtx(ctx, key, "1", window); err != nil {
			logx.WithContext(ctx).Errorf("sign-in: cannot age the failure count: %v", err)
		}
	}

	return int(failures)
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
