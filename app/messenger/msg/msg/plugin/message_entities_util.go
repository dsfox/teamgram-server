// Copyright 2024 Teamgram Authors
//  All rights reserved.
//
// Author: Benqi (wubenqi@gmail.com)
//

package plugin

import (
	"context"
	"github.com/teamgram/teamgram-server/app/service/biz/user/user"
	"sort"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/mention"

	"mvdan.cc/xurls/v2"
)

// markedAlready reports whether a mention already starts at this offset.
//
// Only mentions, and only the offset: two marks of different kinds over one
// word are ordinary - bold over a mention is a thing people write - and it is
// the second mention over the first that has no meaning.
func markedAlready(entities mtproto.MessageEntitySlice, offset int32) bool {
	for _, entity := range entities {
		if entity.Offset != offset {
			continue
		}
		switch entity.PredicateName {
		case mtproto.Predicate_messageEntityMention,
			mtproto.Predicate_messageEntityMentionName:
			return true
		}
	}
	return false
}

// urlMarkedAlready reports whether a link already starts at this offset - as a
// plain url or as text with a link under it. The server's own scan runs over
// the whole text whatever the client marked, and a link stored twice arrived
// as two identical entities over one word.
func urlMarkedAlready(entities mtproto.MessageEntitySlice, offset int32) bool {
	for _, entity := range entities {
		if entity.Offset != offset {
			continue
		}
		switch entity.PredicateName {
		case mtproto.Predicate_messageEntityUrl,
			mtproto.Predicate_messageEntityTextUrl:
			return true
		}
	}
	return false
}

// usernameLen is how much of the tag is actually a username: letters, digits
// and underscore. The tag scanner runs to the next space, so "@dsfox," arrives
// here with the comma attached - and a mention that includes punctuation
// resolves a username nobody has.
func usernameLen(tag string) int {
	for i := 0; i < len(tag); i++ {
		c := tag[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' {
			continue
		}
		return i
	}
	return len(tag)
}

func RemakeMessage(ctx context.Context, plugin MsgPlugin, message *mtproto.Message, fromId int64, noWebpage bool, hasBot func() bool) *mtproto.Message {
	var (
		entities mtproto.MessageEntitySlice
		idxList  []int
	)

	/*
		fmt.Println(*update.ChannelPost.Entities)
		// For each entity
		for _, e := range *update.ChannelPost.Entities {
			// Get the whole update Text
			str := update.ChannelPost.Text
			// Encode it into utf16
			utfEncodedString := utf16.Encode([]rune(str))
			// Decode just the piece of string I need
			runeString := utf16.Decode(utfEncodedString[e.Offset : e.Offset+e.Length])
			// Transform []rune into string
			str = string(runeString)
			fmt.Println(str)
		}
	*/
	getIdxList := func() []int {
		if len(idxList) == 0 {
			idxList = mention.EncodeStringToUTF16Index(message.Message)
		}
		return idxList
	}

	// The client's own marks first: they are what the guards below compare
	// against, and a guard that runs before the list is filled guards nothing.
	for _, entity := range message.Entities {
		switch entity.PredicateName {
		case mtproto.Predicate_inputMessageEntityMentionName:
			if entity.GetUserId_INPUTUSER().GetPredicateName() == mtproto.Predicate_inputUserSelf {
				entity.UserId_INPUTUSER.UserId = fromId
			}

			if entity.UserId_INPUTUSER.UserId != 0 {
				// TODO(@benqi): check user_id
				entityMentionName := mtproto.MakeTLMessageEntityMentionName(&mtproto.MessageEntity{
					Offset:       entity.Offset,
					Length:       entity.Length,
					UserId_INT64: entity.UserId_INPUTUSER.UserId,
				})

				entities = append(entities, entityMentionName.To_MessageEntity())
			}
			// }
		default:
			entities = append(entities, entity)
		}
	}

	var firstUrl string
	rIndexes := xurls.Relaxed().FindAllStringIndex(message.Message, -1)
	if len(rIndexes) > 0 {
		if len(idxList) == 0 {
			getIdxList()
		}
		for idx, v := range rIndexes {
			if idx == 0 {
				firstUrl = message.Message[v[0]:v[1]]
			}
			// The client may have marked this link itself, and most do. Adding
			// ours beside it stored the same link twice.
			if urlMarkedAlready(entities, int32(idxList[v[0]])) {
				continue
			}
			entityUrl := mtproto.MakeTLMessageEntityUrl(&mtproto.MessageEntity{
				Offset: int32(idxList[v[0]]),
				Length: int32(idxList[v[1]] - idxList[v[0]]),
			}).To_MessageEntity()
			entities = append(entities, entityUrl)
		}
	}

	if !noWebpage && firstUrl != "" && plugin != nil {
		webpage, _ := plugin.GetWebpagePreview(ctx, firstUrl)
		if webpage != nil {
			message.Media = mtproto.MakeTLMessageMediaWebPage(&mtproto.MessageMedia{
				Webpage: webpage,
			}).To_MessageMedia()
		}
	}

	tags := mention.GetTags('@', message.Message, '(', ')')
	if len(tags) > 0 {
		for _, tag := range tags {
			// The scanner runs a tag to the next space, so "@dsfox," comes
			// back with the comma attached - marked, it is a mention of a
			// username nobody has. The mention stops at the word.
			name := tag.Tag[:usernameLen(tag.Tag)]
			if name == "" {
				continue
			}
			if len(idxList) == 0 {
				getIdxList()
			}
			mention2 := mtproto.MakeTLMessageEntityMention(&mtproto.MessageEntity{
				Offset: int32(idxList[tag.Index]),
				Length: int32(idxList[tag.Index+len(name)+1] - idxList[tag.Index]),
			}).To_MessageEntity()

			// stole field UserId_5
			if plugin != nil {
				if v, _ := plugin.UsernameResolveUsername(ctx, &user.TLUserResolveUsername{
					Username: name,
				}); v != nil {
					if v.GetPredicateName() == mtproto.Predicate_peerUser {
						mention2.UserId_INT64 = v.UserId
					}
				}
			}
			//for _, v := range names.Datas {
			//	//
			//	if uname, ok := names[tag.Tag]; ok {
			//		if uname.PeerType == model.PEER_USER {
			//			mention2.UserId_INT32 = uname.PeerId
			//		}
			//	}
			//}
			// The client may have marked this mention itself, and most do. Its
			// own entity is already in the list above, so adding ours beside it
			// stores the mention twice - two marks over the same word, and the
			// second one a byte longer or shorter whenever the two count the
			// "@" differently. Found by the scenario that sends an @mention and
			// reads back what arrived.
			if !markedAlready(entities, mention2.Offset) {
				entities = append(entities, mention2)
			}
		}
	}

	tags = mention.GetTags('#', message.Message)
	for _, tag := range tags {
		if len(idxList) == 0 {
			getIdxList()
		}
		hashtag := mtproto.MakeTLMessageEntityHashtag(&mtproto.MessageEntity{
			Offset: int32(idxList[tag.Index]),
			Length: int32(idxList[tag.Index+len(tag.Tag)+1] - idxList[tag.Index]),
			Url:    "#" + tag.Tag, // NOTE: hack, steal url field
		}).To_MessageEntity()
		entities = append(entities, hashtag)
	}

	if hasBot != nil && hasBot() {
		tags = mention.GetTags('/', message.Message)
		for _, tag := range tags {
			if len(idxList) == 0 {
				getIdxList()
			}
			hashtag := mtproto.MakeTLMessageEntityBotCommand(&mtproto.MessageEntity{
				Offset: int32(idxList[tag.Index]),
				Length: int32(idxList[tag.Index+len(tag.Tag)+1] - idxList[tag.Index]),
			}).To_MessageEntity()
			entities = append(entities, hashtag)
		}
	}

	sort.Sort(entities)
	message.Entities = entities
	return message
}
