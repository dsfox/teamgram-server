-- When a device last asked the server anything about encryption (#138).
--
-- A device that reinstalls comes back with a new auth key, and the old row in
-- auth_users stays as it was: nobody logged it out, so by the record it is a
-- live device for ever. Every conversation started afterwards took a key
-- package for it and added a leaf that can never read, and the person's own
-- phone was told it has a device it does not have.
--
-- There was no signal to tell the two apart. auth_users.date_active is never
-- written; auths.date_active is written when a session announces itself and not
-- again - the phone running while this was written last said so the evening
-- before. What is left is the one thing only a real device does: every round it
-- asks how many devices this account has, which reaches the server as a publish
-- with nothing in it. That is what is written here.
--
-- Seeded from the newest key package each device published, because that is the
-- last time we know it was there. A device that is running writes a fresh time
-- within half a minute of this landing.
create table if not exists mls_devices (
    user_id     bigint      not null,
    auth_key_id bigint      not null,
    last_seen   int         not null,
    primary key (user_id, auth_key_id)
) engine = innodb default charset = utf8mb4;

insert ignore into mls_devices (user_id, auth_key_id, last_seen)
select user_id, auth_key_id, max(date) from mls_key_packages group by user_id, auth_key_id;
