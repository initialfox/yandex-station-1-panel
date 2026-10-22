#!/usr/bin/env bash
# Установка Alice Panel на Станцию 1 через SSH (без SFTP — стрим tar по stdin).
#
#   ./scripts/install-ssh.sh
#   SSH_HOST=192.168.1.116 SSH_PORT=9865 ./scripts/install-ssh.sh
#   ./scripts/install-ssh.sh /path/to/alice-panel-station1.tar
#
# Пароль: SSH_PASS или ssh-agent / ключ. Для пароля нужен sshpass (brew install sshpass).

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TAR="${1:-$ROOT/alice-panel-station1.tar}"
HOST="${SSH_HOST:-192.168.1.116}"
PORT="${SSH_PORT:-9865}"
USER="${SSH_USER:-root}"
PREFIX="${PANEL_PREFIX:-/data/local/alice-panel}"
BINDIR="${PANEL_BINDIR:-/data/local/bin}"
LISTEN="${PANEL_LISTEN:-0.0.0.0:8080}"
REMOTE_TMP="/data/local/tmp/alice-panel-station1.tar"

if [ ! -f "$TAR" ]; then
  echo "нет $TAR — сначала: bash scripts/pack-release.sh" >&2
  exit 1
fi

SSH_BASE=(ssh -p "$PORT" -o StrictHostKeyChecking=accept-new -o PreferredAuthentications=password,publickey)
if [ -n "${SSH_PASS:-}" ]; then
  if ! command -v sshpass >/dev/null 2>&1; then
    echo "нужен sshpass: brew install sshpass (или ssh-ключ без пароля)" >&2
    exit 1
  fi
  SSH_BASE=(sshpass -p "$SSH_PASS" "${SSH_BASE[@]}")
fi

REMOTE="${USER}@${HOST}"
echo "==> SSH $REMOTE:$PORT"
echo "==> stream $(basename "$TAR") → $REMOTE_TMP"

# На станции нет SFTP — гоняем tar через ssh cat
"${SSH_BASE[@]}" "$REMOTE" "export ANDROID_DATA=/data ANDROID_ROOT=/system; mkdir -p /data/local/tmp '$BINDIR'; cat > '$REMOTE_TMP'" < "$TAR"

echo "==> распаковка + запуск"
"${SSH_BASE[@]}" "$REMOTE" "export ANDROID_DATA=/data ANDROID_ROOT=/system; \
  set -e; \
  pkill alice-httpd 2>/dev/null || true; sleep 1; \
  rm -rf '$PREFIX'; mkdir -p '$PREFIX'; \
  cd '$PREFIX'; \
  (toybox tar -xf '$REMOTE_TMP' 2>/dev/null || tar -xf '$REMOTE_TMP'); \
  rm -f '$REMOTE_TMP' ._* 2>/dev/null || true; \
  chmod 755 alice-httpd; \
  printf '%s\n' '#!/system/bin/sh' \
    'export ANDROID_DATA=/data' \
    'export ANDROID_ROOT=/system' \
    'cd $PREFIX || exit 1' \
    'pkill alice-httpd 2>/dev/null' \
    'sleep 1' \
    'setsid ./alice-httpd -listen $LISTEN -root . -repo ./repo -bindir $BINDIR </dev/null >httpd.log 2>&1 &' \
    'echo \$\!' > start.sh; \
  chmod 755 start.sh; \
  sh start.sh; \
  sleep 2; \
  (curl -s http://127.0.0.1:8080/api/health || wget -qO- http://127.0.0.1:8080/api/health); \
  echo"

echo "==> готово. Открой http://${HOST}:8080 (или LAN IP колонки)"
