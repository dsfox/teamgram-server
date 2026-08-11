// Mints an invitation: the code a person types on the sign-in screen when there
// is no session of theirs to send one to.
//
// There is no SMS in this service, so this is how a new person gets in, and how
// a familiar one gets back in on a new phone. An invitation works once and
// expires; nothing about it is tied to a number, so it is worth exactly as much
// as the trouble of passing it along.
//
//	invite                             # for somebody new, good for a day
//	invite --phone +79991234567        # for that number only - a lost phone
//	invite --hours 2 --note "Natalya"
//	invite --list                      # what is outstanding
//	invite --recovery --phone +7999...  # a recovery phrase for an account
//
// The recovery phrase is normally handed to a person by the server itself, at
// registration, in their service chat. --recovery is for the accounts that
// predate that, and for somebody who has lost both the phone and the paper. It
// refuses to overwrite a phrase that exists, because that paper is somebody's
// only way back; --force says you know.
//
// A code with no number opens only a phone that has no account yet, so handing
// one out cannot cost somebody their account. Getting back into an account that
// exists needs --phone: it names what the code may open, and nothing else.
//
// It runs inside the container, where the key-value store is reachable by name -
// the same place the alert tool runs.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/teamgram/teamgram-server/pkg/code/invite"

	"github.com/zeromicro/go-zero/core/stores/kv"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func main() {
	hours := flag.Int("hours", 24, "how long the invitation is good for")
	note := flag.String("note", "", "who it is for, for your own records")
	phone := flag.String("phone", "", "the only number this invitation may open")
	anyone := flag.Bool("anyone", false, "mint an invitation for a number you do not know yet")
	list := flag.Bool("list", false, "show outstanding invitations")
	recovery := flag.Bool("recovery", false, "mint this number's recovery phrase instead of an invitation")
	force := flag.Bool("force", false, "with --recovery: replace a phrase that already exists")
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

	if *recovery {
		mintRecovery(ctx, store, *phone, *force)
		return
	}

	if *phone == "" && !*anyone {
		fmt.Fprintln(os.Stderr,
			"which number is this for? --phone +79991234567\n"+
				"Nothing here can prove a number belongs to the person typing it, so an\n"+
				"invitation naming one is what makes it mean anything. For somebody whose\n"+
				"number you do not know yet: --anyone.")
		os.Exit(1)
	}
	if *phone != "" && *anyone {
		fmt.Fprintln(os.Stderr, "--phone and --anyone say opposite things; pick one")
		os.Exit(1)
	}

	code, err := mint(ctx, store, *hours, invite.Invitation{Phone: *phone, Note: *note})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(code)
	if *phone != "" {
		fmt.Fprintf(os.Stderr, "good for %d hour(s), works once, opens %s only\n", *hours, *phone)
	} else {
		fmt.Fprintf(os.Stderr, "good for %d hour(s), works once, for a number with no account\n", *hours)
	}
}

// mintRecovery hands back the account's own way in. Unlike an invitation this
// one does not expire: it is worth nothing until a phone is lost, and that can
// happen at any time.
func mintRecovery(ctx context.Context, store kv.Store, phone string, force bool) {
	if phone == "" {
		fmt.Fprintln(os.Stderr, "--recovery needs --phone: a recovery phrase belongs to one account")
		os.Exit(1)
	}

	if !force && invite.HasRecoveryPhrase(ctx, store, phone) {
		fmt.Fprintf(os.Stderr,
			"%s already has a recovery phrase and somebody has it written down.\n"+
				"Minting another makes that paper worthless. --force if you mean it.\n", phone)
		os.Exit(1)
	}

	phrase, err := invite.MintRecoveryPhrase(ctx, store, phone, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(phrase)
	fmt.Fprintf(os.Stderr,
		"the recovery phrase for %s. Works once, never expires, and cannot be read\n"+
			"back - hand it over now or mint another. The server replaces it with\n"+
			"one of its own the next time this account signs in, so that nobody is\n"+
			"left holding a code that was never passed along.\n", phone)
}

// mint writes an invitation nobody has used yet. The code is as long as every
// other sign-in code, because the clients draw exactly as many boxes as the
// server declares - a shorter one simply cannot be typed in.
func mint(ctx context.Context, store kv.Store, hours int, inv invite.Invitation) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		code := invite.Code()
		key := invite.InvitationKey(code)

		// Taken already: try another rather than overwrite somebody's invitation.
		// A missing key is an empty string, not an error - checking the error is
		// how this loop used to decide every code was taken.
		if existing, err := store.GetCtx(ctx, key); err == nil && existing != "" {
			continue
		}

		if inv.Note == "" {
			inv.Note = "minted " + time.Now().Format(time.RFC3339)
		}
		if err := store.SetexCtx(ctx, key, invite.Encode(inv), hours*3600); err != nil {
			return "", fmt.Errorf("cannot write the invitation: %w", err)
		}

		// Noted, so that --list can find it: the store cannot walk its own keys.
		if err := invite.RememberOutstanding(ctx, store, code); err != nil {
			fmt.Fprintf(os.Stderr, "the invitation was minted but not listed: %v\n", err)
		}
		return code, nil
	}

	return "", fmt.Errorf("could not find a free code after twenty tries")
}

func showOutstanding(ctx context.Context, store kv.Store) {
	// Asked of the list the minting keeps, because the store cannot walk its own
	// keys. What used to be here walked every five-digit code while codes were
	// six digits, so it always answered "no invitations outstanding" - including
	// one second after minting one.
	live := invite.Outstanding(ctx, store)
	for _, item := range live {
		who := item.Invitation.Phone
		if who == "" {
			who = "(anybody new)"
		}
		fmt.Printf("%s  %-16s %s\n", item.Code, who, item.Invitation.Note)
	}
	if len(live) == 0 {
		fmt.Println("no invitations outstanding")
	}
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
