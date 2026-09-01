package core

import (
	"github.com/teamgram/proto/mtproto"
)

// MlsMembersOf says what the delivery service holds about a conversation: the
// leaves it has been told the group holds, whether the device behind each one
// is still answering, and who among them has a device with no leaf here.
//
// It replaces a dance every client did on every round. Each walked its own
// tree, counted the leaves per person, asked how many devices each of those
// people had, and worked out the answer by arithmetic. Only the counting was
// theirs to do; the rest was always the server's to answer, and asking it in
// pieces is how the pieces came to disagree - a count from one call and the
// names from another, about one moment, which nobody dared act on for anybody
// but their own account (#132, #136, #139).
//
// What it does not answer is who is plainly absent from the chat and holds no
// leaf at all. That half needs the chat's participant list, which the caller
// already has and this service does not - and it is the half a client could
// always work out for itself. The half it never could is here.
//
// Nothing is learnt from this that the conversation does not already show:
// every member can see every leaf in it.
//
// mls.membersOf peer_id:long group_id:bytes = mls.Members;
func (c *MlsCore) MlsMembersOf(in *mtproto.TLMlsMembersOf) (*mtproto.Mls_Members, error) {
	groupId := in.GetGroupId()
	if len(groupId) == 0 {
		c.Logger.Errorf("mls.membersOf - a question needs a conversation")
		return nil, mtproto.ErrInternalServerError
	}

	held, wanting, err := c.svcCtx.Members.Holding(c.ctx, groupId)
	if err != nil {
		return nil, mtproto.ErrInternalServerError
	}

	// The epoch is answered even when nothing is known about the membership:
	// it comes from the commit box rather than from the roster, and a device
	// that is behind learns it here rather than by sending a commit to find
	// out.
	epoch, _, err := c.svcCtx.Groups.Epoch(c.ctx, groupId)
	if err != nil {
		c.Logger.Errorf("mls.membersOf - cannot read the epoch of %x: %v", groupId, err)
		epoch = 0
	}

	leaves := make([]*mtproto.Mls_Leaf, 0, len(held))
	dead := 0
	for _, leaf := range held {
		if !leaf.Alive {
			dead++
		}
		leaves = append(leaves, &mtproto.Mls_Leaf{
			Name:   leaf.Name,
			UserId: leaf.UserId,
			Alive:  leaf.Alive,
		})
	}

	// Said out loud, because the two numbers underneath are what every
	// argument about a group turns into: how many leaves nobody is behind, and
	// how many people are here with a phone the group has never met.
	c.Logger.Infof("mls.membersOf - %x at epoch %d holds %d leaf/leaves, %d of them dead, %d person/people short",
		groupId, epoch, len(leaves), dead, len(wanting))
	return &mtproto.Mls_Members{Epoch: epoch, Holds: leaves, Wanting: wanting}, nil
}
