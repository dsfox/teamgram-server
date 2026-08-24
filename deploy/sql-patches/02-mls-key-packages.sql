-- The directory of MLS key packages (#38).
--
-- A key package is what somebody needs in order to add a device to an encrypted
-- conversation. Every device publishes a supply of them; whoever starts a
-- conversation takes one per device of the other side. They are meant to be used
-- once - reusing one costs the forward secrecy of the messages that follow - so
-- taking a package removes it.
--
-- The server understands none of this. It stores opaque bytes, hands them out in
-- the order they arrived, and counts what is left so a device knows when to
-- publish more. It cannot read them, and cannot tell a real one from a forged
-- one either: that is what key transparency (#44) is for, and until it exists
-- this table is the weak point of the whole scheme rather than a detail.

CREATE TABLE IF NOT EXISTS `mls_key_packages` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) NOT NULL,
  -- Which device published it. Per device, not per person: several devices of
  -- one person are several members of the same conversation.
  `auth_key_id` bigint(20) NOT NULL,
  `key_package` blob NOT NULL,
  -- A hash of the bytes, so the same package cannot be published twice. The
  -- server does not understand the package, but it can tell two of them apart.
  `fingerprint` char(64) NOT NULL,
  -- A last-resort package is handed out when the supply has run dry, rather
  -- than refusing to start a conversation. It is reused, which is why it is
  -- marked: a device that keeps serving one has stopped publishing and that is
  -- worth noticing.
  `last_resort` tinyint(1) NOT NULL DEFAULT '0',
  `date` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_device` (`user_id`, `auth_key_id`),
  -- The same bytes must not be published twice: a duplicate would be handed to
  -- two conversations and used twice.
  UNIQUE KEY `uk_fingerprint` (`user_id`, `auth_key_id`, `fingerprint`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
