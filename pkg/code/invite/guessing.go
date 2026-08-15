package invite

import (
	"context"
	"strconv"
	"time"

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

// And between those two, a wait that doubles. Ten wrong codes in an hour is a
// ceiling, not a cost: a machine can spend nine of them in under a second and
// come back next hour, for ever. Doubling makes the tenth guess cost more than
// the first nine together, and it costs a person who mistyped their phrase
// twice about two seconds - which is the difference the phrase needs, because
// six words are 2^66 and the only way to make that number matter is to charge
// for every attempt at it.
const (
	// Nobody is charged for the first few. A person mistypes six words they
	// copied off paper - twice, if the paper is old - and making them wait for
	// their own account is a punishment aimed at the wrong person. A machine
	// does not stop at three, which is the whole difference between them.
	freeMisses  = 3
	firstWait   = 1
	longestWait = 20 * 60
)

const (
	failuresPrefix     = "signin_failures:"
	addrFailuresPrefix = "signin_failures_addr:"
	waitPrefix         = "signin_wait:"
)

func waitKey(phone string) string {
	return waitPrefix + Digits(phone)
}

// waiting reports how many seconds this number still owes, if any. The value is
// the moment it may try again, so a clock that survives a restart of the store
// is the whole mechanism - a counter that resets when the server falls over is
// an invitation to make it fall over.
func (v *verifier) waiting(ctx context.Context, phone string) int {
	if v.store == nil || phone == "" {
		return 0
	}
	value, err := v.store.GetCtx(ctx, waitKey(phone))
	if err != nil {
		logx.WithContext(ctx).Errorf("sign-in: cannot read the wait: %v", err)
		return 0
	}
	until, _ := strconv.ParseInt(value, 10, 64)
	if left := until - time.Now().Unix(); left > 0 {
		return int(left)
	}
	return 0
}

// charge makes the next attempt wait, twice as long as the last one did.
func (v *verifier) charge(ctx context.Context, phone string, failures int) {
	if v.store == nil || phone == "" || failures < 1 {
		return
	}
	if failures <= freeMisses {
		return
	}
	seconds := firstWait
	for i := freeMisses + 1; i < failures && seconds < longestWait; i++ {
		seconds *= 2
	}
	if seconds > longestWait {
		seconds = longestWait
	}
	until := strconv.FormatInt(time.Now().Unix()+int64(seconds), 10)
	if err := v.store.SetexCtx(ctx, waitKey(phone), until, seconds+1); err != nil {
		logx.WithContext(ctx).Errorf("sign-in: cannot charge for the guess: %v", err)
	}
}

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

	if seconds := v.waiting(ctx, a.PhoneNumber); seconds > 0 {
		logx.WithContext(ctx).Infof("sign-in refused: this number owes %ds", seconds)
		return true, seconds
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
		n := v.countFailure(ctx, failuresKey(a.PhoneNumber), failureWindow)
		v.charge(ctx, a.PhoneNumber, n)
		if n >= maxFailuresPerPhone {
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
	if _, err := v.store.DelCtx(ctx, failuresKey(phone), waitKey(phone)); err != nil {
		logx.WithContext(ctx).Errorf("sign-in: cannot clear the failure count: %v", err)
	}
}

// errWait is what a number that has been guessed at too often gets back. It
// says what is wrong and what fixes it, which "invalid code" does not: both
// clients show the wait and count it down.
func errWait(seconds int) error {
	return mtproto.NewErrFloodWaitX(int32(seconds))
}
