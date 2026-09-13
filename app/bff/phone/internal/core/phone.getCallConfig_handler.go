package core

import (
	"github.com/teamgram/proto/mtproto"
)

// PhoneGetCallConfig is the engine tuning both phones poll every twelve hours.
//
// phone.getCallConfig = DataJSON;
//
// An empty object is the whole answer today. Android hands the text to the
// legacy libtgvoip ServerConfig and reads a handful of keys with defaults
// (Instance.ServerConfig: use_system_ns, use_system_aec, the codec switches);
// iOS carries it into OngoingCallContext as serializedData and reads nothing
// from it. Nothing the engines look for lacks a default, so there is nothing to
// say yet - and this is the same text the stub layer answered before the route
// existed, so routing changes no phone's behaviour.
func (c *PhoneCore) PhoneGetCallConfig(in *mtproto.TLPhoneGetCallConfig) (*mtproto.DataJSON, error) {
	return mtproto.MakeTLDataJSON(&mtproto.DataJSON{Data: "{}"}).To_DataJSON(), nil
}
