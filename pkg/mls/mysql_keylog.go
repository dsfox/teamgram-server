package mls

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/mod/sumdb/tlog"
)

// MysqlKeyLog keeps the key log (#44) in three tables: the entries, the tree's
// stored hashes (tlog's layout, one row per hash), and one row holding the
// size and the log key's seed. The size row is what serialises appends: an
// append locks it, so two devices publishing at once are numbered one after
// the other and neither overwrites a hash of the other's.
type MysqlKeyLog struct {
	db *sqlx.DB
}

func NewMysqlKeyLog(db *sqlx.DB) *MysqlKeyLog {
	return &MysqlKeyLog{db: db}
}

type keyLogStateRow struct {
	Size   int64  `db:"size"`
	LogKey []byte `db:"log_key"`
}

const keyLogStateQuery = "select size, log_key from mls_key_log_state where id = 1"

// LogKey reads the seed, making it on first use. Made here rather than in the
// SQL patch so a database put together any other way still has one; insert
// ignore decides between two processes making it at once.
func (s *MysqlKeyLog) LogKey(ctx context.Context) ([]byte, error) {
	var row keyLogStateRow
	err := s.db.QueryRowPartial(ctx, &row, keyLogStateQuery)
	if err == nil && len(row.LogKey) == ed25519.SeedSize {
		return row.LogKey, nil
	}
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(ctx,
		"insert ignore into mls_key_log_state(id, size, log_key) values (1, 0, ?)", seed); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowPartial(ctx, &row, keyLogStateQuery); err != nil {
		return nil, err
	}
	if len(row.LogKey) != ed25519.SeedSize {
		return nil, fmt.Errorf("the log key is %d bytes", len(row.LogKey))
	}
	logx.WithContext(ctx).Infof("mls: the key log has a key of its own now")
	return row.LogKey, nil
}

func (s *MysqlKeyLog) Size(ctx context.Context) (int64, error) {
	var row keyLogStateRow
	err := s.db.QueryRowPartial(ctx, &row, keyLogStateQuery)
	if errors.Is(err, sqlx.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return row.Size, err
}

type keyLogHashRow struct {
	Idx  int64  `db:"idx"`
	Hash []byte `db:"hash"`
}

// readHashes answers in the order asked, which tlog requires.
func readHashes(q func(v interface{}, query string, args ...interface{}) error, indexes []int64) ([]tlog.Hash, error) {
	if len(indexes) == 0 {
		return nil, nil
	}
	args := make([]interface{}, len(indexes))
	for i, idx := range indexes {
		args[i] = idx
	}
	var rows []keyLogHashRow
	query := "select idx, hash from mls_key_log_hashes where idx in (?" +
		strings.Repeat(", ?", len(indexes)-1) + ")"
	if err := q(&rows, query, args...); err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, err
	}
	byIndex := make(map[int64]tlog.Hash, len(rows))
	for _, r := range rows {
		var h tlog.Hash
		if len(r.Hash) != tlog.HashSize {
			return nil, fmt.Errorf("the stored hash %d is %d bytes", r.Idx, len(r.Hash))
		}
		copy(h[:], r.Hash)
		byIndex[r.Idx] = h
	}
	out := make([]tlog.Hash, len(indexes))
	for i, idx := range indexes {
		h, ok := byIndex[idx]
		if !ok {
			return nil, fmt.Errorf("the stored hash %d is missing", idx)
		}
		out[i] = h
	}
	return out, nil
}

func (s *MysqlKeyLog) ReadHashes(ctx context.Context, indexes []int64) ([]tlog.Hash, error) {
	return readHashes(func(v interface{}, query string, args ...interface{}) error {
		return s.db.QueryRowsPartial(ctx, v, query, args...)
	}, indexes)
}

type keyLogRow struct {
	Idx          int64  `db:"idx"`
	UserId       int64  `db:"user_id"`
	Name         []byte `db:"name"`
	SignatureKey []byte `db:"signature_key"`
	Date         int32  `db:"date"`
}

func (s *MysqlKeyLog) Records(ctx context.Context, userId int64) ([]KeyRecord, error) {
	var rows []keyLogRow
	err := s.db.QueryRowsPartial(ctx, &rows,
		"select idx, user_id, name, signature_key, date from mls_key_log where user_id = ? order by idx", userId)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, err
	}
	out := make([]KeyRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, KeyRecord{Index: r.Idx, UserId: r.UserId, Name: r.Name, SignatureKey: r.SignatureKey, Date: r.Date})
	}
	return out, nil
}

func (s *MysqlKeyLog) Append(ctx context.Context, r KeyRecord, hashesFor func(n int64, r tlog.HashReader) ([]tlog.Hash, error)) (bool, error) {
	// The size row has to exist before it can be locked.
	if _, err := s.LogKey(ctx); err != nil {
		return false, err
	}
	appended := false
	err := s.db.Transact(ctx, func(tx *sqlx.Tx) error {
		var state keyLogStateRow
		if err := tx.QueryRowPartial(&state, keyLogStateQuery+" for update"); err != nil {
			return err
		}
		var already int64
		if err := tx.QueryRowPartial(&already,
			"select count(*) from mls_key_log where name = ? and signature_key = ?", r.Name, r.SignatureKey); err != nil {
			return err
		}
		if already > 0 {
			return nil
		}
		n := state.Size
		hashes, err := hashesFor(n, tlog.HashReaderFunc(func(indexes []int64) ([]tlog.Hash, error) {
			return readHashes(tx.QueryRowsPartial, indexes)
		}))
		if err != nil {
			return err
		}
		if _, err := tx.Exec("insert into mls_key_log(idx, user_id, name, signature_key, date) values (?, ?, ?, ?, ?)",
			n, r.UserId, r.Name, r.SignatureKey, r.Date); err != nil {
			return err
		}
		first := tlog.StoredHashIndex(0, n)
		for i, h := range hashes {
			if _, err := tx.Exec("insert into mls_key_log_hashes(idx, hash) values (?, ?)", first+int64(i), h[:]); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("update mls_key_log_state set size = ? where id = 1", n+1); err != nil {
			return err
		}
		appended = true
		return nil
	})
	return appended, err
}

type publishedKeyRow struct {
	UserId     int64  `db:"user_id"`
	KeyPackage []byte `db:"key_package"`
}

// Backfill logs one package of every identity in the directory, so the log
// starts from the devices that exist rather than from the next publish. Run
// once at start; Witness skips whatever is logged already.
func (s *MysqlKeyLog) Backfill(ctx context.Context, log *KeyLog, now int32) {
	var rows []publishedKeyRow
	err := s.db.QueryRowsPartial(ctx, &rows,
		"select user_id, key_package from mls_key_packages where id in "+
			"(select max(id) from mls_key_packages where name <> '' group by user_id, auth_key_id, name)")
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		logx.WithContext(ctx).Errorf("mls: cannot read the directory to start the key log from: %v", err)
		return
	}
	logged, refused := 0, 0
	for _, r := range rows {
		if err := log.Witness(ctx, r.UserId, r.KeyPackage, now); err != nil {
			// Counted, not logged one by one: a stand holds thousands of
			// scenario placeholders, and the summary below says how many.
			refused++
			continue
		}
		logged++
	}
	logx.WithContext(ctx).Infof("mls: the key log checked %d identities of the directory, %d not logged", logged, refused)
}
