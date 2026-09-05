package pushrelay

import (
	"strings"
	"testing"
)

func TestDecodeRefusesWords(t *testing.T) {
	for _, body := range []string{
		`{"platform":"apns","token":"ab","title":"hi"}`,
		`{"platform":"apns","token":"ab","body":"buy now"}`,
		`{"platform":"apns","token":"ab","text":"x"}`,
	} {
		if _, err := Decode(strings.NewReader(body)); err == nil {
			t.Errorf("accepted a field that could carry words: %s", body)
		}
	}
}

func TestDecodeAcceptsExactlyTheSixFields(t *testing.T) {
	p, err := Decode(strings.NewReader(`{"platform":"fcm","token":"t","sandbox":false,"badge":2,"from_id":"5","p":"AAAA"}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Platform != "fcm" || p.Token != "t" || p.Badge != 2 || p.FromId != "5" || p.P != "AAAA" {
		t.Fatalf("decoded wrong: %+v", p)
	}
	if _, err := Decode(strings.NewReader(`{"platform":"sms","token":"t"}`)); err == nil {
		t.Error("an unknown platform was accepted")
	}
	if _, err := Decode(strings.NewReader(`{"platform":"apns"}`)); err == nil {
		t.Error("a push without a token was accepted")
	}
}
