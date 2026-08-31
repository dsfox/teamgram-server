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
	Name        []byte `db:"name"`
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
// ForgetOtherNames throws away this device's packages that belong to an
// identity it no longer has.
//
// The device says which identity it has now; anything else it once published
// is not merely useless but harmful - it can still be claimed, and the
// invitation built from it can never be opened by anybody (#136).
func (s *MysqlStore) ForgetOtherNames(ctx context.Context, userId, authKeyId int64, name []byte) (int, error) {
	const query = "delete from mls_key_packages where user_id = ? and auth_key_id = ? and name <> ?"
	result, err := s.db.Exec(ctx, query, userId, authKeyId, name)
	if err != nil {
		logx.WithContext(ctx).Errorf("mls: cannot forget the packages of an older identity: %v", err)
		return 0, err
	}
	gone, err := result.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return int(gone), nil
}

func (s *MysqlStore) Insert(ctx context.Context, p KeyPackage) (bool, error) {
	const query = "insert into mls_key_packages(user_id, auth_key_id, key_package, fingerprint, name, last_resort, date) " +
		"values (?, ?, ?, ?, ?, ?, ?)"

	_, err := s.db.Exec(ctx, query, p.UserId, p.AuthKeyId, p.Bytes, p.Fingerprint, p.Name, p.LastResort, p.Date)
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
		"select id, user_id, auth_key_id, key_package, fingerprint, name, last_resort, date "+
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
		"select id, user_id, auth_key_id, key_package, fingerprint, name, last_resort, date "+
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

// stillSignedIn keeps a device out of both answers below once its session is
// gone. The rows it leaves behind are not: nothing deletes a device's key
// packages when it is signed out, and there is no one place where that could be
// hooked - a session ends by being reset, by logging out, by being taken away.
// So it is asked at the point of use, which holds however the session ended.
//
// Without it a conversation started with somebody adds a leaf for a phone that
// no longer exists and cannot ever open what is sent to it, and the person's own
// device count includes a phone they no longer have (#137). Measured on the live
// server the day it was written: 227 packages of 5 devices with no session.
//
// Sign-out itself is already tidy: mls.ForgetDevice throws the rows away from
// the reset. What was found on the live server was the state it cannot reach -
// the account behind five devices had been deleted and the session records went
// with it, while 227 key packages stayed. This holds for that, and for whatever
// other way a session ends that nobody has thought of yet.
//
// `deleted = 0` is what the server itself means by a session, everywhere it asks
// (app/service/authsession/.../auth_users_dao.go).
const stillSignedIn = " and exists (select 1 from auth_users a where " +
	"a.auth_key_id = mls_key_packages.auth_key_id and " +
	"a.user_id = mls_key_packages.user_id and a.deleted = 0)" +
	// And seen since, which is the other half of the same question (#138). A
	// device that reinstalls leaves its old row in auth_users exactly as it
	// was - nobody logged it out - so by the record it is alive for ever, and
	// the only thing that tells it apart from a phone in a drawer is that it
	// never asks anything again.
	//
	// Two weeks, because being wrong is cheap in one direction and not in the
	// other. A phone that comes back after longer publishes, is seen, and is
	// let into whatever it missed by the comparison that already runs; a
	// phantom kept is a dead leaf in every conversation started from then on.
	//
	// Taken out and measured without it: the account is told it has two
	// devices when one of them stopped asking a fortnight ago.
	" and exists (select 1 from mls_devices d where " +
	"d.auth_key_id = mls_key_packages.auth_key_id and " +
	"d.user_id = mls_key_packages.user_id and " +
	"d.last_seen > unix_timestamp() - 14 * 86400)"

// CountDevices is how many devices of this person have published anything.
//
// Counted in SQL rather than by taking the list and measuring it. The list is
// read into a slice of plain integers, and asking for it here answered zero for
// a person the same query answers one row for - so the count is asked as a
// count, the way everything else in this file asks for one.
func (s *MysqlStore) CountDevices(ctx context.Context, userId int64) (int, error) {
	var count int

	err := s.db.QueryRowPartial(ctx, &count,
		"select count(distinct auth_key_id) from mls_key_packages where user_id = ?"+
			stillSignedIn,
		userId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		logx.WithContext(ctx).Errorf("mls: cannot count devices: %v", err)
		return 0, err
	}
	logx.WithContext(ctx).Infof("mls: %d has published from %d device(s)", userId, count)
	return count, nil
}

// deviceRow is one device of a person and the identity it publishes under.
type deviceRow struct {
	AuthKeyId int64  `db:"auth_key_id"`
	Name      []byte `db:"name"`
}

// DeviceNames is the leaf name each device of this person publishes under, one
// per device that CountDevices counts.
//
// One row per device rather than per package, because a device publishes many
// and they all carry the same name - and after #136 it is the same name, since
// what an older identity left is thrown away the moment the device says which
// identity it has now.
//
// A name may be empty: a device that published before key packages carried one
// is counted here and cannot be named. It keeps its row, so the caller learns
// that this person has a device it cannot account for rather than being handed
// a shorter list and quietly concluding a leaf is dead (#139).
func (s *MysqlStore) DeviceNames(ctx context.Context, userId int64) ([][]byte, error) {
	var rows []deviceRow

	err := s.db.QueryRowsPartial(ctx, &rows,
		"select auth_key_id, max(name) as name from mls_key_packages where user_id = ?"+
			stillSignedIn+" group by auth_key_id order by auth_key_id",
		userId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		logx.WithContext(ctx).Errorf("mls: cannot list the device names: %v", err)
		return nil, err
	}

	names := make([][]byte, 0, len(rows))
	for _, r := range rows {
		if r.Name == nil {
			r.Name = []byte{}
		}
		names = append(names, r.Name)
	}
	return names, nil
}

func (s *MysqlStore) Devices(ctx context.Context, userId int64) ([]int64, error) {
	var devices []int64

	err := s.db.QueryRowsPartial(ctx, &devices,
		"select distinct auth_key_id from mls_key_packages where user_id = ?"+
			stillSignedIn, userId)
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

// Seen writes down that this device asked the server something, which is what
// tells a phone that is merely off from one that is never coming back (#138).
// Every round of every device reaches Publish - the question "how many devices
// have I" is a publish with nothing in it - so this is where it goes.
func (s *MysqlStore) Seen(ctx context.Context, userId, authKeyId int64, now int32) error {
	_, err := s.db.Exec(ctx,
		"insert into mls_devices(user_id, auth_key_id, last_seen) values (?, ?, ?) "+
			"on duplicate key update last_seen = values(last_seen)",
		userId, authKeyId, now)
	if err != nil {
		logx.WithContext(ctx).Errorf("mls: cannot note that %d/%d was seen: %v",
			userId, authKeyId, err)
	}
	return err
}
