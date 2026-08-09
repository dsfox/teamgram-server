package mls

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/teamgram/marmota/pkg/stores/sqlx"

	"github.com/zeromicro/go-zero/core/logx"
)

// MysqlStore keeps key packages in the table of the same name.
//
// Written by hand rather than generated, because the two operations that matter
// are not ordinary rows-in-a-table: taking a package has to remove it in the
// same breath it reads it, or two conversations starting at once take the same
// one; and a duplicate has to be a quiet no rather than an error, so that a
// client retrying after a lost answer is not punished for it.
type MysqlStore struct {
	db *sqlx.DB
}

func NewMysqlStore(db *sqlx.DB) *MysqlStore {
	return &MysqlStore{db: db}
}

type keyPackageRow struct {
	Id          int64  `db:"id"`
	UserId      int64  `db:"user_id"`
	AuthKeyId   int64  `db:"auth_key_id"`
	KeyPackage  []byte `db:"key_package"`
	Fingerprint string `db:"fingerprint"`
	LastResort  bool   `db:"last_resort"`
	Date        int32  `db:"date"`
}

func (r keyPackageRow) toKeyPackage() KeyPackage {
	return KeyPackage{
		UserId:      r.UserId,
		AuthKeyId:   r.AuthKeyId,
		Bytes:       r.KeyPackage,
		Fingerprint: r.Fingerprint,
		LastResort:  r.LastResort,
		Date:        r.Date,
	}
}

// Insert adds a package, and says false rather than failing when the same bytes
// are already there. The unique key does the deciding: asking first and then
// inserting would let two requests both decide there was room.
func (s *MysqlStore) Insert(ctx context.Context, p KeyPackage) (bool, error) {
	const query = "insert into mls_key_packages(user_id, auth_key_id, key_package, fingerprint, last_resort, date) " +
		"values (?, ?, ?, ?, ?, ?)"

	_, err := s.db.Exec(ctx, query, p.UserId, p.AuthKeyId, p.Bytes, p.Fingerprint, p.LastResort, p.Date)
	if err != nil {
		if isDuplicate(err) {
			return false, nil
		}
		logx.WithContext(ctx).Errorf("mls: cannot insert a key package: %v", err)
		return false, err
	}
	return true, nil
}

// TakeOne removes and returns the oldest ordinary package for a device.
//
// The delete is what claims it. Reading first and deleting afterwards would let
// two conversations starting at the same moment take the same package, and a
// package used twice costs the forward secrecy of everything after it.
func (s *MysqlStore) TakeOne(ctx context.Context, userId, authKeyId int64) (*KeyPackage, error) {
	var row keyPackageRow

	err := s.db.QueryRowPartial(ctx, &row,
		"select id, user_id, auth_key_id, key_package, fingerprint, last_resort, date "+
			"from mls_key_packages where user_id = ? and auth_key_id = ? and last_resort = 0 "+
			"order by id limit 1",
		userId, authKeyId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		logx.WithContext(ctx).Errorf("mls: cannot read a key package: %v", err)
		return nil, err
	}

	result, err := s.db.Exec(ctx, "delete from mls_key_packages where id = ?", row.Id)
	if err != nil {
		logx.WithContext(ctx).Errorf("mls: cannot claim a key package: %v", err)
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		// Somebody else took it between the read and the delete. Theirs, then;
		// the caller asks again rather than getting a package twice.
		return s.TakeOne(ctx, userId, authKeyId)
	}

	p := row.toKeyPackage()
	return &p, nil
}

func (s *MysqlStore) LastResort(ctx context.Context, userId, authKeyId int64) (*KeyPackage, error) {
	var row keyPackageRow

	err := s.db.QueryRowPartial(ctx, &row,
		"select id, user_id, auth_key_id, key_package, fingerprint, last_resort, date "+
			"from mls_key_packages where user_id = ? and auth_key_id = ? and last_resort = 1 "+
			"order by id desc limit 1",
		userId, authKeyId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		logx.WithContext(ctx).Errorf("mls: cannot read the last-resort package: %v", err)
		return nil, err
	}

	p := row.toKeyPackage()
	return &p, nil
}

func (s *MysqlStore) CountAvailable(ctx context.Context, userId, authKeyId int64) (int, error) {
	var count int

	err := s.db.QueryRowPartial(ctx, &count,
		"select count(*) from mls_key_packages where user_id = ? and auth_key_id = ? and last_resort = 0",
		userId, authKeyId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		logx.WithContext(ctx).Errorf("mls: cannot count key packages: %v", err)
		return 0, err
	}
	return count, nil
}

func (s *MysqlStore) Devices(ctx context.Context, userId int64) ([]int64, error) {
	var devices []int64

	err := s.db.QueryRowsPartial(ctx, &devices,
		"select distinct auth_key_id from mls_key_packages where user_id = ?", userId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		logx.WithContext(ctx).Errorf("mls: cannot list devices: %v", err)
		return nil, err
	}
	return devices, nil
}

// isDuplicate recognises the unique key refusing the same bytes twice. Matched
// on the text because the driver is behind two wrappers here; the alternative
// is asking first, which is the race this exists to avoid.
func isDuplicate(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate entry")
}
