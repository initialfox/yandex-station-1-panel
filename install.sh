#!/system/bin/sh
# Alice Panel · Яндекс Станция 1
#
# На колонке curl без CA-бандла — нужен -k:
#   curl -kfsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
# или:
#   wget --no-check-certificate -qO- https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
#
# Опционально зафиксировать релиз:
#   PANEL_TAG=v0.1.0 sh install.sh

set -e

# глушим шум bionic при запуске из dropbear/ssh
export ANDROID_DATA="${ANDROID_DATA:-/data}"
export ANDROID_ROOT="${ANDROID_ROOT:-/system}"

REPO="${PANEL_REPO:-initialfox/yandex-station-1-panel}"
TAG="${PANEL_TAG:-latest}"
PREFIX="${PANEL_PREFIX:-/data/local/alice-panel}"
BINDIR="${PANEL_BINDIR:-/data/local/bin}"
LISTEN="${PANEL_LISTEN:-0.0.0.0:8080}"

if [ "$TAG" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/alice-panel-station1.tar"
else
  URL="https://github.com/${REPO}/releases/download/${TAG}/alice-panel-station1.tar"
fi

echo "==> Alice Panel · Станция 1"
echo "    repo=$REPO tag=$TAG"
echo "    url=$URL"

mkdir -p "$PREFIX" /data/local/tmp "$BINDIR"
TMP="/data/local/tmp/alice-panel-$$.tar"
rm -f "$TMP"

download() {
  # Станция 1: системный curl без CA → SSL verify падает (curl 60)
  if command -v curl >/dev/null 2>&1; then
    curl -kfL --retry 3 -o "$TMP" "$URL" || curl -fL --retry 3 -o "$TMP" "$URL"
  elif command -v wget >/dev/null 2>&1; then
    wget --no-check-certificate -O "$TMP" "$URL" || wget -O "$TMP" "$URL"
  else
    echo "нужен curl или wget" >&2
    exit 1
  fi
}

echo "==> скачиваю пакет…"
download
ls -l "$TMP"

echo "==> распаковываю в $PREFIX"
pkill alice-httpd 2>/dev/null || true
sleep 1
rm -rf "$PREFIX"
mkdir -p "$PREFIX"
cd "$PREFIX"
if command -v toybox >/dev/null 2>&1; then
  toybox tar -xf "$TMP"
else
  tar -xf "$TMP"
fi
rm -f "$TMP" ._* 2>/dev/null || true
chmod 755 alice-httpd start.sh 2>/dev/null || chmod 755 alice-httpd

cat > start.sh <<EOF
#!/system/bin/sh
export ANDROID_DATA=/data
export ANDROID_ROOT=/system
cd "$PREFIX" || exit 1
pkill alice-httpd 2>/dev/null
sleep 1
setsid ./alice-httpd -listen $LISTEN -root . -repo ./repo -bindir $BINDIR </dev/null >httpd.log 2>&1 &
echo \$!
EOF
chmod 755 start.sh

echo "==> запуск"
sh start.sh
sleep 2

if curl -kfsS "http://127.0.0.1:8080/api/health" >/dev/null 2>&1 || curl -fsS "http://127.0.0.1:8080/api/health" >/dev/null 2>&1; then
  IP=$(ip -4 addr show wlan0 2>/dev/null | sed -n 's/.*inet \([0-9.]*\).*/\1/p' | head -1)
  echo "==> OK  http://${IP:-<IP>}:8080"
  curl -ksS "http://127.0.0.1:8080/api/health" 2>/dev/null || curl -sS "http://127.0.0.1:8080/api/health"
  echo
else
  echo "==> не ответил /api/health — смотри $PREFIX/httpd.log" >&2
  tail -20 "$PREFIX/httpd.log" 2>/dev/null || true
  exit 1
fi
