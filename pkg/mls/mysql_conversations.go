package mls

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"

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
// is - which is the caller's own when it was first, when nobody has touched
// what was there for longer than an invitation lives, and somebody else's
// otherwise.
//
// The second case is what keeps a chat from dying quietly. A device told a
// different conversation throws its own away and waits to be let in, which is
// right while somebody is inside the one that won. When nobody is - every phone
// in the chat reinstalled, or the answer was won by a conversation a rebuilding
// device made and nobody followed - every device loses, waits, and there is
// nobody left to invite any of them. Nothing else in the system ever revisits
// the row, so the chat stops being encrypted for good (#142).
//
// A fortnight is not a guess. It is WelcomeLife read out loud: an invitation is
// swept after that long (#138, #140), so a device that lost a claim and was not
// invited within one will never be invited - the invitation, if it was ever
// made, has gone. Holding the answer past that point costs the chat its
// encryption for ever and buys nothing, which is why the two numbers are the
// same one rather than two that happen to agree.
//
// It applies to fewer and fewer rows. A conversation the roster knows about is
// decided above, by whether anybody is still in it; only one settled before the
// roster existed and not committed in since reaches the fortnight at all, and
// any commit takes a row out of that set for good (#147).
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

	held, err := s.Of(ctx, peerId)
	if err != nil {
		return nil, err
	}
	if held == nil || bytes.Equal(held, groupId) {
		return held, nil
	}

	// Whether the conversation holding this chat still has anybody in it who is
	// here. That question could not be asked before - nobody knew who was in a
	// group - and a fortnight since the claim was the closest thing to an
	// answer (#142). Now it is asked directly, and a conversation nobody is
	// left in stops holding the chat at once (#147).
	known, live, err := s.stillThere(ctx, held)
	if err != nil {
		return nil, err
	}

	var takenOver bool
	if known {
		if live > 0 {
			return held, nil
		}
		// The group id is in the condition for the same reason the insert above
		// is an insert-ignore: two devices arriving together on a conversation
		// nobody is in would otherwise both take it, which is the split this
		// row exists to prevent. The first moves it and the second no longer
		// matches what it judged.
		const takeOver = "update mls_conversations set group_id = ?, date = ? " +
			"where peer_id = ? and group_id = ?"
		result, err = s.db.Exec(ctx, takeOver, groupId, date, peerId, held)
		if err != nil {
			logx.WithContext(ctx).Errorf(
				"mls: cannot take over the conversation of %d: %v", peerId, err)
			return nil, err
		}
		taken, err := result.RowsAffected()
		takenOver = err == nil && taken == 1
		if takenOver {
			logx.WithContext(ctx).Infof(
				"mls: %x held %d and has nobody left in it; %x has it now",
				held, peerId, groupId)
		}
	} else {
		// Nothing is known about that conversation: it was settled before the
		// roster existed and nobody has committed in it since, so there is no
		// list to find nobody in. The old rule stands for exactly these, and
		// only these - releasing a chat because the server happens not to know
		// who is in its conversation would hand it to whoever asked next.
		//
		// It empties itself: any commit or any new group writes a roster. When
		// the last conversation without one has gone, this branch can go with
		// it, and until then a fortnight is still better than for ever (#142).
		const takeOverStale = "update mls_conversations set group_id = ?, date = ? " +
			"where peer_id = ? and date < ?"
		result, err = s.db.Exec(ctx, takeOverStale, groupId, date, peerId, date-WelcomeLife)
		if err != nil {
			logx.WithContext(ctx).Errorf(
				"mls: cannot take over the conversation of %d: %v", peerId, err)
			return nil, err
		}
		taken, err := result.RowsAffected()
		takenOver = err == nil && taken == 1
		if takenOver {
			logx.WithContext(ctx).Infof(
				"mls: %d had been held by a conversation nobody came back to and nothing is known about; %x has it now",
				peerId, groupId)
		}
	}
	if takenOver {
		return groupId, nil
	}

	return s.Of(ctx, peerId)
}

// How long a device may go without asking the server anything before it is
// taken for gone.
//
// The same fortnight the device count uses (#138), and for the same reason:
// a phone that comes back after longer publishes, is seen, and is let into
// whatever it missed by the comparison that already runs, while a phantom kept
// is a dead leaf in every conversation started from then on.
const DeviceLife = int32(14 * 86400)

// stillThere says whether anything is known about who holds this conversation,
// and how many of them have a device that has been seen lately.
//
// The two answers are separate on purpose. "Nobody is in it" and "nobody has
// told us who is in it" look identical from a count of zero, and they call for
// opposite things: the first releases the chat, the second must not.
//
// Asked about people rather than about leaves, because a leaf names a device by
// the MLS identity's own number and the mapping to an auth key lives in
// mls_key_packages.name - which step 3 of #147 reads and this does not need: a
// conversation with a live member in it is being used, whichever of their
// phones is doing the using.
func (s *MysqlConversations) stillThere(ctx context.Context, groupId []byte) (known bool, live int, err error) {
	var held int
	if err = s.db.QueryRowPartial(ctx, &held,
		"select count(*) from mls_members where group_id = ?", groupId); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logx.WithContext(ctx).Errorf("mls: cannot read who holds %x: %v", groupId, err)
			return false, 0, err
		}
		err = nil
	}
	if held == 0 {
		return false, 0, nil
	}

	if err = s.db.QueryRowPartial(ctx, &live,
		"select count(*) from mls_members m join mls_devices d on d.user_id = m.user_id "+
			"where m.group_id = ? and d.last_seen > ?",
		groupId, int32(time.Now().Unix())-DeviceLife); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logx.WithContext(ctx).Errorf("mls: cannot count who is still in %x: %v", groupId, err)
			return true, 0, err
		}
		err = nil
	}
	return true, live, nil
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
