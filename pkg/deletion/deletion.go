// Package deletion removes a person's copies of messages for good (#220).
//
// Upstream only marked a deleted message (deleted = 1): the row stayed, with
// its words or ciphertext, and so did the copy of it in user_pts_updates that
// getDifference replays, and every reaction to it. The privacy policy says a
// message is kept "until you or the person you wrote to delete them", and the
// nightly backup inherits whatever the database holds.
//
// Every way of deleting a message - for oneself, for everyone, a cleared
// history, the call list - ends in the messages store's delete, which calls
// Forget. It does three things, in this order:
//
//  1. the user's copies in user_pts_updates lose their words, file and
//     entities. The rows stay: getDifference reads pts in order, and a gap
//     would be a hole in every phone's sequence;
//  2. reactions to a message go once no copy of it is left in anybody's box;
//  3. the rows go.
//
// Files a deleted message carried are left to cmd/forgetdeleted, which removes
// those nothing in the database names any more: a file is shared by forwards,
// profiles and chats, and finding out it is free takes the whole database.
package deletion

import (
	"database/sql"
	"fmt"
	"strings"
)

// Exec runs one statement, in a transaction or not - whichever the caller has.
type Exec func(query string, args ...interface{}) (sql.Result, error)

// The update kinds that carry a whole message.
const carriesMessage = "json_unquote(json_extract(update_data, '$.predicate_name')) in ('updateNewMessage', 'updateEditMessage')"

// What a blanked copy keeps: everything but the words, the file and the
// entities. json_replace, not json_set: a service message has no words, and
// must not be given an empty text it never had.
const Blanked = "json_remove(json_replace(update_data, '$.message_MESSAGE.message', ''), " +
	"'$.message_MESSAGE.media', '$.message_MESSAGE.entities', '$.message_MESSAGE.reply_to.quote_text')"

func idList(ids []int32) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprint(id)
	}
	return strings.Join(parts, ", ")
}

// BlankUpdates is the statement for step 1; it takes the user id.
func BlankUpdates(ids []int32) string {
	return "update user_pts_updates set update_data = " + Blanked +
		" where user_id = ? and " + carriesMessage +
		" and cast(json_extract(update_data, '$.message_MESSAGE.id') as signed) in (" + idList(ids) + ")"
}

// DropReactions is the statement for step 2; it takes the user id twice.
// Copies already marked deleted by an older server count as gone.
func DropReactions(table string, ids []int32) string {
	in := idList(ids)
	return "delete from message_reactions where dialog_message_id in (" +
		"select gone.dialog_message_id from (" +
		"select dialog_message_id from " + table + " where user_id = ? and user_message_box_id in (" + in + ")" +
		") gone where not exists (" +
		"select 1 from " + table + " other where other.dialog_message_id = gone.dialog_message_id" +
		" and other.deleted = 0 and not (other.user_id = ? and other.user_message_box_id in (" + in + "))))"
}

// DropRows is the statement for step 3; it takes the user id. Rows an older
// server only marked go as well.
func DropRows(table string, ids []int32) string {
	return "delete from " + table + " where user_id = ? and user_message_box_id in (" + idList(ids) + ")"
}

// Forget removes this user's copies of these messages, and reports how many
// rows went.
func Forget(exec Exec, table string, userId int64, ids []int32) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if _, err := exec(BlankUpdates(ids), userId); err != nil {
		return 0, fmt.Errorf("cannot blank the copies in user_pts_updates: %w", err)
	}
	if _, err := exec(DropReactions(table, ids), userId, userId); err != nil {
		return 0, fmt.Errorf("cannot drop the reactions: %w", err)
	}
	result, err := exec(DropRows(table, ids), userId)
	if err != nil {
		return 0, fmt.Errorf("cannot drop the rows: %w", err)
	}
	return result.RowsAffected()
}
