// Mints an invitation: the code a person types on the sign-in screen when there
// is no session of theirs to send one to.
//
// There is no SMS in this service, so this is how a new person gets in, and how
// a familiar one gets back in on a new phone. An invitation works once and
// expires; nothing about it is tied to a number, so it is worth exactly as much
// as the trouble of passing it along.
//
//	invite                       # a code, good for a day
//	invite --hours 2 --note "Natalya, new phone"
//	invite --list                # what is outstanding
//
// It runs inside the container, where the key-value store is reachable by name -
// the same place the alert tool runs.
package main

import (
	"context"
	cryptorand "crypto/rand"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/teamgram/teamgram-server/pkg/code/invite"

	"github.com/zeromicro/go-zero/core/stores/kv"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func main() {
	hours := flag.Int("hours", 24, "how long the invitation is good for")
	note := flag.String("note", "", "who it is for, for your own records")
	list := flag.Bool("list", false, "show outstanding invitations")
	flag.Parse()

	store := kv.NewStore(kv.KvConf{{
		RedisConf: redis.RedisConf{
			Host: envOr("REDIS_HOST", "redis:6379"),
			Type: "node",
			Pass: os.Getenv("REDIS_PASS"),
		},
		Weight: 100,
	}})

	ctx := context.Background()

	if *list {
		showOutstanding(ctx, store)
		return
	}

	code, err := mint(ctx, store, *hours, *note)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(code)
	fmt.Fprintf(os.Stderr, "good for %d hour(s), works once\n", *hours)
}

// mint writes an invitation nobody has used yet. The code is five digits
// because that is the size of the field both clients draw.
func mint(ctx context.Context, store kv.Store, hours int, note string) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		code := fiveDigits()
		key := invite.InvitationKey(code)

		// Taken already: try another rather than overwrite somebody's invitation.
		if _, err := store.GetCtx(ctx, key); err == nil {
			continue
		}

		value := note
		if value == "" {
			value = "minted " + time.Now().Format(time.RFC3339)
		}
		if err := store.SetexCtx(ctx, key, value, hours*3600); err != nil {
			return "", fmt.Errorf("cannot write the invitation: %w", err)
		}
		return code, nil
	}

	return "", fmt.Errorf("could not find a free code after twenty tries")
}

func showOutstanding(ctx context.Context, store kv.Store) {
	// The store has no listing of its own here, so this walks the space the
	// codes live in. Five digits is a hundred thousand keys, which is nothing to
	// ask about one at a time only when somebody wants the list.
	found := 0
	for n := 0; n < 100000; n++ {
		code := fmt.Sprintf("%05d", n)
		value, err := store.GetCtx(ctx, invite.InvitationKey(code))
		if err != nil || value == "" {
			continue
		}
		fmt.Printf("%s  %s\n", code, value)
		found++
	}
	if found == 0 {
		fmt.Println("no invitations outstanding")
	}
}

// fiveDigits is a code with no pattern to it: crypto/rand, because guessing is
// the attack this has to survive.
func fiveDigits() string {
	const digits = "0123456789"
	b := make([]byte, 5)
	if _, err := randRead(b); err != nil {
		// Falling back to something predictable would defeat the point.
		panic(fmt.Sprintf("no randomness available: %v", err))
	}
	out := strings.Builder{}
	for _, v := range b {
		out.WriteByte(digits[int(v)%len(digits)])
	}
	return out.String()
}

func randRead(b []byte) (int, error) {
	return cryptorand.Read(b)
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
