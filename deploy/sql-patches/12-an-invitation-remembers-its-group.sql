-- A code minted from a group's add-members picker remembers the group, and
-- sign-up puts the person into it (#164). Zero for every code minted before,
-- and for a code minted from the Contacts tab: ice9 alone.
--
-- apply-patches.sh records each file once, so a plain ALTER is safe on the
-- live server; the stand's database is created from scratch with every patch.
ALTER TABLE `invitations` ADD COLUMN `chat_id` bigint(20) NOT NULL DEFAULT '0';
