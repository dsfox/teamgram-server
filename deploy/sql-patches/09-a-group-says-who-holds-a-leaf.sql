-- Who holds a leaf in each conversation, as the committer says (#147).
--
-- The code held a stricter line than the plan: the server was not to know who
-- is in a group. That bought no privacy - chat_participants, mls_devices and
-- mls_key_packages are already here, the group id travels in the clear in every
-- message header, and on every commit the server already expands a list of
-- people into a list of devices to decide which mailboxes to fill. What it cost
-- was five issues' worth of machinery to work around not knowing.
--
-- Learned from the one party that knows for certain: the device that made the
-- commit. It is that device's own tree and it has just changed it. The roster
-- arrives whole rather than as a difference, so any commit repairs it, an
-- addition and a removal are the same operation, and a server that missed one
-- is corrected by the next rather than staying wrong for ever.
--
-- `leaf` is the identity exactly as MLS carries it, which in this fork is the
-- bytes of <user_id>/<device_id>. The part before the slash is the only thing
-- parsed out of it, and it is kept in its own column so that asking "who is in
-- this group" is an index lookup rather than a scan with a string function.
--
-- Note what is *not* here: auth_key_id. The plan named it, and the leaf does
-- not carry it - the part after the slash is the identity's own device number,
-- chosen by the client, not an auth key. The mapping from a leaf to a device
-- lives in mls_key_packages.name, which is where step 3 will read it.
create table if not exists mls_members (
    group_id  varbinary(64)  not null,
    leaf      varbinary(255) not null,
    user_id   bigint         not null,
    -- The epoch this roster was reported at. A commit that arrives after a
    -- newer one has already been recorded is ignored rather than allowed to
    -- put the group back the way it was.
    epoch     bigint         not null,
    joined_at int            not null,
    seen_at   int            not null,
    primary key (group_id, leaf),
    key idx_group_user (group_id, user_id),
    key idx_user (user_id)
) engine = innodb default charset = utf8mb4;
