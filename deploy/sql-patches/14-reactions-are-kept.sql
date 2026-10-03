-- Reactions to messages (#18): who reacted to which message with what.
--
-- One row per person per message - a person has one reaction on a message, and
-- giving another replaces it. A message is named by its dialog_message_id, the
-- id every copy of it shares whoever's box the copy sits in, so a reaction is
-- written once and every member of the conversation reads the same row.
--
-- Kept in the open, beside the encrypted message it answers: the owner decided
-- so on 3 October. docs/08-what-the-server-holds.md says what that means.
CREATE TABLE IF NOT EXISTS message_reactions (
  dialog_message_id bigint NOT NULL,
  user_id bigint NOT NULL,
  reaction varchar(16) NOT NULL,
  reacted_at int NOT NULL,
  PRIMARY KEY (dialog_message_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

-- A reaction travels to every box the message is in: "the copies of this
-- message in these people's boxes", which nothing indexed - every reaction
-- would read the whole messages table.
ALTER TABLE `messages`
  ADD KEY `idx_dialog_message` (`user_id`, `dialog_message_id`);
