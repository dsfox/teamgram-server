#!/usr/bin/env bash
# Собирает бинарники под сервер (Linux x86-64) на любой машине с Go.
#
# Зачем отдельно от build.sh: компиляция — самая тяжёлая часть сборки, и на
# скромном сервере она занимает десятки минут, а пакет сгенерированного
# протокола там вообще упирается в память. Go умеет собирать под чужую
# архитектуру без эмуляции, поэтому разумнее компилировать на рабочей машине,
# а серверу отдавать готовое.
#
# Один сервис (dfs) тянет обработку webp, а та написана на C, поэтому нужен
# кросс-компилятор C. Проще всего его даёт zig: `brew install zig`. Без zig
# соберутся все сервисы, кроме dfs, и скрипт честно об этом скажет.
#
# Запуск: server/build-linux.sh
set -euo pipefail

cd "$(dirname "$0")"
APP="$PWD/app"
INSTALL="$PWD/teamgramd"

export GOOS=linux GOARCH=amd64
export GOFLAGS=${GOFLAGS:--mod=mod}

# Статическая линковка с musl: бинарнику всё равно, какие библиотеки на сервере.
# Линковку выполняет сам zig (linkmode external) — стандартный компоновщик Go
# не умеет собирать чужую архитектуру вместе с объектниками C.
LDFLAGS="-s -w"
if command -v zig >/dev/null; then
  export CGO_ENABLED=1
  export CC="zig cc -target x86_64-linux-musl"
  export CXX="zig c++ -target x86_64-linux-musl"
  LDFLAGS="$LDFLAGS -linkmode external -extldflags '-static'"
else
  export CGO_ENABLED=0
  echo "zig не найден: dfs собран не будет (brew install zig)"
  echo
fi

# Пути к сервисам совпадают с build.sh — список один на оба скрипта
SERVICES="
service/idgen/cmd/idgen
service/status/cmd/status
service/dfs/cmd/dfs
service/media/cmd/media
service/authsession/cmd/authsession
service/biz/biz/cmd/biz
messenger/msg/cmd/msg
messenger/sync/cmd/sync
bff/bff/cmd/bff
interface/session/cmd/session
interface/gnetway/cmd/gnetway
"

started=$(date +%s)
for put in $SERVICES; do
  name=$(basename "$put")
  printf '%-14s ' "$name"
  (cd "$APP/$put" && eval go build -ldflags=\"$LDFLAGS\" -o "$INSTALL/bin/$name" .)
  echo "готово"
done

echo
echo "собрано за $(( $(date +%s) - started )) с, всего $(du -sh "$INSTALL/bin" | cut -f1)"
file "$INSTALL/bin/bff" | cut -c1-80
