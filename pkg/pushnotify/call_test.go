package pushnotify

import (
	"context"
	"testing"

	"github.com/teamgram/teamgram-server/pkg/devices"
	"github.com/teamgram/teamgram-server/pkg/fcm"
)

// A call rings every device of the callee a push can reach, awake or not:
// the iPhone through its VoIP token with the call sealed inside, the Android
// through its one token with a wake-up. The iPhone's message token is left
// alone - a banner is not a ring, and iOS kills an app that gets a VoIP push
// with no call in it, so nothing but a call may ever use that token. A device
// with no secret cannot be handed a call it could open, so it is skipped
// rather than sent something empty.
func TestACallRingsTheVoipAndFcmTokensAndNoOther(t *testing.T) {
	n, registry, got := notifierForTest(t, 200)
	secret := secretForTest()
	registry.list = []devices.DeviceDO{
		{TokenType: devices.TokenTypeAPNs, Token: "apns-tok", AppSandbox: true, Secret: secret, UserId: 1002, AuthKeyId: 1},
		{TokenType: devices.TokenTypeAPNsVoIP, Token: "voip-tok", AppSandbox: true, Secret: secret, UserId: 1002, AuthKeyId: 1},
		{TokenType: devices.TokenTypeFCM, Token: "fcm-tok", Secret: secret, UserId: 1002, AuthKeyId: 2},
		{TokenType: devices.TokenTypeFCM, Token: "fcm-no-secret", UserId: 1002, AuthKeyId: 3},
	}
	updates := []byte{0x74, 0xae, 0x8f, 0x1b, 9, 9}

	n.ringDevices(context.Background(), 1002, 1001, 777, updates)

	if len(*got) != 2 {
		t.Fatalf("the relay saw %d pushes, expected the VoIP and the FCM one: %v", len(*got), *got)
	}
	byToken := map[string]map[string]any{}
	for _, sent := range *got {
		byToken[sent["token"].(string)] = sent
		if sent["call"] != true {
			t.Errorf("%s was not sent as a call: %v", sent["token"], sent)
		}
		for field := range sent {
			switch field {
			case "platform", "token", "sandbox", "call", "p", "badge":
			default:
				t.Errorf("a field the relay does not take for a call: %s", field)
			}
		}
	}

	apple := byToken["voip-tok"]
	if apple == nil || apple["platform"] != "apns" || apple["sandbox"] != true {
		t.Fatalf("the VoIP push is %v", apple)
	}
	inside, err := fcm.OpenEnvelope(secret, apple["p"].(string))
	if err != nil {
		t.Fatalf("the iPhone cannot open its envelope: %v", err)
	}
	if inside["updates"] != "dK6PGwkJ" {
		t.Errorf("the iPhone's envelope carries %v, not the update", inside)
	}

	android := byToken["fcm-tok"]
	if android == nil || android["platform"] != "fcm" {
		t.Fatalf("the FCM push is %v", android)
	}
	inside, err = fcm.OpenEnvelope(secret, android["p"].(string))
	if err != nil {
		t.Fatalf("the Android cannot open its envelope: %v", err)
	}
	if inside["loc_key"] != "PHONE_CALL_REQUEST" || inside["user_id"] != "1002" {
		t.Errorf("the Android's envelope is %v", inside)
	}
}

// A token Apple says is gone is forgotten on a call as on a message.
func TestADeadVoipTokenIsForgotten(t *testing.T) {
	n, registry, _ := notifierForTest(t, 410)
	registry.list = []devices.DeviceDO{
		{TokenType: devices.TokenTypeAPNsVoIP, Token: "old-voip", Secret: secretForTest(), UserId: 1002, AuthKeyId: 1},
	}
	n.ringDevices(context.Background(), 1002, 1001, 777, []byte{1})
	if len(registry.forgotten) != 1 || registry.forgotten[0] != "old-voip" {
		t.Fatalf("forgotten: %v", registry.forgotten)
	}
}
