package quietlog

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoregistry"

	_ "github.com/teamgram/proto/mtproto"
	_ "github.com/teamgram/teamgram-server/app/service/biz/user/user"
)

func TestTheMethodsThatCarryAPersonAreFound(t *testing.T) {
	found := map[string]bool{}
	for _, method := range NewScanner(protoregistry.GlobalFiles).Methods() {
		found[method] = true
	}

	for _, method := range []string{
		// The failure that started this: the number of somebody not on ice9.
		"/user.RPCUser/user_getUserIdByPhone",
		"/mtproto.RPCAuthorization/auth_signIn",
		"/mtproto.RPCAuthorization/auth_signUp",
		// An address book is a list of InputContact: one message down.
		"/mtproto.RPCContacts/contacts_importContacts",
	} {
		if !found[method] {
			t.Errorf("%s carries a person and is not found", method)
		}
	}

	// And it is not everything: a call names a user by id, a message is
	// content the policy speaks of separately.
	for _, method := range []string{
		"/mtproto.RPCVoipCalls/phone_requestCall",
		"/mtproto.RPCMessages/messages_sendMessage",
	} {
		if found[method] {
			t.Errorf("%s carries no personal field and is found anyway", method)
		}
	}
}
