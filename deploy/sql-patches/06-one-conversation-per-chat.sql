-- Which conversation belongs to which chat, decided once and by the server.
--
-- Nothing decided it before, and every device that wanted to send into a chat
-- with no conversation started one of its own. Between two people that almost
-- always settles on one; in a group where three people begin within a minute it
-- does not, and three people ended up in two conversations that cannot read each
-- other, with no way back (#135).
--
-- Devices cannot settle it among themselves: the ones that lose have to be told
-- by somebody, and when everybody is offline and arrives in a random order there
-- is nobody to tell them. So the first claim wins and the primary key is what
-- makes that atomic.
--
-- This tells the server which chat a conversation belongs to, which it was
-- deliberately not told before. It learns nothing by it: the group id travels in
-- the clear in the header of every message, so the mapping could always be read
-- off any one of them.
CREATE TABLE IF NOT EXISTS `mls_conversations` (
  `peer_id` bigint(20) NOT NULL COMMENT 'the dialog: negative for a chat, positive for a person',
  `group_id` varbinary(64) NOT NULL,
  `date` int(11) NOT NULL,
  PRIMARY KEY (`peer_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
