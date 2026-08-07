package devices

import (
	"context"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/zeromicro/go-zero/core/logx"
)

// Registry — список устройств, на которые пользователю можно отправить
// уведомление. Общий для двух сервисов: notification сюда пишет (приложение
// сообщило токен), sync отсюда читает (кому отправлять, когда человек оффлайн).
type Registry struct {
	db *sqlx.DB
}

func NewRegistry(db *sqlx.DB) *Registry {
	return &Registry{db: db}
}

// Register запоминает токен устройства.
//
// Приложение присылает токен при каждом запуске, а Apple время от времени его
// меняет — поэтому запись обновляется, а не добавляется второй раз.
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

// Unregister убирает устройство: человек вышел из учётной записи или запретил
// уведомления.
func (r *Registry) Unregister(ctx context.Context, authKeyId, userId int64, tokenType int32, token string) error {
	query := "delete from devices where auth_key_id = ? and user_id = ? and token_type = ? and token = ?"

	if _, err := r.db.Exec(ctx, query, authKeyId, userId, tokenType, token); err != nil {
		logx.WithContext(ctx).Errorf("devices.Unregister(%d, %d) - error: %v", userId, tokenType, err)
		return err
	}

	return nil
}

// ListByUser — все устройства пользователя.
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

// Forget убирает токен, который Apple объявила недействительной: приложение
// удалено с устройства. Продолжать слать на такой токен запрещено правилами
// Apple, а нам это ещё и лишняя работа при каждом сообщении.
func (r *Registry) Forget(ctx context.Context, tokenType int32, token string) error {
	query := "delete from devices where token_type = ? and token = ?"

	if _, err := r.db.Exec(ctx, query, tokenType, token); err != nil {
		logx.WithContext(ctx).Errorf("devices.Forget(%d) - error: %v", tokenType, err)
		return err
	}

	return nil
}
