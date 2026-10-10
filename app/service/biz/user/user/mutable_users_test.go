package user

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

func someone(id int64, name string) *mtproto.ImmutableUser {
	return mtproto.MakeTLImmutableUser(&mtproto.ImmutableUser{
		User: mtproto.MakeTLUserData(&mtproto.UserData{Id: id, FirstName: name, AccessHash: 1}).To_UserData(),
	}).To_ImmutableUser()
}

// Every id asked for comes back as a user, in the order asked: one whose row
// is gone as an empty user (#210). A difference that named somebody and gave
// no user for them left an iPhone asking again for good.
func TestEveryNamedIdGetsAUser(t *testing.T) {
	users := &Vector_ImmutableUser{Datas: []*mtproto.ImmutableUser{someone(1, "Me"), someone(2, "Here")}}

	got := users.GetUserListByIdList(1, 2, 3)
	if len(got) != 2 {
		t.Fatalf("asked for 2 and 3, got %d users", len(got))
	}
	if got[0].GetId() != 2 || got[0].GetPredicateName() != mtproto.Predicate_user {
		t.Errorf("the user who is here came back as %s %d", got[0].GetPredicateName(), got[0].GetId())
	}
	if got[1].GetId() != 3 || got[1].GetPredicateName() != mtproto.Predicate_userEmpty {
		t.Errorf("the user who is gone came back as %s %d, not as an empty user", got[1].GetPredicateName(), got[1].GetId())
	}
}

func TestWithoutTheOneAskingThereIsNoList(t *testing.T) {
	users := &Vector_ImmutableUser{Datas: []*mtproto.ImmutableUser{someone(2, "Here")}}
	if got := users.GetUserListByIdList(1, 2, 3); len(got) != 0 {
		t.Fatalf("with the one asking missing, %d users came back", len(got))
	}
}
