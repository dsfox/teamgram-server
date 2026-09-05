package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Sixty lookups an hour is plenty for the one honest use of
// contacts.resolvePhone - inviting somebody by their number, one at a time -
// and far too few for walking the numbers of a country to learn who is here
// (#170). Counted per account, per hour of the clock, in Redis, so that a
// restart of the service does not start the count again.
const lookupsPerHour = 60

const lookupsPrefix = "phone_lookups:"

// lookupCounter is what this needs of the key-value store.
type lookupCounter interface {
	IncrCtx(ctx context.Context, key string) (int64, error)
	ExpireCtx(ctx context.Context, key string, seconds int) error
}

// lookupAllowed counts one more lookup for this account and answers whether it
// may go ahead; when it may not, how many seconds until the next hour. A store
// that cannot be reached lets the lookup through and says so: the limit is a
// guard against enumeration, not a reason to refuse everybody.
func lookupAllowed(ctx context.Context, store lookupCounter, userId int64, now time.Time) (bool, int32, error) {
	if store == nil {
		return true, 0, errors.New("no key-value store to count lookups in")
	}
	hour := now.Unix() / 3600
	key := fmt.Sprintf("%s%d:%d", lookupsPrefix, userId, hour)
	n, err := store.IncrCtx(ctx, key)
	if err != nil {
		return true, 0, err
	}
	if n == 1 {
		// The key lives to the end of its hour and a little past, then goes.
		_ = store.ExpireCtx(ctx, key, 3600+60)
	}
	if n > lookupsPerHour {
		left := (hour+1)*3600 - now.Unix()
		if left < 1 {
			left = 1
		}
		return false, int32(left), nil
	}
	return true, 0, nil
}
