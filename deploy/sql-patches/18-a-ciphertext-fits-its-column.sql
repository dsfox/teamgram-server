-- A message's text as the server keeps it, wide enough for its ciphertext (#269).
--
-- varchar(6000) held any text a person can type, and in a chat that encrypts
-- the column holds the ciphertext instead: 4096 Cyrillic characters become
-- 11125, and anything past about 2170 was refused by MySQL with error 1406,
-- which the client took for a 500 and sent again for good. TEXT holds 65535
-- bytes; the server answers MESSAGE_TOO_LONG past that rather than letting
-- the database refuse it.
alter table messages
    modify `message` text collate utf8mb4_unicode_ci not null default ('');
