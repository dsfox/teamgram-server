package devices

import (
	"context"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/zeromicro/go-zero/core/logx"
)

// Registry is the list of devices a person can be notified on. Shared by two
// services: notification writes here (the app reported a token) and sync reads
// from here (whom to notify while the person is offline).
type Registry struct {
	db *sqlx.DB
}

func NewRegistry(db *sqlx.DB) *Registry {
	return &Registry{db: db}
}

// Register remembers a device token.
//
// The app reports the token on every launch and Apple rotates it now and then,
// so the row is updated instead of being inserted a second time.
func (r *Registry) Register(ctx context.Context, d *DeviceDO) error {
	query := "insert into devices(auth_key_id, user_id, token_type, token, no_muted, app_sandbox, secret, other_uids) " +
		"values (:auth_key_id, :user_id, :token_type, :token, :no_muted, :app_sandbox, :secret, :other_uids) " +
		"on duplicate key update token = values(token), no_muted = values(no_muted), " +
		"app_sandbox = values(app_sandbox), secret = values(secret), other_uids = values(other_uids), state = 0"

	if _, err := r.db.NamedExec(ctx, query, d); err != nil {
		logx.WithContext(ctx).Errorf("devices.Register(%d, %d) - error: %v", d.UserId, d.TokenType, err)
		return err
	}

	return nil
}

// Unregister drops a device: the person signed out or disabled notifications.
func (r *Registry) Unregister(ctx context.Context, authKeyId, userId int64, tokenType int32, token string) error {
	query := "delete from devices where auth_key_id = ? and user_id = ? and token_type = ? and token = ?"

	if _, err := r.db.Exec(ctx, query, authKeyId, userId, tokenType, token); err != nil {
		logx.WithContext(ctx).Errorf("devices.Unregister(%d, %d) - error: %v", userId, tokenType, err)
		return err
	}

	return nil
}

// ListByUser returns every device of the user.
func (r *Registry) ListByUser(ctx context.Context, userId int64) ([]DeviceDO, error) {
	var list []DeviceDO
	query := "select id, auth_key_id, user_id, token_type, token, no_muted, locked_period, app_sandbox, secret, other_uids, state " +
		"from devices where user_id = ?"

	if err := r.db.QueryRowsPartial(ctx, &list, query, userId); err != nil {
		logx.WithContext(ctx).Errorf("devices.ListByUser(%d) - error: %v", userId, err)
		return nil, err
	}

	return list, nil
}

// Forget drops a token Apple declared invalid: the app was removed from the
// device. Apple's rules forbid sending to such a token, and for us it is wasted
// work on every message.
func (r *Registry) Forget(ctx context.Context, tokenType int32, token string) error {
	query := "delete from devices where token_type = ? and token = ?"

	if _, err := r.db.Exec(ctx, query, tokenType, token); err != nil {
		logx.WithContext(ctx).Errorf("devices.Forget(%d) - error: %v", tokenType, err)
		return err
	}

	return nil
}
