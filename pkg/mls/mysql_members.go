package mls

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/teamgram/marmota/pkg/stores/sqlx"

	"github.com/zeromicro/go-zero/core/logx"
)

// MysqlMembers remembers who holds a leaf in each conversation (#147).
//
// It is told rather than inferred, and that distinction is the whole design.
// The server cannot open a tree, so it cannot tell an addition from a removal
// by looking - that is a property of MLS, not a gap here. What it can do is be
// told by the one party that knows for certain: the device that made the
// commit, whose own tree it is and which has just changed it.
//
// Whole rather than incremental. Any commit repairs the roster, so drift has
// nowhere to accumulate; an addition and a removal are the same operation; and
// a server that missed one message is corrected by the next rather than staying
// wrong for ever. It is the same reasoning that made membership a fact to be
// compared rather than an event to be hooked (#124), one layer down.
//
// Nothing checks that the list is true and nothing can. It grants the committer
// nothing they did not have: whoever can commit already decides the membership.
// The roster records what they did; it does not extend what they may do. So it
// is evidence about the world rather than a rule about it, and it is checked
// the way evidence is - against the phones.
type MysqlMembers struct {
	db *sqlx.DB
}

func NewMysqlMembers(db *sqlx.DB) *MysqlMembers {
	return &MysqlMembers{db: db}
}

// One row of the roster, as the table holds it.
type memberRow struct {
	GroupId  []byte `db:"group_id"`
	Leaf     []byte `db:"leaf"`
	UserId   int64  `db:"user_id"`
	Epoch    int64  `db:"epoch"`
	JoinedAt int32  `db:"joined_at"`
	SeenAt   int32  `db:"seen_at"`
}

// Record replaces what this group is known to hold.
//
// An empty roster is not a statement. A caller with nothing to say sends none -
// the vouch of #139 has no tree in scope - and taking that for "this group holds
// nobody" would empty a roster every time somebody said something unrelated.
//
// A roster from an epoch older than the one already recorded is ignored.
// Commits arrive in order on the ordinary route and out of order on every
// other, and a late one would otherwise put the group back the way it was.
//
// No transaction. What the roster names is written first and what it does not
// name is deleted after, so a reader in between sees a group with somebody in
// it twice over rather than a group with nobody in it - and the next commit
// repairs whatever a crash in the middle left, which is the property the whole
// design rests on.
func (m *MysqlMembers) Record(ctx context.Context, groupId []byte, epoch int64, leaves [][]byte, now int32) error {
	if len(groupId) == 0 || len(leaves) == 0 {
		return nil
	}

	// The newest row of this group, read whole rather than as one column: the
	// schema gate checks a struct's db tags against the database only when the
	// query beside it names the table, so a `select max(epoch)` would leave
	// every column of mls_members unchecked - and the gate said so.
	var newest memberRow
	const latest = "select group_id, leaf, user_id, epoch, joined_at, seen_at " +
		"from mls_members where group_id = ? order by epoch desc limit 1"
	recorded := int64(-1)
	if err := m.db.QueryRowPartial(ctx, &newest, latest, groupId); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logx.WithContext(ctx).Errorf("mls members - cannot read what %x holds: %v", groupId, err)
			return err
		}
	} else {
		recorded = newest.Epoch
	}
	if recorded > epoch {
		logx.WithContext(ctx).Infof(
			"mls members - %x is already at epoch %d, ignoring a roster from %d",
			groupId, recorded, epoch)
		return nil
	}

	const remember = "insert into mls_members (group_id, leaf, user_id, epoch, joined_at, seen_at) " +
		"values (?, ?, ?, ?, ?, ?) " +
		"on duplicate key update epoch = greatest(epoch, values(epoch)), seen_at = values(seen_at)"
	written := 0
	for _, leaf := range leaves {
		// The only thing parsed out of a leaf, and it is parsed here rather
		// than trusted from the caller: a device naming somebody else's user id
		// would be writing a row about them.
		who, ok := userOf(leaf)
		if !ok {
			logx.WithContext(ctx).Errorf("mls members - %q is not a leaf name, skipping it", leaf)
			continue
		}
		if _, err := m.db.Exec(ctx, remember, groupId, leaf, who, epoch, now, now); err != nil {
			logx.WithContext(ctx).Errorf("mls members - cannot write a leaf of %x: %v", groupId, err)
			return err
		}
		written++
	}
	if written == 0 {
		return nil
	}

	// Everything the roster did not name is gone from the group, and it is said
	// by what is *not* in the list rather than by a removal message - the server
	// is never told about removals and never can be.
	//
	// By epoch and by when it was last named, because two rosters can arrive at
	// one epoch: a claim carries no epoch at all and every one of them is
	// recorded at zero. What was named a moment ago has this pass's `seen_at`
	// and survives; what was not still has the old one and goes. Two rosters
	// within the same second at the same epoch are the one case this cannot
	// separate, and the next commit repairs it.
	const forget = "delete from mls_members where group_id = ? and (epoch < ? or (epoch = ? and seen_at < ?))"
	if _, err := m.db.Exec(ctx, forget, groupId, epoch, epoch, now); err != nil {
		logx.WithContext(ctx).Errorf("mls members - cannot forget the leaves %x lost: %v", groupId, err)
		return err
	}
	return nil
}

// userOf reads the account out of a leaf name, which is <user_id>/<device_id>.
func userOf(leaf []byte) (int64, bool) {
	slash := bytes.IndexByte(leaf, '/')
	if slash <= 0 {
		return 0, false
	}
	who, err := strconv.ParseInt(string(leaf[:slash]), 10, 64)
	if err != nil || who == 0 {
		return 0, false
	}
	return who, true
}

func holds(leaves [][]byte, leaf []byte) bool {
	for _, held := range leaves {
		if bytes.Equal(held, leaf) {
			return true
		}
	}
	return false
}
