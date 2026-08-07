package devices

import (
	"strconv"
	"strings"
)

// DeviceDO is a row of the devices table: the notification token of one app on
// one device.
//
// A device is identified by its authorization key: the key lives exactly as long
// as the app session on the phone and survives notification token changes. Hence
// the table is unique on (auth_key_id, user_id, token_type) while the token
// itself is overwritten.
//
// The names in the db tags are checked against the database by
// tests/schema_gate.py.
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

// Token types from account.registerDevice. Only APNs matters to us: we do not
// build an Android app yet and we do not request VoIP tokens.
const (
	TokenTypeAPNs = 1
)

// IsAPNs reports whether a notification can be sent to this token through Apple.
func (d *DeviceDO) IsAPNs() bool {
	return d.TokenType == TokenTypeAPNs && d.Token != ""
}

// JoinUids packs the list of accounts signed in on this device into a single
// string. The app sends it along with the token; for now we keep it only so that
// nothing is lost when the record is read back.
func JoinUids(uids []int64) string {
	parts := make([]string, 0, len(uids))
	for _, uid := range uids {
		parts = append(parts, strconv.FormatInt(uid, 10))
	}

	return strings.Join(parts, ",")
}
