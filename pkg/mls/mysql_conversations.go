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
//
// Which chat a row is about is a pair rather than a number, and that is the
// whole of #155. A device names a chat by its dialog id, and for a chat between
// two that is the other person - so each side calls one chat by a different
// number, each is first under its own, both win, and they talk in conversations
// that cannot read each other with nothing anywhere to say so. ChatKey below is
// where the two names become one, and everything here is keyed by what it
// returns.
type MysqlConversations struct {
	db *sqlx.DB
}

func NewMysqlConversations(db *sqlx.DB) *MysqlConversations {
	return &MysqlConversations{db: db}
}

// ChatKey names a chat the same way on both sides of it.
//
// A group is already named the same everywhere: its dialog id is the chat
// itself, it is negative, and who is asking does not change it.
//
// A chat between two is named by neither side alone. The caller says "the chat
// with that person" and means the pair, so the pair is what is written down,
// smallest first - the one form both sides produce out of the two halves each
// of them holds (#155).
//
// A chat with oneself is the pair of one number, which is symmetric already and
// needs no case of its own.
func ChatKey(callerId, peerId int64) (int64, int64) {
	if peerId <= 0 {
		return peerId, 0
	}
	if callerId < peerId {
		return callerId, peerId
	}
	return peerId, callerId
}

type conversationRow struct {
	PeerId  int64  `db:"peer_id"`
	WithId  int64  `db:"with_id"`
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
func (s *MysqlConversations) Claim(ctx context.Context, peerId, withId int64, groupId []byte, date int32) ([]byte, error) {
	// Insert first and read only if it did not take. Reading first and writing
	// afterwards is the same race written out longhand: two devices both read
	// nothing and both write, and the second overwrites the first.
	const claim = "insert ignore into mls_conversations(peer_id, with_id, group_id, date) values (?, ?, ?, ?)"
	result, err := s.db.Exec(ctx, claim, peerId, withId, groupId, date)
	if err != nil {
		logx.WithContext(ctx).Errorf("mls: cannot claim the conversation of %d/%d: %v", peerId, withId, err)
		return nil, err
	}
	if taken, err := result.RowsAffected(); err == nil && taken == 1 {
		return groupId, nil
	}

	held, err := s.Of(ctx, peerId, withId)
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
			"where peer_id = ? and with_id = ? and group_id = ?"
		result, err = s.db.Exec(ctx, takeOver, groupId, date, peerId, withId, held)
		if err != nil {
			logx.WithContext(ctx).Errorf(
				"mls: cannot take over the conversation of %d/%d: %v", peerId, withId, err)
			return nil, err
		}
		taken, err := result.RowsAffected()
		takenOver = err == nil && taken == 1
		if takenOver {
			logx.WithContext(ctx).Infof(
				"mls: %x held %d/%d and has nobody left in it; %x has it now",
				held, peerId, withId, groupId)
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
			"where peer_id = ? and with_id = ? and date < ?"
		result, err = s.db.Exec(ctx, takeOverStale, groupId, date, peerId, withId, date-WelcomeLife)
		if err != nil {
			logx.WithContext(ctx).Errorf(
				"mls: cannot take over the conversation of %d/%d: %v", peerId, withId, err)
			return nil, err
		}
		taken, err := result.RowsAffected()
		takenOver = err == nil && taken == 1
		if takenOver {
			logx.WithContext(ctx).Infof(
				"mls: %d/%d had been held by a conversation nobody came back to and nothing is known about; %x has it now",
				peerId, withId, groupId)
		}
	}
	if takenOver {
		return groupId, nil
	}

	return s.Of(ctx, peerId, withId)
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
//
// The chat is the pair from ChatKey and never one side's name for it: asking
// under one number answers about a different chat on each of the two phones in
// it (#155).
func (s *MysqlConversations) Of(ctx context.Context, peerId, withId int64) ([]byte, error) {
	var row conversationRow
	const query = "select peer_id, with_id, group_id, date from mls_conversations " +
		"where peer_id = ? and with_id = ?"
	if err := s.db.QueryRowPartial(ctx, &row, query, peerId, withId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		logx.WithContext(ctx).Errorf(
			"mls: cannot read the conversation of %d/%d: %v", peerId, withId, err)
		return nil, err
	}
	return row.GroupId, nil
}
