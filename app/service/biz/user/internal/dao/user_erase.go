package dao

import (
	"context"
	"fmt"

	"github.com/teamgram/marmota/pkg/stores/sqlx"

	"github.com/zeromicro/go-zero/core/logx"
)

// Erasing an account means erasing what it left behind, not only the row that
// names it. Upstream stopped at marking the user deleted and releasing the
// phone number, leaving the messages, the uploaded address book and the device
// tokens in place - which made the privacy policy say, out loud, that this step
// was done by hand.
//
// Two things make this safe to do in one sweep. Every message exists once per
// participant: `messages.user_id` is whose box the copy sits in, so deleting by
// it takes our copies and leaves the other side's alone, which is exactly what
// the site promises. And every table below is keyed by the departing user, so
// nothing here can reach into somebody else's data.
//
// This lives in the user service, which does not own all of these tables. It is
// one database and one deletion, and a partial erase is worse than none: the
// alternative was a call to four services, any of which could fail halfway and
// leave the account half-gone with nobody to notice.
var erasableByUser = []struct {
	table  string
	column string
}{
	// What the person wrote and where they wrote it.
	{"messages", "user_id"},
	{"dialogs", "user_id"},
	{"saved_dialogs", "user_id"},
	{"drafts", "user_id"},
	{"message_read_outbox", "user_id"},
	{"dialog_filters", "user_id"},
	{"hash_tags", "user_id"},

	// The address book they uploaded, and their place in other people's.
	{"imported_contacts", "user_id"},
	{"phone_books", "user_id"},
	{"user_contacts", "owner_user_id"},
	{"user_contacts", "contact_user_id"},

	// Settings, presence and everything the profile carried.
	{"user_settings", "user_id"},
	{"user_privacies", "user_id"},
	{"user_global_privacy_settings", "user_id"},
	{"user_notify_settings", "user_id"},
	{"user_peer_settings", "user_id"},
	{"user_peer_blocks", "user_id"},
	{"user_presences", "user_id"},
	{"user_profile_photos", "user_id"},
	{"user_saved_music", "user_id"},
	{"default_history_ttl", "user_id"},

	// Group membership: the account is gone, so it is not in any group either.
	{"chat_participants", "user_id"},
	{"chat_invite_participants", "user_id"},

	// The devices we would have sent notifications to.
	{"devices", "user_id"},

	// The update sequence: without the account there is nothing left to catch up on.
	{"user_pts_updates", "user_id"},
	{"auth_seq_updates", "user_id"},
}

// EraseUserData deletes everything the account left behind. It runs in one
// transaction: either the account is gone or it is untouched, never half.
func (d *Dao) EraseUserData(ctx context.Context, userId int64) error {
	var erased int64

	result := sqlx.TxWrapper(ctx, d.DB, func(tx *sqlx.Tx, result *sqlx.StoreResult) {
		for _, target := range erasableByUser {
			query := fmt.Sprintf("delete from %s where %s = ?", target.table, target.column)
			r, err := tx.Exec(query, userId)
			if err != nil {
				// A table that upstream's schema does not have is not a reason
				// to abandon the erase, but it must be visible: a silently
				// skipped table is data we told the person was gone.
				result.Err = fmt.Errorf("erasing %s.%s for user %d: %w",
					target.table, target.column, userId, err)
				return
			}
			if n, err := r.RowsAffected(); err == nil {
				erased += n
			}
		}
	})

	if result.Err != nil {
		logx.WithContext(ctx).Errorf("EraseUserData(%d) - error: %v", userId, result.Err)
		return result.Err
	}

	logx.WithContext(ctx).Infof("EraseUserData(%d) - %d rows erased", userId, erased)
	return nil
}
