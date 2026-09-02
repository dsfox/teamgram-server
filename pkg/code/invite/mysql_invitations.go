package invite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
)

// MysqlInvitations is the record of who brought whom (#47).
type MysqlInvitations struct {
	db *sqlx.DB
}

func NewMysqlInvitations(db *sqlx.DB) *MysqlInvitations {
	return &MysqlInvitations{db: db}
}

// invitationRow is one row of `invitations`, named so the schema gate can read
// its columns.
type invitationRow struct {
	Code       string `db:"code"`
	InviterId  int64  `db:"inviter_id"`
	Phone      string `db:"phone"`
	MintedAt   int32  `db:"minted_at"`
	RedeemedAt int32  `db:"redeemed_at"`
	InviteeId  int64  `db:"invitee_id"`
}

// Minted writes the row the moment a code exists.
func (s *MysqlInvitations) Minted(ctx context.Context, code, phone string, inviterId int64, at int32) error {
	const query = "insert into invitations(code, inviter_id, phone, minted_at) values (?, ?, ?, ?)"
	if _, err := s.db.Exec(ctx, query, code, inviterId, phone, at); err != nil {
		return fmt.Errorf("cannot record the invitation: %w", err)
	}
	return nil
}

// Redeemed marks the row whose code was just spent for this number.
func (s *MysqlInvitations) Redeemed(ctx context.Context, code, phone string, at int32) error {
	const query = "update invitations set redeemed_at = ? " +
		"where code = ? and phone = ? and redeemed_at = 0 order by minted_at desc limit 1"
	if _, err := s.db.Exec(ctx, query, at, code, phone); err != nil {
		return fmt.Errorf("cannot record the redemption: %w", err)
	}
	return nil
}

// Adopted names the account that came in on the most recent redeemed
// invitation for this number - called from signUp, where the id exists.
func (s *MysqlInvitations) Adopted(ctx context.Context, phone string, inviteeId int64) error {
	const query = "update invitations set invitee_id = ? " +
		"where phone = ? and redeemed_at > 0 and invitee_id = 0 order by redeemed_at desc limit 1"
	if _, err := s.db.Exec(ctx, query, inviteeId, phone); err != nil {
		return fmt.Errorf("cannot record who came in: %w", err)
	}
	return nil
}

// LiveCode is the unredeemed code this inviter already holds for this number,
// or "" - so minting again revokes it rather than piling up.
func (s *MysqlInvitations) LiveCode(ctx context.Context, phone string, inviterId int64) (string, error) {
	const query = "select code, inviter_id, phone, minted_at, redeemed_at, invitee_id from invitations " +
		"where inviter_id = ? and phone = ? and redeemed_at = 0 order by minted_at desc limit 1"
	var row invitationRow
	err := s.db.QueryRowPartial(ctx, &row, query, inviterId, phone)
	if errors.Is(err, sqlx.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("cannot look up a live invitation: %w", err)
	}
	return row.Code, nil
}
