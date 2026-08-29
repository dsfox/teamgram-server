package mls

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/teamgram/marmota/pkg/stores/sqlx"

	"github.com/zeromicro/go-zero/core/logx"
)

// MysqlGroups remembers which epoch each conversation is on.
type MysqlGroups struct {
	db *sqlx.DB
}

func NewMysqlGroups(db *sqlx.DB) *MysqlGroups {
	return &MysqlGroups{db: db}
}

type groupRow struct {
	Epoch int64 `db:"epoch"`
}

func (s *MysqlGroups) Epoch(ctx context.Context, groupId []byte) (int64, bool, error) {
	var row groupRow
	err := s.db.QueryRowPartial(ctx, &row,
		"select epoch from mls_groups where group_id = ?", groupId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		logx.WithContext(ctx).Errorf("mls: cannot read the epoch: %v", err)
		return 0, false, err
	}
	return row.Epoch, true, nil
}

// Take moves a conversation from `from` to `from+1` and leaves the commit for
// every device named - both, or neither.
//
// The database decides who moved it, not the caller: two clients arriving with
// the same `from` must not both be told yes, and no amount of reading
// beforehand can promise that - between the read and the write is exactly where
// the other one gets in. So the epoch is in the WHERE, and the row count is the
// answer.
//
// A conversation nobody has mentioned yet is inserted. If two do that at once
// one of them collides on the primary key and is told no, which is the same
// answer for the same reason.
//
// One transaction around both halves, and that is the point of the method
// existing at all. Moving the epoch first and handing out the copies afterwards
// left two ways to hurt somebody: a device refused in between was told the group
// had moved and could not yet fetch what moved it, and a delivery that failed
// halfway left the group ahead with some of its members permanently behind -
// they need the commit they never got before anything after it will apply
// (#120).
func (s *MysqlGroups) Take(ctx context.Context, groupId []byte, from int64, deliveries []Commit) (bool, error) {
	const leave = "insert into mls_commits(user_id, auth_key_id, from_id, group_id, epoch, commit_bytes, date) " +
		"values (?, ?, ?, ?, ?, ?, ?)"

	moved := false
	err := s.db.Transact(ctx, func(tx *sqlx.Tx) error {
		result, err := tx.Exec(
			"update mls_groups set epoch = ? where group_id = ? and epoch = ?",
			from+1, groupId, from)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}

		if affected != 1 {
			// Nothing moved: either this group is new, or somebody else moved
			// it first. Inserting tells the two apart - a duplicate key is the
			// second.
			if _, err = tx.Exec(
				"insert into mls_groups(group_id, epoch) values (?, ?)", groupId, from+1); err != nil {
				if strings.Contains(err.Error(), "Duplicate entry") ||
					strings.Contains(err.Error(), "1062") {
					return nil // moved stays false: somebody else was first.
				}
				return err
			}
		}

		for _, c := range deliveries {
			if _, err = tx.Exec(leave,
				c.UserId, c.AuthKeyId, c.FromId, c.GroupId, c.Epoch, c.Bytes, c.Date); err != nil {
				return err
			}
		}
		moved = true
		return nil
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("mls: cannot take the commit: %v", err)
		return false, err
	}
	return moved, nil
}

// MysqlCommits keeps commits waiting in the table of the same name.
type MysqlCommits struct {
	db *sqlx.DB
}

func NewMysqlCommits(db *sqlx.DB) *MysqlCommits {
	return &MysqlCommits{db: db}
}

type commitRow struct {
	Id          int64  `db:"id"`
	UserId      int64  `db:"user_id"`
	AuthKeyId   int64  `db:"auth_key_id"`
	FromId      int64  `db:"from_id"`
	GroupId     []byte `db:"group_id"`
	Epoch       int64  `db:"epoch"`
	CommitBytes []byte `db:"commit_bytes"`
	Date        int32  `db:"date"`
}

// Waiting is what a device has not applied yet, oldest first.
//
// The order is not a nicety: a commit applies only to the epoch it was made
// from, so out of order every one but the first fails.
func (s *MysqlCommits) Waiting(ctx context.Context, userId, authKeyId int64) ([]Commit, error) {
	var rows []commitRow

	err := s.db.QueryRowsPartial(ctx, &rows,
		"select id, user_id, auth_key_id, from_id, group_id, epoch, commit_bytes, date "+
			"from mls_commits where user_id = ? and auth_key_id = ? order by id",
		userId, authKeyId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		logx.WithContext(ctx).Errorf("mls: cannot read the waiting commits: %v", err)
		return nil, err
	}

	waiting := make([]Commit, 0, len(rows))
	for _, r := range rows {
		waiting = append(waiting, Commit{
			Id:        r.Id,
			UserId:    r.UserId,
			AuthKeyId: r.AuthKeyId,
			FromId:    r.FromId,
			GroupId:   r.GroupId,
			Epoch:     r.Epoch,
			Bytes:     r.CommitBytes,
			Date:      r.Date,
		})
	}
	return waiting, nil
}

// Confirm forgets commits a device says it has applied, and only its own.
func (s *MysqlCommits) Confirm(ctx context.Context, userId, authKeyId int64, ids []int64) (int, error) {
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
		"delete from mls_commits where user_id = ? and auth_key_id = ? and id in ("+placeholders+")",
		args...)
	if err != nil {
		logx.WithContext(ctx).Errorf("mls: cannot confirm commits: %v", err)
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

// Devices asks the key package table, for the same reason welcomes do: a device
// is reachable here exactly when it has published something to be reached by.
func (s *MysqlCommits) Devices(ctx context.Context, userId int64) ([]int64, error) {
	return (&MysqlStore{db: s.db}).Devices(ctx, userId)
}
