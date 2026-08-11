package pushnotify

import (
	"testing"

	"github.com/teamgram/proto/mtproto"

	"github.com/teamgram/teamgram-server/pkg/devices"
)

func apnsDevice(authKeyId int64, token string) devices.DeviceDO {
	return devices.DeviceDO{
		AuthKeyId: authKeyId,
		TokenType: devices.TokenTypeAPNs,
		Token:     token,
	}
}

func tokens(list []devices.DeviceDO) []string {
	out := make([]string, 0, len(list))
	for _, d := range list {
		out = append(out, d.Token)
	}

	return out
}

func TestOfflineTargets(t *testing.T) {
	phone := apnsDevice(1, "phone")
	tablet := apnsDevice(2, "tablet")

	cases := []struct {
		name   string
		list   []devices.DeviceDO
		online []int64
		want   []string
	}{{
		name: "app closed everywhere: notify both devices",
		list: []devices.DeviceDO{phone, tablet},
		want: []string{"phone", "tablet"},
	}, {
		name:   "app open on the phone: notify only the tablet",
		list:   []devices.DeviceDO{phone, tablet},
		online: []int64{1},
		want:   []string{"tablet"},
	}, {
		name:   "person reads on every device: nobody to notify",
		list:   []devices.DeviceDO{phone, tablet},
		online: []int64{1, 2},
		want:   []string{},
	}, {
		name: "an Android device is a target too, through Firebase",
		list: []devices.DeviceDO{{AuthKeyId: 3, TokenType: devices.TokenTypeFCM, Token: "android"}, phone},
		want: []string{"android", "phone"},
	}, {
		name:   "an Android device with the app open is left alone",
		list:   []devices.DeviceDO{{AuthKeyId: 3, TokenType: devices.TokenTypeFCM, Token: "android"}, phone},
		online: []int64{3},
		want:   []string{"phone"},
	}, {
		name: "a token type we cannot reach is skipped",
		list: []devices.DeviceDO{{AuthKeyId: 5, TokenType: 7, Token: "something else"}, phone},
		want: []string{"phone"},
	}, {
		name: "empty token is skipped",
		list: []devices.DeviceDO{{AuthKeyId: 4, TokenType: devices.TokenTypeAPNs}, phone},
		want: []string{"phone"},
	}, {
		name: "an empty Firebase token is skipped as well",
		list: []devices.DeviceDO{{AuthKeyId: 6, TokenType: devices.TokenTypeFCM}, phone},
		want: []string{"phone"},
	}, {
		name:   "a session without a device does not disturb the others",
		list:   []devices.DeviceDO{phone},
		online: []int64{99},
		want:   []string{"phone"},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := tokens(offlineTargets(c.list, c.online))
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}

// A notification has to say which conversation it is about.
//
// Without it a tap opens whatever the app opens by default - which is what
// happened on a real phone: the push arrived at once, and tapping it went to
// the settings screen. The client reads `from_id`, as a decimal string, and
// only for a conversation between two people.
func TestNotificationNamesTheChat(t *testing.T) {
	if int32(mtproto.PEER_USER) != 2 {
		t.Fatalf("the client's numbering for a private chat has moved: %d", mtproto.PEER_USER)
	}
}
