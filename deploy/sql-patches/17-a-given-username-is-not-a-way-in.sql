-- The usernames the server handed out rather than the person chose (#239).
--
-- Every account gets a username at sign-up, made from the name, so that a
-- mention in a group reads @eduard for everybody instead of whatever the one
-- mentioning him had saved him as. A name made that way is easy to guess, and
-- a username is the one way a stranger finds somebody here (#181). So a given
-- one does not count as a way in: search skips it, and resolving it answers
-- only the account itself, its contacts and the people in a group with it.
-- Once the person chooses a username themselves, its row here is dropped.
--
-- Keyed on the pair: a name given out, then freed and chosen by somebody else,
-- is not theirs to hide.
create table if not exists username_generated (
    user_id    bigint      not null,
    username   varchar(32) not null,
    created_at int         not null,
    primary key (user_id, username)
) engine = InnoDB default charset = utf8mb4 collate = utf8mb4_unicode_ci;
