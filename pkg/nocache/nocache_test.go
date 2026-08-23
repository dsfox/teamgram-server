package nocache

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/teamgram/marmota/pkg/stores/sqlc"
	"github.com/teamgram/marmota/pkg/stores/sqlx"
)

// TestItFitsTheSeam is the one that matters: the eight call sites replace
// sqlc.NewConn(db, c.Cache) with this, and the fifty-five generated DAOs go on
// holding the same sqlc.CachedConn. If the interface ever drifts, this stops
// compiling here rather than in every one of them.
func TestItFitsTheSeam(t *testing.T) {
	var db *sqlx.DB
	conn := sqlc.NewConnWithCache(db, New())
	_ = conn
}

func TestEveryReadIsAMiss(t *testing.T) {
	c := New()

	var v string
	err := c.GetCtx(context.Background(), "a key", &v)
	if !c.IsNotFound(err) {
		t.Fatalf("a read answered %v, want a miss", err)
	}
	if v != "" {
		t.Fatalf("a miss wrote %q into the value", v)
	}
}

func TestAWriteChangesNothing(t *testing.T) {
	c := New()
	ctx := context.Background()

	if err := c.SetCtx(ctx, "a key", "a value"); err != nil {
		t.Fatalf("Set answered %v", err)
	}

	var v string
	if err := c.GetCtx(ctx, "a key", &v); !c.IsNotFound(err) {
		t.Fatalf("the value came back after a Set: %v %q", err, v)
	}
}

func TestTheQueryRunsEveryTime(t *testing.T) {
	c := New()
	ctx := context.Background()

	asked := 0
	query := func(val any) error {
		asked++
		*(val.(*int)) = asked
		return nil
	}

	var got int
	for i := 1; i <= 3; i++ {
		if err := c.TakeCtx(ctx, &got, "a key", query); err != nil {
			t.Fatalf("Take answered %v", err)
		}
		if got != i {
			t.Fatalf("read %d answered %d, want the query's %d - it was cached", i, got, i)
		}
	}
}

// A query that finds nothing must say so in the words the callers test for.
// The generated DAOs compare against sqlc.ErrNotFound, which is sql.ErrNoRows.
func TestNotFoundReachesTheCaller(t *testing.T) {
	c := New()

	err := c.TakeCtx(context.Background(), new(int), "a key", func(val any) error {
		return sqlx.ErrNotFound
	})
	if !errors.Is(err, sql.ErrNoRows) || !errors.Is(err, sqlc.ErrNotFound) {
		t.Fatalf("a query that found nothing answered %v", err)
	}
	if !c.IsNotFound(err) {
		t.Fatalf("IsNotFound did not recognise %v", err)
	}
}

func TestTheQueryErrorReachesTheCaller(t *testing.T) {
	c := New()
	boom := errors.New("the database is down")

	err := c.TakeCtx(context.Background(), new(int), "a key", func(val any) error {
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Take swallowed the error, answered %v", err)
	}
}

func TestEveryKeyIsAskedFor(t *testing.T) {
	c := New()
	ctx := context.Background()
	want := []string{"one", "two", "three"}

	var asked []string
	err := c.TakesCtx(ctx,
		func(keys ...string) (map[string]any, error) {
			asked = append(asked, keys...)
			return map[string]any{"one": 1}, nil
		},
		func(k, v string) (any, error) {
			t.Fatal("cacheF was called, so something answered from the cache")
			return nil, nil
		},
		want...)
	if err != nil {
		t.Fatalf("Takes answered %v", err)
	}
	if len(asked) != len(want) {
		t.Fatalf("asked for %v, want all of %v", asked, want)
	}
	for i, k := range want {
		if asked[i] != k {
			t.Fatalf("asked for %v, want all of %v", asked, want)
		}
	}
}

// A query is a round trip to the database. No keys, no trip.
func TestNoKeysNoQuery(t *testing.T) {
	c := New()
	ctx := context.Background()

	query := func(keys ...string) (map[string]any, error) {
		t.Fatal("the database was asked for nothing at all")
		return nil, nil
	}
	if err := c.TakesCtx(ctx, query, nil); err != nil {
		t.Fatalf("Takes answered %v", err)
	}

	queryE := func(expire time.Duration, keys ...string) (map[string]any, error) {
		t.Fatal("the database was asked for nothing at all")
		return nil, nil
	}
	if err := c.TakesWithExpireCtx(ctx, queryE, nil); err != nil {
		t.Fatalf("TakesWithExpire answered %v", err)
	}
}

func TestTakesReportsTheQueryError(t *testing.T) {
	c := New()
	boom := errors.New("the database is down")

	err := c.TakesCtx(context.Background(),
		func(keys ...string) (map[string]any, error) { return nil, boom },
		nil, "a key")
	if !errors.Is(err, boom) {
		t.Fatalf("Takes swallowed the error, answered %v", err)
	}
}
