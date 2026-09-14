package core

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

func rule(predicate string, users ...int64) *mtproto.PrivacyRule {
	return &mtproto.PrivacyRule{PredicateName: predicate, Users: users}
}

// The answer a call's p2p_allowed rests on (#14): "who may connect to me
// directly". Everybody, my contacts, nobody, with exceptions - the three
// choices the phones offer and the two lists beside them.
func TestPrivacyRulesDecideWhoIsAllowed(t *testing.T) {
	const me, friend, stranger = int64(1), int64(2), int64(3)
	contacts := map[int64]bool{friend: true}

	for _, tc := range []struct {
		name  string
		rules []*mtproto.PrivacyRule
		peer  int64
		want  bool
	}{
		{"no rules means everybody", nil, stranger, true},
		{"everybody", []*mtproto.PrivacyRule{rule(mtproto.Predicate_privacyValueAllowAll)}, stranger, true},
		{"everybody but one", []*mtproto.PrivacyRule{rule(mtproto.Predicate_privacyValueAllowAll), rule(mtproto.Predicate_privacyValueDisallowUsers, stranger)}, stranger, false},
		{"contacts, and this is one", []*mtproto.PrivacyRule{rule(mtproto.Predicate_privacyValueAllowContacts)}, friend, true},
		{"contacts, and this is not one", []*mtproto.PrivacyRule{rule(mtproto.Predicate_privacyValueAllowContacts)}, stranger, false},
		{"contacts plus one more", []*mtproto.PrivacyRule{rule(mtproto.Predicate_privacyValueAllowContacts), rule(mtproto.Predicate_privacyValueAllowUsers, stranger)}, stranger, true},
		{"nobody", []*mtproto.PrivacyRule{rule(mtproto.Predicate_privacyValueDisallowAll)}, friend, false},
		{"nobody but one", []*mtproto.PrivacyRule{rule(mtproto.Predicate_privacyValueDisallowAll), rule(mtproto.Predicate_privacyValueAllowUsers, friend)}, friend, true},
	} {
		got := privacyAllows(me, tc.rules, tc.peer, func(id, peer int64) bool { return id == me && contacts[peer] })
		if got != tc.want {
			t.Errorf("%s: %v, expected %v", tc.name, got, tc.want)
		}
	}
}
