package pushnotify

import (
	"testing"

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
	phone := apnsDevice(1, "телефон")
	tablet := apnsDevice(2, "планшет")

	cases := []struct {
		name   string
		list   []devices.DeviceDO
		online []int64
		want   []string
	}{{
		name: "приложение закрыто везде — уведомление на оба устройства",
		list: []devices.DeviceDO{phone, tablet},
		want: []string{"телефон", "планшет"},
	}, {
		name:   "на телефоне приложение открыто — уведомление только на планшет",
		list:   []devices.DeviceDO{phone, tablet},
		online: []int64{1},
		want:   []string{"планшет"},
	}, {
		name:   "человек читает переписку на всех устройствах — уведомлять некого",
		list:   []devices.DeviceDO{phone, tablet},
		online: []int64{1, 2},
		want:   []string{},
	}, {
		name: "чужой тип токена пропускается: отправить его через Apple нельзя",
		list: []devices.DeviceDO{{AuthKeyId: 3, TokenType: 2, Token: "android"}, phone},
		want: []string{"телефон"},
	}, {
		name: "пустой токен пропускается",
		list: []devices.DeviceDO{{AuthKeyId: 4, TokenType: devices.TokenTypeAPNs}, phone},
		want: []string{"телефон"},
	}, {
		name:   "сессия без устройства не мешает остальным",
		list:   []devices.DeviceDO{phone},
		online: []int64{99},
		want:   []string{"телефон"},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := tokens(offlineTargets(c.list, c.online))
			if len(got) != len(c.want) {
				t.Fatalf("получено %v, ожидалось %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("получено %v, ожидалось %v", got, c.want)
				}
			}
		})
	}
}
