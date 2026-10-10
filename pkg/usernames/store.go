package usernames

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
)

// What a counter may go up to before the store gives up on a name: far past
// anything a real server holds, and short of looping for ever on a fault.
const lastCounter = 1_000_000

// Store hands out usernames and remembers which ones it handed out.
//
// The `username` table is what decides who holds a name - its unique key is
// case-insensitive, so two sign-ups at once cannot take the same one - and
// `users.username` is what clients are shown. Both are written, as
// account.updateUsername writes them.
type Store struct {
	db *sqlx.DB
	// Names something else answers to before any account: the server's
	// built-in bots resolve ahead of the username table.
	reserved func(name string) bool
}

func NewStore(db *sqlx.DB, reserved func(name string) bool) *Store {
	if reserved == nil {
		reserved = func(string) bool { return false }
	}
	return &Store{db: db, reserved: reserved}
}

// Assign gives this account the first free username its name makes, marks it
// as given rather than chosen, and answers with it.
func (s *Store) Assign(ctx context.Context, userID int64, first, last string) (string, error) {
	for _, name := range Steps(first, last) {
		taken, err := s.take(ctx, userID, name)
		if err != nil {
			return "", err
		}
		if taken {
			return name, s.mark(ctx, userID, name)
		}
	}

	// Every step is held: a counter after the last one. The names already
	// held under it are asked for once rather than tried one by one - a stand
	// holds thousands of Alices - and a sign-up that takes the free one in
	// between is met by the unique key, and the next is tried.
	base := Base(first, last)
	for attempt := 0; attempt < 5; attempt++ {
		held, err := s.held(ctx, base)
		if err != nil {
			return "", err
		}
		for n := 1; n <= lastCounter; n++ {
			name := Counted(base, n)
			if name == "" || held[name] {
				continue
			}
			taken, err := s.take(ctx, userID, name)
			if err != nil {
				return "", err
			}
			if taken {
				return name, s.mark(ctx, userID, name)
			}
			break
		}
	}
	return "", fmt.Errorf("usernames: no free username for user %d after %q", userID, base)
}

// IsGenerated says whether this account's username is the one handed out
// here, rather than one the person chose. Keyed on the pair: a name given
// out, then freed and chosen by somebody else, is theirs.
func (s *Store) IsGenerated(ctx context.Context, userID int64, name string) (bool, error) {
	var rows []UsernameGeneratedDO
	err := s.db.QueryRowsPartial(ctx, &rows,
		"select user_id, username, created_at from username_generated where user_id = ? and username = ?",
		userID, name)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return false, err
	}
	return len(rows) > 0, nil
}

// Forget drops what was handed out to this account: it chose a username of
// its own, or it is gone.
func (s *Store) Forget(ctx context.Context, userID int64) error {
	_, err := s.db.Exec(ctx, "delete from username_generated where user_id = ?", userID)
	return err
}

func (s *Store) take(ctx context.Context, userID int64, name string) (bool, error) {
	if s.reserved(name) {
		return false, nil
	}
	_, err := s.db.Exec(ctx,
		"insert into username(username, peer_type, peer_id, editable, active, order2, deleted) values (?, 2, ?, 1, 1, ?, 0)",
		name, userID, time.Now().Unix()<<32)
	if err != nil {
		if sqlx.IsDuplicate(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Store) mark(ctx context.Context, userID int64, name string) error {
	if _, err := s.db.Exec(ctx,
		"insert into username_generated(user_id, username, created_at) values (?, ?, ?)",
		userID, name, time.Now().Unix()); err != nil && !sqlx.IsDuplicate(err) {
		return err
	}
	_, err := s.db.Exec(ctx, "update users set username = ? where id = ?", name, userID)
	return err
}

func (s *Store) held(ctx context.Context, base string) (map[string]bool, error) {
	var names []string
	err := s.db.QueryRowsPartial(ctx, &names, "select username from username where username like ?", base+"%")
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, err
	}
	held := make(map[string]bool, len(names))
	for _, name := range names {
		held[strings.ToLower(name)] = true
	}
	return held, nil
}
