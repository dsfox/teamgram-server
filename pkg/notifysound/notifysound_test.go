package notifysound

import (
	"strings"
	"testing"

	"github.com/teamgram/proto/mtproto"
)

func TestEveryChoiceSurvivesTheColumn(t *testing.T) {
	cases := []struct {
		sound  *mtproto.NotificationSound
		stored string
	}{
		{nil, "default"},
		{mtproto.MakeTLNotificationSoundDefault(nil).To_NotificationSound(), "default"},
		{mtproto.MakeTLNotificationSoundNone(nil).To_NotificationSound(), "none"},
		{mtproto.MakeTLNotificationSoundRingtone(&mtproto.NotificationSound{Id: 77}).To_NotificationSound(), "ringtone:77"},
		{mtproto.MakeTLNotificationSoundLocal(&mtproto.NotificationSound{Title: "Adara", Data: "content://media/internal/audio/media/42"}).To_NotificationSound(), "local:Adara\tcontent://media/internal/audio/media/42"},
	}
	for _, c := range cases {
		if got := Encode(c.sound); got != c.stored {
			t.Errorf("%v encodes as %q, not %q", c.sound, got, c.stored)
		}
		back := Decode(c.stored)
		want := c.sound
		if want == nil {
			want = mtproto.MakeTLNotificationSoundDefault(nil).To_NotificationSound()
		}
		if back.GetPredicateName() != want.GetPredicateName() || back.GetId() != want.GetId() ||
			back.GetTitle() != want.GetTitle() || back.GetData() != want.GetData() {
			t.Errorf("%q decodes as %v, not %v", c.stored, back, want)
		}
	}
}

func TestWhatTheTableHeldBeforeReadsAsTheDefault(t *testing.T) {
	for _, stored := range []string{"", "-1", "default", "100", "ringtone:", "ringtone:x", "nonsense"} {
		if got := Decode(stored).GetPredicateName(); got != mtproto.Predicate_notificationSoundDefault {
			t.Errorf("%q decodes as %s", stored, got)
		}
		if IsChosen(stored) && stored != "100" && stored != "nonsense" && stored != "ringtone:" && stored != "ringtone:x" {
			t.Errorf("%q counts as a choice", stored)
		}
	}
}

func TestALocalSoundTooLongForTheColumnIsTheDefault(t *testing.T) {
	long := mtproto.MakeTLNotificationSoundLocal(&mtproto.NotificationSound{
		Title: "x", Data: strings.Repeat("y", 300),
	}).To_NotificationSound()
	if got := Encode(long); got != "default" {
		t.Fatalf("a %d-byte value would not fit varchar(255): %q", len(got), got[:20])
	}
}

func TestApplyWritesTheSameChoiceForEveryPlatform(t *testing.T) {
	settings := &mtproto.PeerNotifySettings{}
	Apply(settings, "none")
	for _, sound := range []*mtproto.NotificationSound{settings.IosSound, settings.AndroidSound, settings.OtherSound} {
		if sound.GetPredicateName() != mtproto.Predicate_notificationSoundNone {
			t.Fatalf("a platform reads %s", sound.GetPredicateName())
		}
	}
	if settings.IosSound == settings.AndroidSound {
		t.Fatal("two platforms share one object; a later edit of one would change the other")
	}
}

func TestWhatTheEnvelopeCarries(t *testing.T) {
	if sound, ringtone := ForApple("default"); sound != "default" || ringtone != "" {
		t.Fatalf("default: %q %q", sound, ringtone)
	}
	if sound, ringtone := ForApple("none"); sound != "" || ringtone != "" {
		t.Fatalf("none should carry no sound: %q %q", sound, ringtone)
	}
	if sound, ringtone := ForApple("ringtone:77"); sound != "0.m4a" || ringtone != "77" {
		t.Fatalf("a tone: %q %q", sound, ringtone)
	}
	if sound, ringtone := ForApple("local:Adara\tcontent://x"); sound != "default" || ringtone != "" {
		t.Fatalf("a phone's own sound is nothing an iPhone can play: %q %q", sound, ringtone)
	}
}

func TestDescribe(t *testing.T) {
	for stored, said := range map[string]string{
		"": "default", "-1": "default", "default": "default", "none": "none",
		"ringtone:7": "ringtone 7", "local:Adara\tcontent://x": "the phone's own",
	} {
		if got := Describe(stored); got != said {
			t.Errorf("%q is described as %q, not %q", stored, got, said)
		}
	}
}
