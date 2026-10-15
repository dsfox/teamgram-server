// Package notifysound is how a chat's sound choice lives on the server:
// written into user_notify_settings.sound as one short string, read back as
// the NotificationSound the clients have spoken since layer 139, and told to
// the push (#282).
//
// One value serves every platform. ice9's people carry one phone per
// account, and a sound only Android knows ("local", the phone's own file)
// reads as the default on an iPhone.
package notifysound

import (
	"strconv"
	"strings"

	"github.com/teamgram/proto/mtproto"
)

const (
	Default = "default"
	None    = "none"
	// A tone the server hands out (pkg/tones): "ringtone:<document id>".
	ringtonePrefix = "ringtone:"
	// The phone's own sound: "local:<title>\t<data>", Android only.
	localPrefix = "local:"
	// The column is varchar(255); anything longer is not worth keeping.
	longest = 255
)

// Encode is the string for the sound a client sent. Nil is the default.
func Encode(sound *mtproto.NotificationSound) string {
	if sound == nil {
		return Default
	}
	encoded := Default
	switch sound.GetPredicateName() {
	case mtproto.Predicate_notificationSoundNone:
		encoded = None
	case mtproto.Predicate_notificationSoundRingtone:
		encoded = ringtonePrefix + strconv.FormatInt(sound.GetId(), 10)
	case mtproto.Predicate_notificationSoundLocal:
		encoded = localPrefix + sound.GetTitle() + "\t" + sound.GetData()
	}
	if len(encoded) > longest {
		return Default
	}
	return encoded
}

// Decode is the sound for a stored string. Anything unknown, the "-1" the
// table keeps for "not set" included, is the default.
func Decode(stored string) *mtproto.NotificationSound {
	switch {
	case stored == None:
		return mtproto.MakeTLNotificationSoundNone(nil).To_NotificationSound()
	case strings.HasPrefix(stored, ringtonePrefix):
		if id, ok := RingtoneId(stored); ok {
			return mtproto.MakeTLNotificationSoundRingtone(&mtproto.NotificationSound{Id: id}).To_NotificationSound()
		}
	case strings.HasPrefix(stored, localPrefix):
		title, data, _ := strings.Cut(strings.TrimPrefix(stored, localPrefix), "\t")
		return mtproto.MakeTLNotificationSoundLocal(&mtproto.NotificationSound{Title: title, Data: data}).To_NotificationSound()
	}
	return mtproto.MakeTLNotificationSoundDefault(nil).To_NotificationSound()
}

// RingtoneId is the document id when the stored sound is a tone of ours.
func RingtoneId(stored string) (int64, bool) {
	if !strings.HasPrefix(stored, ringtonePrefix) {
		return 0, false
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(stored, ringtonePrefix), 10, 64)
	return id, err == nil && id != 0
}

// IsChosen says whether a stored string is a choice at all: the default,
// an empty cell and the table's "-1" all mean "as the kind says".
func IsChosen(stored string) bool {
	return stored != "" && stored != "-1" && stored != Default
}

// Apply writes the same stored choice into a peer's settings for every
// platform, in the shape the clients read.
func Apply(settings *mtproto.PeerNotifySettings, stored string) {
	settings.IosSound = Decode(stored)
	settings.AndroidSound = Decode(stored)
	settings.OtherSound = Decode(stored)
}

// ForApple is what the envelope's aps carries so the iPhone's extension
// plays the choice: Apple's default sound; no sound at all; or, for a tone,
// the id under "ringtone" with the extension's own fallback file as the
// sound (what the extension expects, NotificationService.swift).
func ForApple(stored string) (sound string, ringtone string) {
	if stored == None {
		return "", ""
	}
	if id, ok := RingtoneId(stored); ok {
		return "0.m4a", strconv.FormatInt(id, 10)
	}
	return Default, ""
}

// Describe is the choice as a log line says it.
func Describe(stored string) string {
	if !IsChosen(stored) {
		return Default
	}
	if id, ok := RingtoneId(stored); ok {
		return "ringtone " + strconv.FormatInt(id, 10)
	}
	if strings.HasPrefix(stored, localPrefix) {
		return "the phone's own"
	}
	return stored
}
