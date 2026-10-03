package reactions

import (
	"context"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
)

// Store keeps who reacted to which message with what: one reaction per person
// per message.
type Store interface {
	Set(ctx context.Context, dialogMessageId, userId int64, reaction string, at int32) error
	Remove(ctx context.Context, dialogMessageId, userId int64) error
	Of(ctx context.Context, dialogMessageId int64) ([]MessageReactionsDO, error)
}

type MysqlStore struct {
	db *sqlx.DB
}

func NewMysqlStore(db *sqlx.DB) *MysqlStore {
	return &MysqlStore{db: db}
}

// Set gives a person's reaction to a message, replacing the one they had.
func (s *MysqlStore) Set(ctx context.Context, dialogMessageId, userId int64, reaction string, at int32) error {
	const query = "insert into message_reactions(dialog_message_id, user_id, reaction, reacted_at) " +
		"values (?, ?, ?, ?) on duplicate key update reaction = values(reaction), reacted_at = values(reacted_at)"
	_, err := s.db.Exec(ctx, query, dialogMessageId, userId, reaction, at)
	return err
}

func (s *MysqlStore) Remove(ctx context.Context, dialogMessageId, userId int64) error {
	const query = "delete from message_reactions where dialog_message_id = ? and user_id = ?"
	_, err := s.db.Exec(ctx, query, dialogMessageId, userId)
	return err
}

func (s *MysqlStore) Of(ctx context.Context, dialogMessageId int64) ([]MessageReactionsDO, error) {
	const query = "select dialog_message_id, user_id, reaction, reacted_at from message_reactions " +
		"where dialog_message_id = ?"
	var rows []MessageReactionsDO
	if err := s.db.QueryRowsPartial(ctx, &rows, query, dialogMessageId); err != nil {
		return nil, err
	}
	return rows, nil
}
