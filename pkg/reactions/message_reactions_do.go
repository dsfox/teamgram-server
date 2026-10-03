package reactions

// MessageReactionsDO is one person's reaction to one message, as the
// message_reactions table holds it (#18). The file is named after the table
// because that is how tests/schema_gate.py finds the columns the code expects.
//
// A message is named by its dialog_message_id, the one id every copy of it
// shares whoever's box the copy sits in, so a reaction is written once and
// read by everybody in the conversation.
type MessageReactionsDO struct {
	DialogMessageId int64  `db:"dialog_message_id"`
	UserId          int64  `db:"user_id"`
	Reaction        string `db:"reaction"`
	ReactedAt       int32  `db:"reacted_at"`
}
