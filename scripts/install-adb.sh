#!/usr/bin/env bash
# Установка Alice Panel на Станцию 1 через ADB push.
# Прямой push в /data/local часто Permission denied → кладём в /sdcard, дальше root shell.
#
#   ./scripts/install-adb.sh
#   ADB_SERIAL=192.168.1.116:5555 ./scripts/install-adb.sh
#   ./scripts/install-adb.sh /path/to/alice-panel-station1.tar

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TAR="${1:-$ROOT/alice-panel-station1.tar}"
PREFIX="${PANEL_PREFIX:-/data/local/alice-panel}"
BINDIR="${PANEL_BINDIR:-/data/local/bin}"
LISTEN="${PANEL_LISTEN:-0.0.0.0:8080}"
REMOTE_TMP="${PANEL_ADB_TMP:-/sdcard/alice-panel-station1.tar}"

ADB=(adb)
if [ -n "${ADB_SERIAL:-}" ]; then
  ADB=(adb -s "$ADB_SERIAL")
elif [ -n "${ANDROID_SERIAL:-}" ]; then
  ADB=(adb -s "$ANDROID_SERIAL")
fi

if [ ! -f "$TAR" ]; then
  echo "нет $TAR — сначала: bash scripts/pack-release.sh" >&2
  exit 1
fi

echo "==> ADB devices"
"${ADB[@]}" devices -l

echo "==> push $(basename "$TAR") → $REMOTE_TMP"
"${ADB[@]}" push "$TAR" "$REMOTE_TMP"

echo "==> распаковка + запуск ($PREFIX)"
# shell как root (на тюльпане adb обычно уже root)
"${ADB[@]}" shell "export ANDROID_DATA=/data ANDROID_ROOT=/system; \
  pkill alice-httpd 2>/dev/null; sleep 1; \
  rm -rf '$PREFIX'; mkdir -p '$PREFIX' '$BINDIR' /data/local/tmp; \
  cd '$PREFIX' && (toybox tar -xf '$REMOTE_TMP' 2>/dev/null || tar -xf '$REMOTE_TMP'); \
  rm -f '$REMOTE_TMP' '$PREFIX'/._* 2>/dev/null; \
  chmod 755 '$PREFIX'/alice-httpd; \
  printf '%s\n' '#!/system/bin/sh' \
    'export ANDROID_DATA=/data' \
    'export ANDROID_ROOT=/system' \
    'cd $PREFIX || exit 1' \
    'pkill alice-httpd 2>/dev/null' \
    'sleep 1' \
    'setsid ./alice-httpd -listen $LISTEN -root . -repo ./repo -bindir $BINDIR </dev/null >httpd.log 2>&1 &' \
    'echo \$\!' > '$PREFIX'/start.sh; \
  chmod 755 '$PREFIX'/start.sh; \
  sh '$PREFIX'/start.sh; \
  sleep 2; \
  curl -s http://127.0.0.1:8080/api/health || wget -qO- http://127.0.0.1:8080/api/health; \
  echo; \
  echo OK"

echo "==> готово. Открой http://<IP-колонки>:8080"
