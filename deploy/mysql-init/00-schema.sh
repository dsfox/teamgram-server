#!/bin/bash
# teamgram schema initialisation.
#
# The upstream way (mounting the sql directory straight into
# docker-entrypoint-initdb.d) does not work: migrate-20260309.sql puts an index on
# the requested column of chat_invite_participants, where there is none (the
# column lives in chat_invites). MySQL aborts initialisation on the first error,
# so everything further down the alphabet is skipped — including rank2 in
# chat_participants and the service account from z_init.sql. Without rank2 the
# participant list query fails and groups do not work at all.
#
# So we set the order ourselves, apply migrations with --force (one failure must
# not kill the rest), and a separate gate catches schema drift against the code:
# tests/schema_gate.py.
set -uo pipefail

SQL_DIR=/upstream-sql
MYSQL="mysql -uroot -p${MYSQL_ROOT_PASSWORD} ${MYSQL_DATABASE}"

echo "[schema] base schema: 1_teamgram.sql"
$MYSQL < "${SQL_DIR}/1_teamgram.sql"

for f in $(ls "${SQL_DIR}"/migrate-*.sql | sort); do
  echo "[schema] migration: $(basename "$f")"
  $MYSQL --force < "$f" 2>&1 | sed 's/^/[schema][warn] /'
done

# What has been applied is written down, so a patch added later can be applied
# to a database that already exists without guessing from the schema whether it
# is there. deploy/apply-patches.sh is the other half of this.
$MYSQL <<'PATCHES'
CREATE TABLE IF NOT EXISTS schema_patches (
  name varchar(191) NOT NULL,
  applied_at int(11) NOT NULL,
  PRIMARY KEY (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
PATCHES

for f in $(ls /sql-patches/*.sql | sort); do
  echo "[schema] our patch: $(basename "$f")"
  $MYSQL < "$f"
  echo "insert into schema_patches(name, applied_at) values ('$(basename "$f")', unix_timestamp());" | $MYSQL
done

echo "[schema] service data: z_init.sql"
$MYSQL --force < "${SQL_DIR}/z_init.sql" 2>&1 | sed 's/^/[schema][warn] /'

echo "[schema] done"
