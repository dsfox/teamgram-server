package mls

import (
	"context"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/zeromicro/go-zero/core/logx"
)

// ForgetDevice throws away everything the server holds for a device that has
// signed out: its key packages, and anything left waiting in its boxes.
//
// Without this a device lives for ever in the one place that counts them.
// Somebody who has signed in and out on three phones is still three devices to
// the server, and the phone they actually hold reads that as "there are two
// more of me to let into every conversation" - so it lets in leaves nobody is
// behind, and every conversation carries them from then on (#41).
//
// The welcomes and commits go with them for a plainer reason: they are
// addressed to a device that will never ask again.
//
// With authKeyId zero it is the whole account, which is what unbinding every
// device of a person means.
func ForgetDevice(ctx context.Context, db *sqlx.DB, userId, authKeyId int64) {
	tables := []string{"mls_key_packages", "mls_welcomes", "mls_commits"}
	for _, table := range tables {
		var err error
		if authKeyId == 0 {
			_, err = db.Exec(ctx, "delete from "+table+" where user_id = ?", userId)
		} else {
			_, err = db.Exec(ctx,
				"delete from "+table+" where user_id = ? and auth_key_id = ?", userId, authKeyId)
		}
		if err != nil {
			// Said and not acted on: a signed-out device leaving rows behind is
			// untidy, and refusing the sign-out over it would be worse.
			logx.WithContext(ctx).Errorf("mls: cannot forget %d/%d in %s: %v",
				userId, authKeyId, table, err)
		}
	}
}
