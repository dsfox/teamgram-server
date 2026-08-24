-- Two lookups the schema never indexed, found by counting rows read.
--
-- One run of the direct_messages scenario - two sign-ups and two messages -
-- read 49,117 rows from a database holding 3,205 usernames and 180 contacts.
-- Nearly all of it, 44,868 rows, came from `username`: the table is indexed on
-- the username itself, but the server mostly asks the other way round, "which
-- username belongs to this peer", and that column pair had no index at all.
-- Fourteen full scans per run. `user_contacts` had the matching hole: a unique
-- key starting at owner_user_id serves "who are this person's contacts", and
-- nothing served "who has this person as a contact".
--
-- With both indexes the same run reads 830 rows instead of 49,117.
--
-- This was invisible while a Redis cache sat in front of the DAOs and is worth
-- fixing on its own merits: a scan costs what the table holds, so it is cheap
-- today and grows with every person who signs up. See ice9 #7.

-- "Which username belongs to this peer" - the shape behind
-- select ... from username where peer_type = ? and peer_id = ?
ALTER TABLE `username`
  ADD KEY `idx_peer` (`peer_type`, `peer_id`);

-- "Who has this person in their contacts" - the reverse of the unique key,
-- used when somebody joins and the people who know them are told.
ALTER TABLE `user_contacts`
  ADD KEY `idx_contact` (`contact_user_id`);
