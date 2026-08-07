// Package appconfig — настройки, которыми сервер управляет поведением клиента.
//
// Клиент Telegram спрашивает их методом help.getAppConfig и по ним решает, что
// показывать: платные подписки, звёзды, кошелёк и прочее. В открытой версии
// teamgram метод возвращает пустоту, поэтому клиент рисует всё подряд, включая
// разделы, за которыми у нас ничего нет.
package appconfig

import "github.com/teamgram/proto/mtproto"

// flags — что выключено. Названия ключей задаёт клиент, менять их нельзя.
var flags = map[string]bool{
	// Скрывает разделы «Premium» и «Business» в настройках: платежей у нас нет,
	// и оба раздела ведут в тупик.
	"premium_purchase_blocked": true,
	// Звёзды — внутренняя валюта, тоже про платежи.
	"stars_purchase_blocked": true,
	// Розыгрыши подписок: без подписок бессмысленны.
	"giveaway_gifts_purchase_available": false,
	"premium_gift_attach_menu_icon":     false,
	"premium_gift_text_field_icon":      false,
}

// Value собирает настройки в том виде, в каком их ждёт клиент.
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
