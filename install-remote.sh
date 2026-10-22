#!/usr/bin/env bash
# Установка с компа: качает tar с GitHub → ADB или SSH.
#
#   curl -fsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install-remote.sh | bash -s -- adb 192.168.1.116
#   curl -fsSL …/install-remote.sh | SSH_PASS='…' bash -s -- ssh
#   curl -fsSL …/install-remote.sh | SSH_HOST=91.x.x.x SSH_PASS='…' bash -s -- ssh

set -euo pipefail

MODE="${1:-}"
ARG2="${2:-}"
REPO="${PANEL_REPO:-initialfox/yandex-station-1-panel}"
TAG="${PANEL_TAG:-latest}"

if [ "$MODE" != "adb" ] && [ "$MODE" != "ssh" ]; then
  echo "usage: bash install-remote.sh adb [IP]" >&2
  echo "       SSH_PASS=… bash install-remote.sh ssh" >&2
  exit 1
fi

if [ "$TAG" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/alice-panel-station1.tar"
else
  URL="https://github.com/${REPO}/releases/download/${TAG}/alice-panel-station1.tar"
fi

TAR="${TMPDIR:-/tmp}/alice-panel-station1-$$.tar"
trap 'rm -f "$TAR"' EXIT

echo "==> качаю $URL"
curl -fL --retry 3 -o "$TAR" "$URL"
ls -lh "$TAR"

# команды на колонке после того как tar уже в /data/local/tmp/alice-panel-station1.tar
on_device() {
  cat <<'EOS'
export ANDROID_DATA=/data ANDROID_ROOT=/system
set -e
pkill alice-httpd 2>/dev/null || true
sleep 1
rm -rf /data/local/alice-panel
mkdir -p /data/local/alice-panel /data/local/bin /data/local/tmp
cd /data/local/alice-panel
toybox tar -xf /data/local/tmp/alice-panel-station1.tar 2>/dev/null || tar -xf /data/local/tmp/alice-panel-station1.tar
rm -f /data/local/tmp/alice-panel-station1.tar /sdcard/alice-panel-station1.tar ._* 2>/dev/null || true
chmod 755 alice-httpd
printf '%s\n' \
  '#!/system/bin/sh' \
  'export ANDROID_DATA=/data' \
  'export ANDROID_ROOT=/system' \
  'cd /data/local/alice-panel || exit 1' \
  'pkill alice-httpd 2>/dev/null' \
  'sleep 1' \
  'setsid ./alice-httpd -listen 0.0.0.0:8080 -root . -repo ./repo -bindir /data/local/bin </dev/null >httpd.log 2>&1 &' \
  'echo $!' > start.sh
chmod 755 start.sh
sh start.sh
sleep 2
curl -s http://127.0.0.1:8080/api/health 2>/dev/null || true
echo
EOS
}

if [ "$MODE" = "adb" ]; then
  command -v adb >/dev/null || { echo "нужен adb" >&2; exit 1; }
  SERIAL="${ARG2:-${ADB_SERIAL:-${ANDROID_SERIAL:-}}}"
  ADB=(adb)
  if [ -n "$SERIAL" ]; then
    case "$SERIAL" in *:*) ;; *.*) SERIAL="${SERIAL}:5555" ;; esac
    echo "==> adb connect $SERIAL"
    adb connect "$SERIAL" || true
    ADB=(adb -s "$SERIAL")
  fi
  "${ADB[@]}" devices -l
  echo "==> push → /sdcard"
  "${ADB[@]}" push "$TAR" /sdcard/alice-panel-station1.tar
  "${ADB[@]}" shell "cp /sdcard/alice-panel-station1.tar /data/local/tmp/alice-panel-station1.tar || true"
  echo "==> setup"
  on_device | "${ADB[@]}" shell sh
  echo "==> OK  http://<IP-колонки>:8080"
  exit 0
fi

HOST="${SSH_HOST:-${ARG2:-192.168.1.116}}"
PORT="${SSH_PORT:-9865}"
USER="${SSH_USER:-root}"
OPTS=(-p "$PORT" -o StrictHostKeyChecking=accept-new)
SSH=(ssh "${OPTS[@]}")
if [ -n "${SSH_PASS:-}" ]; then
  command -v sshpass >/dev/null || { echo "нужен sshpass (brew/apt install sshpass)" >&2; exit 1; }
  SSH=(sshpass -p "$SSH_PASS" ssh "${OPTS[@]}")
fi
REMOTE="${USER}@${HOST}"
echo "==> ssh $REMOTE:$PORT — stream tar"
"${SSH[@]}" "$REMOTE" "mkdir -p /data/local/tmp; cat > /data/local/tmp/alice-panel-station1.tar" < "$TAR"
echo "==> setup"
on_device | "${SSH[@]}" "$REMOTE" sh
echo "==> OK  http://${HOST}:8080"
