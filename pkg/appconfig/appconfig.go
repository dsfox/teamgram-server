// Package appconfig holds the settings by which the server steers client behaviour.
//
// The Telegram client asks for them via help.getAppConfig and decides what to
// show: paid subscriptions, stars, the wallet and so on. In the open teamgram
// build the method returns nothing, so the client draws everything, including
// sections with nothing behind them.
package appconfig

import "github.com/teamgram/proto/mtproto"

// flags lists what is switched off. The client dictates the key names; they must not change.
var flags = map[string]bool{
	// Hides the "Premium" and "Business" sections in settings: we have no
	// payments and both sections lead nowhere.
	"premium_purchase_blocked": true,
	// Stars are an internal currency, also about payments.
	"stars_purchase_blocked": true,
	// Subscription giveaways: pointless without subscriptions.
	"giveaway_gifts_purchase_available": false,
	"premium_gift_attach_menu_icon":     false,
	"premium_gift_text_field_icon":      false,
}

// Value assembles the settings in the shape the client expects.
func Value() *mtproto.JSONValue {
	values := make([]*mtproto.JSONObjectValue, 0, len(flags))
	for key, enabled := range flags {
		values = append(values, mtproto.MakeTLJsonObjectValue(&mtproto.JSONObjectValue{
			Key:   key,
			Value: mtproto.MakeTLJsonBool(&mtproto.JSONValue{
				Value_BOOL: mtproto.ToBool(enabled),
			}).To_JSONValue(),
		}).To_JSONObjectValue())
	}

	return mtproto.MakeTLJsonObject(&mtproto.JSONValue{
		Value_VECTORJSONOBJECTVALUE: values,
	}).To_JSONValue()
}
