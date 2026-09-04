package pushnotify

import (
	"context"
	"os"
	"testing"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/pkg/devices"
)

// Reading from the database is not covered anywhere else: the scenarios verify
// that a token is written, while reading it back is done by the sending code,
// which never runs without an Apple key. Column types drift away from the Go
// struct easily and such drift shows up only in production.
//
// The test needs a running stand (deploy/docker-compose.yml). Without one it is
// silently skipped so that builds without a database keep working.
const testDSN = "teamgram:teamgram@tcp(127.0.0.1:3306)/teamgram?charset=utf8mb4&parseTime=true"

func openTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	dsn := testDSN
	if v := os.Getenv("TEST_MYSQL_DSN"); v != "" {
		dsn = v
	}

	db := sqlx.NewMySQL(&sqlx.Config{DSN: dsn, Active: 2, Idle: 2})
	if err := db.QueryRow(context.Background(), new(int), "select 1"); err != nil {
		t.Skipf("database unavailable, test skipped: %v", err)
	}

	return db
}

func TestReadDeviceBack(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	registry := devices.NewRegistry(db)

	const userId = -424242 // a deliberately non-existent person, so real data stays untouched
	want := devices.DeviceDO{
		AuthKeyId:  -1,
		UserId:     userId,
		TokenType:  devices.TokenTypeAPNs,
		Token:      "read-back check",
		NoMuted:    true,
		AppSandbox: true,
		Secret:     "00ff",
		OtherUids:  "1,2",
	}

	signIn(t, db, want.AuthKeyId, userId)
	if err := registry.Register(ctx, &want); err != nil {
		t.Fatalf("cannot store the device: %v", err)
	}
	t.Cleanup(func() {
		_ = registry.Unregister(ctx, want.AuthKeyId, want.UserId, want.TokenType, want.Token)
	})

	list, err := registry.ListByUser(ctx, userId)
	if err != nil {
		t.Fatalf("cannot read the devices back: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one device, got %d", len(list))
	}

	// The same row once the key has signed out: still in the table, and no
	// longer a device anybody holds. Read back, it was a banner on a phone
	// with no account on it (4 September).
	if _, err := db.Exec(ctx, "update auth_users set deleted = 1 where auth_key_id = ? and user_id = ?",
		want.AuthKeyId, userId); err != nil {
		t.Fatalf("cannot sign the key out: %v", err)
	}
	if left, err := registry.ListByUser(ctx, userId); err != nil {
		t.Fatalf("cannot read the devices back after signing out: %v", err)
	} else if len(left) != 0 {
		t.Fatalf("a device whose key signed out is still read back: %d device(s)", len(left))
	}

	got := list[0]
	switch {
	case got.Token != want.Token:
		t.Errorf("token: %q instead of %q", got.Token, want.Token)
	case got.TokenType != want.TokenType:
		t.Errorf("token type: %d instead of %d", got.TokenType, want.TokenType)
	case !got.AppSandbox:
		t.Error("sandbox flag lost: the notification would go to the wrong Apple server")
	case !got.NoMuted:
		t.Error("no_muted flag lost")
	case got.Secret != want.Secret:
		t.Errorf("secret: %q instead of %q", got.Secret, want.Secret)
	case !got.IsAPNs():
		t.Error("the device is not recognised as notifiable")
	}
}

// signIn gives the key the standing of a session: a devices row is read back
// only while auth_users says its key is signed in, so a test that only writes
// the device is testing a phone nobody is signed in on.
func signIn(t *testing.T, db *sqlx.DB, authKeyId, userId int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.Exec(ctx,
		"insert into auth_users(auth_key_id, user_id, hash, date_created, date_active) values (?, ?, ?, 0, 0)",
		authKeyId, userId, authKeyId); err != nil {
		t.Fatalf("cannot sign the key in: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(ctx, "delete from auth_users where auth_key_id = ? and user_id = ?", authKeyId, userId)
	})
}

func TestUnreadCountReadsAsNumber(t *testing.T) {
	db := openTestDB(t)

	// sum() in MySQL returns a decimal rather than an integer: sloppy reading
	// code fails here with a type conversion error.
	n := &Notifier{db: db}
	ctx := context.Background()

	if got := n.unreadCount(ctx, -424242); got != 0 {
		t.Fatalf("expected 0 for a person without conversations, got %d", got)
	}

	// An empty selection returns the zero from coalesce and proves nothing about
	// types. The real check needs a person with unread messages.
	var userId int64
	err := db.QueryRow(ctx, &userId,
		"select user_id from dialogs where unread_count > 0 and deleted = 0 limit 1")
	if err != nil {
		t.Skip("no unread messages in the database, nothing to check the type on")
	}

	if got := n.unreadCount(ctx, userId); got <= 0 {
		t.Fatalf("the person has unread messages, but the count is %d", got)
	}
}

func TestMutedForUnknownPeer(t *testing.T) {
	db := openTestDB(t)

	// No settings means the chat is not muted. The opposite answer would mean
	// notifications silently reach nobody.
	n := &Notifier{db: db}
	if n.muted(context.Background(), -424242, 2, -424243) {
		t.Fatal("a chat without settings is treated as muted")
	}
}
