#!/usr/bin/env bash
# Собрать alice-panel-station1.tar для GitHub Release (без gzip — на станции нет gunzip).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

test -x dist/alice-httpd-linux-arm64 || { echo "сначала: cd httpd && ./build.sh"; exit 1; }
test -f web/dist/index.html || { echo "сначала: cd web && npm run build"; exit 1; }

STAGE="$ROOT/.release-stage"
rm -rf "$STAGE"
mkdir -p "$STAGE/repo/packages"
cp dist/alice-httpd-linux-arm64 "$STAGE/alice-httpd"
cp -R web/dist/. "$STAGE/"
cp repo/packages.json repo/station-profile.json "$STAGE/repo/"
cp -R repo/packages/zsh repo/packages/fastfetch "$STAGE/repo/packages/"
chmod +x "$STAGE/alice-httpd"

export COPYFILE_DISABLE=1
OUT="$ROOT/alice-panel-station1.tar"
rm -f "$OUT"
tar -C "$STAGE" -cf "$OUT" .
rm -rf "$STAGE"
ls -lh "$OUT"
echo "upload to GitHub Release as alice-panel-station1.tar"
