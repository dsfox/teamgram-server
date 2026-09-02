-- Who brought whom (#47). A record and nothing more: the owner reads it when
-- there is something to look into; nothing in the product acts on it.
--
-- In MySQL rather than beside the code in the key-value store, because the
-- record has to outlive the code - a code lives seven days, the record for
-- ever. Dates are Unix seconds like the rest of the schema.
--
-- `code` alone is not unique across time: codes are six digits and a spent one
-- can be minted again later. The pair with `minted_at` is.
CREATE TABLE IF NOT EXISTS `invitations` (
  `code`        varchar(16) COLLATE utf8mb4_bin NOT NULL,
  `inviter_id`  bigint(20) NOT NULL,
  `phone`       varchar(32) COLLATE utf8mb4_bin NOT NULL,
  `minted_at`   int(11) NOT NULL,
  `redeemed_at` int(11) NOT NULL DEFAULT '0',
  `invitee_id`  bigint(20) NOT NULL DEFAULT '0',
  PRIMARY KEY (`code`, `minted_at`),
  KEY `by_phone` (`phone`, `redeemed_at`),
  KEY `by_inviter` (`inviter_id`, `phone`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
