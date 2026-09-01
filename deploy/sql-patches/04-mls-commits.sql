-- Membership changes and the order they happen in (#40).
--
-- MLS moves a group forward one epoch at a time, and a commit is only valid
-- against the epoch it was made from. Two people adding somebody at the same
-- moment produce two commits from the same epoch, and the protocol can take
-- only one: the other must be told, rebuilt on top of the winner, and sent
-- again. RFC 9420 gives that job to the delivery service, which here is us.
--
-- This comment used to say that here is the one place the server stops being
-- dumb - that it learns a group exists and which epoch it is on, "not who is
-- in it", and that the plan had said so from the start. Both halves were
-- wrong, and they are worth leaving on the record because the code was written
-- against them for weeks. The plan says the opposite in as many words
-- (docs/06-mls-plan.md, "What stays visible to us even when this is done"),
-- and since #147 the server is told a group's membership outright and keeps it
-- in mls_members.
--
-- What it still cannot do is read a word of what is said, tell an addition
-- from a removal, or join a group. docs/08-what-the-server-holds.md is the
-- whole reckoning, including what that invented line cost.

-- Which epoch each conversation is on, so two commits from the same one cannot
-- both be accepted.
--
-- The group id is the one MLS made; the server never chose it and cannot read
-- anything with it.
CREATE TABLE IF NOT EXISTS `mls_groups` (
  `group_id` varbinary(64) NOT NULL,
  -- The epoch the next commit must declare. A commit that names anything else
  -- lost a race and is refused.
  `epoch` bigint(20) NOT NULL,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Commits waiting for a device, in the shape mls_welcomes already has: a
-- conversation is with people, and each of their devices has to be told
-- separately.
--
-- Kept until the device says it has applied one, not merely received it. A
-- commit lost between the two leaves that device an epoch behind, where nothing
-- new can be read - and it surfaces much later, as a group that went quiet for
-- one person.
CREATE TABLE IF NOT EXISTS `mls_commits` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) NOT NULL,
  `auth_key_id` bigint(20) NOT NULL,
  `from_id` bigint(20) NOT NULL,
  `group_id` varbinary(64) NOT NULL,
  -- The epoch this commit was made from. A device applies them in this order
  -- and can tell at a glance whether one is already behind it.
  `epoch` bigint(20) NOT NULL,
  `commit_bytes` blob NOT NULL,
  `date` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_device` (`user_id`, `auth_key_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
