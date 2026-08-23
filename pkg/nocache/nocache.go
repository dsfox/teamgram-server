// Package nocache is the row cache with no Redis behind it.
//
// The generated DAOs hold a sqlc.CachedConn and reach through it for every
// read, and building one the usual way - sqlc.NewConn(db, c.Cache) - requires
// a Redis to point at. But there is a seam beside it:
//
//	func NewConnWithCache(db *sqlx.DB, c cache.BatchCache) CachedConn
//
// cache.BatchCache is an interface, so a connection can be given a cache of
// our own and every one of the fifty-five generated DAOs goes on compiling
// untouched. This is that cache: every read a miss, every write ignored. The
// query behind it runs each time, which for a messenger this size is a
// database that is not the bottleneck - and a cache that never lies is worth
// more than one that saves a query. See ice9 #7.
//
// The interface is marmota's, not go-zero's: the eight call sites import
// github.com/teamgram/marmota/pkg/stores/sqlc, whose CachedConn holds a
// BatchCache - go-zero's Cache plus the four Takes methods for reading many
// keys at once. BatchCache embeds Cache, so one type answers both.
//
// If a hot path ever appears, it is answered here rather than anywhere else:
// the same interface takes an in-process TTL cache (core/collection.Cache) and
// nothing outside this file has to know.
package nocache

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/teamgram/marmota/pkg/stores/cache"
)

// Always is a cache that holds nothing.
type Always struct{}

// New returns a cache that answers every read with "not there".
func New() cache.BatchCache {
	return Always{}
}

// errNotFound is what a miss looks like. sql.ErrNoRows, because that is the
// error marmota hands its own cache to report one - see the NewConn above the
// seam we use - and the same error sqlx.ErrNotFound is an alias of, so callers
// testing either one still recognise it.
var errNotFound = sql.ErrNoRows

func (Always) Del(keys ...string) error { return nil }

func (Always) DelCtx(ctx context.Context, keys ...string) error { return nil }

func (Always) Get(key string, val any) error { return errNotFound }

func (Always) GetCtx(ctx context.Context, key string, val any) error { return errNotFound }

func (Always) IsNotFound(err error) bool { return errors.Is(err, errNotFound) }

func (Always) Set(key string, val any) error { return nil }

func (Always) SetCtx(ctx context.Context, key string, val any) error { return nil }

func (Always) SetWithExpire(key string, val any, expire time.Duration) error { return nil }

func (Always) SetWithExpireCtx(ctx context.Context, key string, val any, expire time.Duration) error {
	return nil
}

// Take goes straight to the query, which is the whole point.
func (a Always) Take(val any, key string, query func(val any) error) error {
	return query(val)
}

func (a Always) TakeCtx(ctx context.Context, val any, key string, query func(val any) error) error {
	return query(val)
}

// TakeWithExpire asks for no expiry, since nothing is kept. The queries built
// on top of this take the duration to hand to a Set they will never reach.
func (a Always) TakeWithExpire(val any, key string, query func(val any, expire time.Duration) error) error {
	return query(val, 0)
}

func (a Always) TakeWithExpireCtx(ctx context.Context, val any, key string,
	query func(val any, expire time.Duration) error) error {
	return query(val, 0)
}

// Takes asks for every key, because none of them is here.
//
// cacheF turns a cached string back into a value and so is never called; the
// map the query returns is what a real cache would write, and is dropped. The
// rows themselves reach the caller the way they always did, appended by the
// query's own closure as it reads them.
func (a Always) Takes(query func(keys ...string) (map[string]any, error),
	cacheF func(k, v string) (any, error), keys ...string) error {
	return a.TakesCtx(context.Background(), query, cacheF, keys...)
}

func (a Always) TakesCtx(ctx context.Context, query func(keys ...string) (map[string]any, error),
	cacheF func(k, v string) (any, error), keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	_, err := query(keys...)
	return err
}

func (a Always) TakesWithExpire(query func(expire time.Duration, keys ...string) (map[string]any, error),
	cacheF func(k, v string) (any, error), keys ...string) error {
	return a.TakesWithExpireCtx(context.Background(), query, cacheF, keys...)
}

func (a Always) TakesWithExpireCtx(ctx context.Context,
	query func(expire time.Duration, keys ...string) (map[string]any, error),
	cacheF func(k, v string) (any, error), keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	_, err := query(0, keys...)
	return err
}
