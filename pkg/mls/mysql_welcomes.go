package mls

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/teamgram/marmota/pkg/stores/sqlx"

	"github.com/zeromicro/go-zero/core/logx"
)

// MysqlWelcomes keeps welcomes waiting in the table of the same name.
type MysqlWelcomes struct {
	db *sqlx.DB
}

func NewMysqlWelcomes(db *sqlx.DB) *MysqlWelcomes {
	return &MysqlWelcomes{db: db}
}

type welcomeRow struct {
	Id        int64  `db:"id"`
	UserId    int64  `db:"user_id"`
	AuthKeyId int64  `db:"auth_key_id"`
	FromId    int64  `db:"from_id"`
	PeerId    int64  `db:"peer_id"`
	Welcome   []byte `db:"welcome"`
	Date      int32  `db:"date"`
}

func (s *MysqlWelcomes) Put(ctx context.Context, w Welcome) error {
	const query = "insert into mls_welcomes(user_id, auth_key_id, from_id, peer_id, welcome, date) values (?, ?, ?, ?, ?, ?)"

	if _, err := s.db.Exec(ctx, query, w.UserId, w.AuthKeyId, w.FromId, w.PeerId, w.Bytes, w.Date); err != nil {
		logx.WithContext(ctx).Errorf("mls: cannot leave a welcome: %v", err)
		return err
	}
	return nil
}

func (s *MysqlWelcomes) Waiting(ctx context.Context, userId, authKeyId int64) ([]Welcome, error) {
	var rows []welcomeRow

	err := s.db.QueryRowsPartial(ctx, &rows,
		"select id, user_id, auth_key_id, from_id, peer_id, welcome, date from mls_welcomes "+
			"where user_id = ? and auth_key_id = ? order by id",
		userId, authKeyId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		logx.WithContext(ctx).Errorf("mls: cannot read the waiting welcomes: %v", err)
		return nil, err
	}

	waiting := make([]Welcome, 0, len(rows))
	for _, r := range rows {
		waiting = append(waiting, Welcome{
			Id:        r.Id,
			UserId:    r.UserId,
			AuthKeyId: r.AuthKeyId,
			FromId:    r.FromId,
			PeerId:    r.PeerId,
			Bytes:     r.Welcome,
			Date:      r.Date,
		})
	}
	return waiting, nil
}

// Confirm forgets welcomes, and only this device's own. The user and device are
// in the condition rather than checked beforehand: a device confirming
// somebody else's would drop a conversation that person never joined.
func (s *MysqlWelcomes) Confirm(ctx context.Context, userId, authKeyId int64, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, 0, len(ids)+2)
	args = append(args, userId, authKeyId)
	for _, id := range ids {
		args = append(args, id)
	}

	result, err := s.db.Exec(ctx,
		"delete from mls_welcomes where user_id = ? and auth_key_id = ? and id in ("+placeholders+")",
		args...)
	if err != nil {
		logx.WithContext(ctx).Errorf("mls: cannot confirm welcomes: %v", err)
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

// Devices asks the key package table, because a device is known here exactly
// when it has published something to be reached by.
func (s *MysqlWelcomes) Devices(ctx context.Context, userId int64) ([]int64, error) {
	return (&MysqlStore{db: s.db}).Devices(ctx, userId)
}
