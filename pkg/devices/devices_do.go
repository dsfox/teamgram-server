package devices

import (
	"strconv"
	"strings"
)

// DeviceDO — строка таблицы devices: токен уведомлений одного приложения
// на одном устройстве.
//
// Устройство опознаётся ключом авторизации: он живёт ровно столько же, сколько
// сессия приложения на телефоне, и переживает смену токена уведомлений.
// Поэтому уникальность в таблице — по (auth_key_id, user_id, token_type),
// а сам токен перезаписывается.
//
// Имена в тегах db сверяются с базой гейтом tests/schema_gate.py.
type DeviceDO struct {
	Id           int64  `db:"id" json:"id"`
	AuthKeyId    int64  `db:"auth_key_id" json:"auth_key_id"`
	UserId       int64  `db:"user_id" json:"user_id"`
	TokenType    int32  `db:"token_type" json:"token_type"`
	Token        string `db:"token" json:"token"`
	NoMuted      bool   `db:"no_muted" json:"no_muted"`
	LockedPeriod int32  `db:"locked_period" json:"locked_period"`
	AppSandbox   bool   `db:"app_sandbox" json:"app_sandbox"`
	Secret       string `db:"secret" json:"secret"`
	OtherUids    string `db:"other_uids" json:"other_uids"`
	State        int32  `db:"state" json:"state"`
}

// Типы токенов из account.registerDevice. Нас интересует только APNs:
// приложение под Android мы пока не собираем, а VoIP-токены не запрашиваем.
const (
	TokenTypeAPNs = 1
)

// IsAPNs — можно ли отправить на этот токен уведомление через Apple.
func (d *DeviceDO) IsAPNs() bool {
	return d.TokenType == TokenTypeAPNs && d.Token != ""
}

// JoinUids складывает список учётных записей, вошедших на этом устройстве,
// в одну строку. Приложение присылает его вместе с токеном; нам он пока нужен
// только чтобы ничего не потерять при обратном разборе.
func JoinUids(uids []int64) string {
	parts := make([]string, 0, len(uids))
	for _, uid := range uids {
		parts = append(parts, strconv.FormatInt(uid, 10))
	}

	return strings.Join(parts, ",")
}
