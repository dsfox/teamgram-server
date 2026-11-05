-- The key log (#44): which signature key belongs to which device, in a log
-- that can only grow (RFC 6962's tree, golang.org/x/mod/sumdb/tlog).
--
-- A phone took on the server's word which key was whose, so a server could
-- hand out a key package built on a key of its own under a real device's name.
-- Every key handed out is logged first; phones check that a key is in the log
-- and that the log they see holds the one they saw before.
--
-- Nothing here is ever updated or deleted except the size, which only grows.
create table if not exists mls_key_log (
    idx           bigint         not null,
    user_id       bigint         not null,
    name          varbinary(255) not null,
    signature_key varbinary(255) not null,
    date          int            not null,
    primary key (idx),
    unique key uk_name_key (name, signature_key),
    key idx_user (user_id)
) engine = innodb default charset = utf8mb4;

-- The tree's stored hashes in tlog's own numbering (StoredHashIndex).
create table if not exists mls_key_log_hashes (
    idx  bigint     not null,
    hash binary(32) not null,
    primary key (idx)
) engine = innodb default charset = utf8mb4;

-- One row: how many records have all their hashes stored, and the seed of the
-- log's Ed25519 key, made by the server on first use. An append locks this row.
create table if not exists mls_key_log_state (
    id      int            not null,
    size    bigint         not null,
    log_key varbinary(64)  not null,
    primary key (id)
) engine = innodb default charset = utf8mb4;
