-- A welcome now says which chat it is for. Without it the device joining files
-- the conversation under whoever sent the invitation, so a group is recorded as
-- the conversation with the person who invited them - and a private message to
-- that person is written with the group's keys (#115).
--
-- Zero for anything already waiting, which is what the clients treat as "not
-- said" and fall back to the old guess for.
alter table mls_welcomes add column peer_id bigint not null default 0 after from_id;
