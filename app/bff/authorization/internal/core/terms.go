package core

import (
	"strings"
	"unicode/utf16"

	"github.com/teamgram/proto/mtproto"
)

// ice9: what a new number is shown before it signs up. Apple's guideline 1.2
// asks that people agree to terms saying objectionable content and abusive
// users are not tolerated. The client puts "By signing up, you agree to the
// Terms of Service" on the name screen and opens this text on a tap.
//
// One text in both languages, because auth.signIn does not know the client's.

const termsURL = "https://ice9.app/terms"

const signUpTermsText = "By signing up you accept the terms at " + termsURL + ". " +
	"We have zero tolerance for objectionable content and abusive users; " +
	"anyone can be blocked, and reported, from their profile.\n\n" +
	"Регистрируясь, вы принимаете условия на " + termsURL + ". " +
	"Мы не терпим неприемлемого контента и оскорбительного поведения; " +
	"любого пользователя можно заблокировать и пожаловаться на него из его профиля."

// SignUpTerms is the terms of service a sign-up answer carries.
func SignUpTerms() *mtproto.Help_TermsOfService {
	return mtproto.MakeTLHelpTermsOfService(&mtproto.Help_TermsOfService{
		Popup:    false,
		Id:       mtproto.MakeTLDataJSON(&mtproto.DataJSON{Data: `"ice9-terms-2026-09-27"`}).To_DataJSON(),
		Text:     signUpTermsText,
		Entities: urlEntities(signUpTermsText, termsURL),
	}).To_Help_TermsOfService()
}

// urlEntities marks every occurrence of url in text as a link. Offsets and
// lengths are UTF-16 code units, which is what the protocol counts - the
// Russian half is two bytes a letter in UTF-8 and one unit in UTF-16.
func urlEntities(text, url string) []*mtproto.MessageEntity {
	var entities []*mtproto.MessageEntity
	length := int32(len(utf16.Encode([]rune(url))))
	for from := 0; ; {
		at := strings.Index(text[from:], url)
		if at < 0 {
			return entities
		}
		offset := int32(len(utf16.Encode([]rune(text[:from+at]))))
		entities = append(entities, mtproto.MakeTLMessageEntityUrl(&mtproto.MessageEntity{
			Offset: offset,
			Length: length,
		}).To_MessageEntity())
		from += at + len(url)
	}
}
