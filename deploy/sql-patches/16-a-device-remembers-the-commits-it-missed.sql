-- The newest commit of each group that a device will never get (#211).
--
-- A device misses a commit two ways: it waits in its box longer than a commit
-- lives and is forgotten when the device asks, or it is never put there because
-- the device had not been seen for a fortnight. Either way that device can no
-- longer reach the group's epoch: everything after the gap needs what is in it.
--
-- Nothing said so before. A phone off for three weeks came back, was answering
-- again, and so its leaf counted as alive: nobody had a reason to take it out
-- and let it back in, and it looped on repair until somebody did it by hand.
-- With this, a leaf whose device missed a commit after the leaf joined is dead
-- to the comparison every client already runs, which takes it out and lets the
-- device in afresh.
--
-- `missed_at` is when the missed change was made, not when the miss was noticed:
-- a commit forgotten long after the device was let back in says nothing about
-- the leaf it has now, whose joined_at is later.
create table if not exists mls_missed (
    group_id    varbinary(64) not null,
    user_id     bigint        not null,
    auth_key_id bigint        not null,
    missed_at   int           not null,
    primary key (group_id, user_id, auth_key_id),
    -- A device that signs out is forgotten by user and device (ForgetDevice).
    key idx_device (user_id, auth_key_id)
) engine = innodb default charset = utf8mb4;
