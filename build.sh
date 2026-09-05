#!/usr/bin/env bash
# A failure of any build must bring the whole script down. Without that an
# unfinished image looks successful: Docker sees a zero exit code while half the
# services are missing inside.
set -euo pipefail

# -s -w drop the symbol table and debug info from the binaries: a third of the
# size that only a debugger needs. Panic stack traces stay readable — they take
# names from another section. On a machine with a tight disk this is half a
# gigabyte off the image.

PWD=`pwd`
ROOT=${PWD}
TEAMGRAMAPP=${PWD}"/app"
INSTALL=${PWD}"/teamgramd"

echo "build idgen ..."
cd ${TEAMGRAMAPP}/service/idgen/cmd/idgen
go build -ldflags="-s -w" -o ${INSTALL}/bin/idgen

echo "build status ..."
cd ${TEAMGRAMAPP}/service/status/cmd/status
go build -ldflags="-s -w" -o ${INSTALL}/bin/status

echo "build dfs ..."
cd ${TEAMGRAMAPP}/service/dfs/cmd/dfs
go build -ldflags="-s -w" -o ${INSTALL}/bin/dfs

echo "build media ..."
cd ${TEAMGRAMAPP}/service/media/cmd/media
go build -ldflags="-s -w" -o ${INSTALL}/bin/media

echo "build authsession ..."
cd ${TEAMGRAMAPP}/service/authsession/cmd/authsession
go build -ldflags="-s -w" -o ${INSTALL}/bin/authsession

echo "build biz ..."
cd ${TEAMGRAMAPP}/service/biz/biz/cmd/biz
go build -ldflags="-s -w" -o ${INSTALL}/bin/biz

echo "build msg ..."
cd ${TEAMGRAMAPP}/messenger/msg/cmd/msg
go build -ldflags="-s -w" -o ${INSTALL}/bin/msg

echo "build sync ..."
cd ${TEAMGRAMAPP}/messenger/sync/cmd/sync
go build -ldflags="-s -w" -o ${INSTALL}/bin/sync

echo "build bff ..."
cd ${TEAMGRAMAPP}/bff/bff/cmd/bff
go build -ldflags="-s -w" -o ${INSTALL}/bin/bff

echo "build session ..."
cd ${TEAMGRAMAPP}/interface/session/cmd/session
go build -ldflags="-s -w" -o ${INSTALL}/bin/session

echo "build gnetway ..."
cd ${TEAMGRAMAPP}/interface/gnetway/cmd/gnetway
go build -ldflags="-s -w" -o ${INSTALL}/bin/gnetway

# The two small tools that run inside the container. They were built by hand
# once and then stopped travelling, which is how a stale binary sits next to
# fresh ones - the same way the language packs stopped being shipped.
echo "build alert ..."
cd ${ROOT}/cmd/alert
go build -ldflags="-s -w" -o ${INSTALL}/bin/alert

echo "build invite ..."
cd ${ROOT}/cmd/invite
go build -ldflags="-s -w" -o ${INSTALL}/bin/invite

echo "build pushrelay ..."
cd ${ROOT}/cmd/pushrelay
go build -ldflags="-s -w" -o ${INSTALL}/bin/pushrelay

#echo "build httpserver ..."
#cd ${TEAMGRAMAPP}/interface/httpserver/cmd/httpserver
#go build -o ${INSTALL}/bin/httpserver
