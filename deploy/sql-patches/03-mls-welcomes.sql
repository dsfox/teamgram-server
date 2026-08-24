-- Welcomes waiting for a device (#39).
--
-- A welcome is what lets a device into a conversation somebody started with it.
-- It waits here rather than travelling as a message, so no client has to hide
-- anything from a chat list.
--
-- It is kept until the device says the conversation is open and saved, not
-- merely delivered. A welcome lost between the two is a conversation that
-- exists on one side and not the other, and that shows up much later as
-- messages which will not open - far from the moment it broke.

CREATE TABLE IF NOT EXISTS `mls_welcomes` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  -- Whose device it is for. Every device of that person gets its own row: a
  -- conversation is with a person, and each of their devices is a member.
  `user_id` bigint(20) NOT NULL,
  `auth_key_id` bigint(20) NOT NULL,
  `from_id` bigint(20) NOT NULL,
  `welcome` blob NOT NULL,
  `date` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_device` (`user_id`, `auth_key_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
