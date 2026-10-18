-- Which device each leaf name belongs to, kept after the device has gone (#263).
--
-- The server matched a leaf to its device through the name the device
-- published its key packages under (#136), and nowhere else. Signing out
-- deletes those packages (ForgetDevice), and with them the only record of
-- whose the leaf was: the leaf of a signed-out - possibly stolen - phone then
-- matched nothing, a leaf that matches nothing is called alive on purpose, and
-- it stayed in every conversation with every commit encrypted to it.
--
-- So the name is written down once, here, and outlives the packages. A row is
-- written only for a name that carries the publisher's own user id.
create table if not exists mls_device_names (
    name        varbinary(255) not null,
    user_id     bigint         not null,
    auth_key_id bigint         not null,
    named_at    int            not null,
    primary key (name),
    key idx_device (user_id, auth_key_id)
) engine = innodb default charset = utf8mb4;

-- What the packages still say, for the devices that published before this.
insert ignore into mls_device_names (name, user_id, auth_key_id, named_at)
select name, user_id, auth_key_id, max(date) from mls_key_packages
where name <> '' group by name, user_id, auth_key_id;
