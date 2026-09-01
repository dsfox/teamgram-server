package mls

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"time"

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

// LeafState is one leaf of a group and whether anybody is behind it.
type LeafState struct {
	Name   []byte
	UserId int64
	Alive  bool
}

// Holding answers what the roster says this group holds, and which of those
// people have a device with no leaf here.
//
// Two questions, one answer, and that is the point of it: they were asked
// separately before - a count of devices from one call, the names of them from
// another - and two answers about one moment could disagree, so nobody dared
// act on them for anybody but their own account. A leaf whose device was gone
// then stayed for ever and every commit was encrypted to it (#139).
//
// A leaf is matched to a device through the name its key packages were
// published under (#136), and **a leaf that matches nothing is called alive**.
// That is deliberate and it is the safe direction: a device that published
// before key packages carried a name cannot be matched at all, and evicting a
// live phone is the worst thing this can lead to. Being wrong the other way
// costs a dead leaf that stays one more round.
//
// Three plain reads and the arithmetic here, rather than one query that invents
// its columns. The schema gate checks a struct's db tags against the database,
// and a `select ... as alive` has no table behind it - the gate said so, and it
// was right: the columns of the tables this really reads were going unchecked.
func (m *MysqlMembers) Holding(ctx context.Context, groupId []byte) ([]LeafState, []int64, error) {
	if len(groupId) == 0 {
		return nil, nil, nil
	}
	seenSince := int32(time.Now().Unix()) - DeviceLife

	held, err := m.leavesOf(ctx, groupId)
	if err != nil {
		return nil, nil, err
	}
	if len(held) == 0 {
		return nil, nil, nil
	}

	names := make([][]byte, 0, len(held))
	for _, row := range held {
		names = append(names, row.Leaf)
	}
	behind, err := m.devicesBehind(ctx, names)
	if err != nil {
		return nil, nil, err
	}

	people := map[int64]bool{}
	for _, row := range held {
		people[row.UserId] = true
	}
	answering, err := m.answering(ctx, people, seenSince)
	if err != nil {
		return nil, nil, err
	}

	leaves := make([]LeafState, 0, len(held))
	alive := map[int64]int{}
	for _, row := range held {
		device, known := behind[string(row.Leaf)]
		// Unknown counts as alive, for the reason above.
		living := !known || answering[device.user][device.key]
		leaves = append(leaves, LeafState{Name: row.Leaf, UserId: row.UserId, Alive: living})
		if living {
			alive[row.UserId]++
		} else if _, seen := alive[row.UserId]; !seen {
			alive[row.UserId] = 0
		}
	}

	// Who has more devices answering than *live* leaves here.
	//
	// Live, and that is the whole of it. Somebody who has replaced a phone has
	// one device answering and one leaf, and counting leaves would say they are
	// not missing anything - which is exactly the reading that left them
	// watching padlocks for ever (#132). The leaf they have is the dead one.
	short := make([]int64, 0)
	for who, living := range alive {
		if len(answering[who]) > living {
			short = append(short, who)
		}
	}
	sort.Slice(short, func(i, j int) bool { return short[i] < short[j] })
	return leaves, short, nil
}

// The leaves this group is recorded as holding.
func (m *MysqlMembers) leavesOf(ctx context.Context, groupId []byte) ([]memberRow, error) {
	var rows []memberRow
	const held = "select group_id, leaf, user_id, epoch, joined_at, seen_at " +
		"from mls_members where group_id = ? order by leaf"
	if err := m.db.QueryRowsPartial(ctx, &rows, held, groupId); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logx.WithContext(ctx).Errorf("mls members - cannot read what %x holds: %v", groupId, err)
			return nil, err
		}
	}
	return rows, nil
}

// One key package, by the columns that say whose device published it.
type packageOfRow struct {
	Name      []byte `db:"name"`
	UserId    int64  `db:"user_id"`
	AuthKeyId int64  `db:"auth_key_id"`
}

type deviceKey struct {
	user int64
	key  int64
}

// Which device each of these leaves belongs to, where anybody can say.
//
// The mapping lives in the name a device publishes its key packages under
// (#136). A leaf missing from the answer is one nobody can account for, and the
// caller treats that as alive.
func (m *MysqlMembers) devicesBehind(ctx context.Context, names [][]byte) (map[string]deviceKey, error) {
	behind := map[string]deviceKey{}
	if len(names) == 0 {
		return behind, nil
	}
	var rows []packageOfRow
	published := "select name, user_id, auth_key_id from mls_key_packages " +
		"where name in (" + placeholders(len(names)) + ")"
	args := make([]any, 0, len(names))
	for _, name := range names {
		args = append(args, name)
	}
	if err := m.db.QueryRowsPartial(ctx, &rows, published, args...); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logx.WithContext(ctx).Errorf("mls members - cannot match leaves to devices: %v", err)
			return nil, err
		}
	}
	for _, row := range rows {
		behind[string(row.Name)] = deviceKey{user: row.UserId, key: row.AuthKeyId}
	}
	return behind, nil
}

// One live device of somebody, by the two columns that name it.
type deviceOfRow struct {
	UserId    int64 `db:"user_id"`
	AuthKeyId int64 `db:"auth_key_id"`
}

// Which devices of these people have been heard from lately.
func (m *MysqlMembers) answering(ctx context.Context, people map[int64]bool, since int32) (map[int64]map[int64]bool, error) {
	answered := map[int64]map[int64]bool{}
	if len(people) == 0 {
		return answered, nil
	}
	var rows []deviceOfRow
	live := "select user_id, auth_key_id from mls_devices " +
		"where user_id in (" + placeholders(len(people)) + ") and last_seen > ?"
	args := make([]any, 0, len(people)+1)
	for who := range people {
		args = append(args, who)
	}
	args = append(args, since)
	if err := m.db.QueryRowsPartial(ctx, &rows, live, args...); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logx.WithContext(ctx).Errorf("mls members - cannot count who is answering: %v", err)
			return nil, err
		}
	}
	for _, row := range rows {
		if answered[row.UserId] == nil {
			answered[row.UserId] = map[int64]bool{}
		}
		answered[row.UserId][row.AuthKeyId] = true
	}
	return answered, nil
}

// placeholders is `?, ?, ?` for an `in` list. A list of none is impossible to
// produce by accident: `in ()` is a syntax error, and every caller checks for
// it before asking.
func placeholders(n int) string {
	if n <= 0 {
		return "null"
	}
	out := make([]byte, 0, n*3)
	for i := 0; i < n; i++ {
		if i > 0 {
			out = append(out, ',', ' ')
		}
		out = append(out, '?')
	}
	return string(out)
}
