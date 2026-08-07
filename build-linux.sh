#!/usr/bin/env bash
# Builds the server binaries (Linux x86-64) on any machine that has Go.
#
# Why separate from build.sh: compilation is the heaviest part of the build. On a
# modest server it takes tens of minutes, and the generated protocol package runs
# straight out of memory there. Go compiles for a foreign architecture without
# emulation, so it is wiser to compile on the workstation and hand the server the
# finished product.
#
# One service (dfs) pulls in webp processing, which is written in C, so a C
# cross-compiler is required. The easiest source is zig: `brew install zig`.
# Without zig every service but dfs is built, and the script says so plainly.
#
# Usage: server/build-linux.sh
set -euo pipefail

cd "$(dirname "$0")"
APP="$PWD/app"
INSTALL="$PWD/teamgramd"

export GOOS=linux GOARCH=amd64
export GOFLAGS=${GOFLAGS:--mod=mod}

# Static linking against musl: the binary does not care which libraries the
# server carries. zig does the linking itself (linkmode external) — the standard
# Go linker cannot combine a foreign architecture with C object files.
LDFLAGS="-s -w"
if command -v zig >/dev/null; then
  export CGO_ENABLED=1
  export CC="zig cc -target x86_64-linux-musl"
  export CXX="zig c++ -target x86_64-linux-musl"
  LDFLAGS="$LDFLAGS -linkmode external -extldflags '-static'"
else
  export CGO_ENABLED=0
  echo "zig not found: dfs will not be built (brew install zig)"
  echo
fi

# Service paths match build.sh — one list serves both scripts
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
for service_path in $SERVICES; do
  name=$(basename "$service_path")
  printf '%-14s ' "$name"
  (cd "$APP/$service_path" && eval go build -ldflags=\"$LDFLAGS\" -o "$INSTALL/bin/$name" .)
  echo "done"
done

echo
echo "built in $(( $(date +%s) - started ))s, total $(du -sh "$INSTALL/bin" | cut -f1)"
file "$INSTALL/bin/bff" | cut -c1-80
