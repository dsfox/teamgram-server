#!/usr/bin/env bash
# Падение любой сборки обязано ронять весь скрипт. Без этого недостроенный
# образ выглядит успешным: Docker видит нулевой код возврата, а внутри
# не хватает половины сервисов.
set -euo pipefail

# -s -w выбрасывают из бинарников таблицу символов и отладочную информацию:
# треть объёма, которая нужна только отладчику. Стектрейсы паник остаются
# читаемыми — они берут имена из другой секции. Для машины с тесным диском
# это разница в полгигабайта на образ.

PWD=`pwd`
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

#echo "build httpserver ..."
#cd ${TEAMGRAMAPP}/interface/httpserver/cmd/httpserver
#go build -o ${INSTALL}/bin/httpserver
