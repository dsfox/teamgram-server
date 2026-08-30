package mls

import (
	"context"
	"database/sql"
	"errors"

	"github.com/teamgram/marmota/pkg/stores/sqlx"

	"github.com/zeromicro/go-zero/core/logx"
)

// MysqlConversations remembers which conversation belongs to which chat.
//
// Nothing decided that before, and every device that wanted to send into a
// chat with no conversation started one of its own. Between two people that
// almost always settles on one; in a group where three people begin within a
// minute it does not, and they end up in conversations that cannot read each
// other with no way back (#135).
//
// The devices cannot settle it among themselves: whoever loses has to be told,
// and when everybody is offline and arrives in a random order there is nobody
// to tell them. So the first claim wins, and the primary key on the chat is
// what makes that atomic - two claims arriving together, one is stored and the
// other is answered with what is stored.
type MysqlConversations struct {
	db *sqlx.DB
}

func NewMysqlConversations(db *sqlx.DB) *MysqlConversations {
	return &MysqlConversations{db: db}
}

type conversationRow struct {
	PeerId  int64  `db:"peer_id"`
	GroupId []byte `db:"group_id"`
	Date    int32  `db:"date"`
}

// Claim says this conversation is the chat's, and answers with the one that
// is - which is the caller's own when it was first, and somebody else's when
// it was not.
func (s *MysqlConversations) Claim(ctx context.Context, peerId int64, groupId []byte, date int32) ([]byte, error) {
	// Insert first and read only if it did not take. Reading first and writing
	// afterwards is the same race written out longhand: two devices both read
	// nothing and both write, and the second overwrites the first.
	const claim = "insert ignore into mls_conversations(peer_id, group_id, date) values (?, ?, ?)"
	result, err := s.db.Exec(ctx, claim, peerId, groupId, date)
	if err != nil {
		logx.WithContext(ctx).Errorf("mls: cannot claim the conversation of %d: %v", peerId, err)
		return nil, err
	}
	if taken, err := result.RowsAffected(); err == nil && taken == 1 {
		return groupId, nil
	}

	return s.Of(ctx, peerId)
}

// Of answers which conversation this chat has, or nothing when it has none.
func (s *MysqlConversations) Of(ctx context.Context, peerId int64) ([]byte, error) {
	var row conversationRow
	const query = "select peer_id, group_id, date from mls_conversations where peer_id = ?"
	if err := s.db.QueryRowPartial(ctx, &row, query, peerId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		logx.WithContext(ctx).Errorf("mls: cannot read the conversation of %d: %v", peerId, err)
		return nil, err
	}
	return row.GroupId, nil
}

// Forget takes the chat's conversation away, so the next claim wins.
//
// For a chat whose conversation nobody can read any more: everybody who held
// it has reinstalled, or it was made before any of this and cannot be joined.
// Without this such a chat would keep pointing at a conversation that exists
// for nobody, and no new one could ever be claimed.
func (s *MysqlConversations) Forget(ctx context.Context, peerId int64) error {
	const query = "delete from mls_conversations where peer_id = ?"
	if _, err := s.db.Exec(ctx, query, peerId); err != nil {
		logx.WithContext(ctx).Errorf("mls: cannot forget the conversation of %d: %v", peerId, err)
		return err
	}
	return nil
}
