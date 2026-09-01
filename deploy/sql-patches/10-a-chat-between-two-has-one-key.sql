-- A chat between two people is one chat, and now it has one row.
--
-- The conversation of a chat was written down under peer_id, which is the
-- dialog as the device asking happens to see it. For a group that is the chat
-- itself and the same number on every device. For a chat between two it is the
-- *other person*, so the two sides name one chat by two numbers and the server
-- cannot see that they mean the same thing: alpha claimed 136908639 and delta
-- claimed 136908607, both were first, both won, and the two of them talked in
-- conversations that could not read each other for ever (#155).
--
-- So a chat between two is keyed by both of its people, smallest first, and a
-- group by its dialog id with nothing beside it. The primary key on the pair is
-- what makes the first claim win atomically, exactly as it did before.
--
-- The rows already here for chats between two cannot be repaired: a row says
-- which person the chat was with and not who was asking, so the pair it stood
-- for cannot be worked out from it. They are also unreachable from now on -
-- nothing will ever look under with_id = 0 for a positive peer_id again - so
-- they go. A chat whose row is dropped is claimed afresh by the next device
-- that needs one, which is the ordinary path and not a special case.
--
-- Groups keep their rows untouched: peer_id is negative, with_id stays zero,
-- and the key they had is the key they get.
ALTER TABLE `mls_conversations`
  ADD COLUMN `with_id` bigint(20) NOT NULL DEFAULT 0
  COMMENT 'the other person in a chat between two - the pair is (smaller, larger); zero for a group'
  AFTER `peer_id`;

DELETE FROM `mls_conversations` WHERE `peer_id` > 0 AND `with_id` = 0;

ALTER TABLE `mls_conversations`
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (`peer_id`, `with_id`);
