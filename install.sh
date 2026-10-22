#!/system/bin/sh
# Alice Panel · Яндекс Станция 1
#
# На колонке curl без CA-бандла — нужен -k:
#   curl -kfsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
#
# Пакеты (zsh, curl, …) — через apkg (yandex-station-repo), не из локального repo панели.
#
# Опционально зафиксировать релиз:
#   PANEL_TAG=v0.4.0 sh install.sh

set -e

export ANDROID_DATA="${ANDROID_DATA:-/data}"
export ANDROID_ROOT="${ANDROID_ROOT:-/system}"
setenforce 0 2>/dev/null || true

REPO="${PANEL_REPO:-initialfox/yandex-station-1-panel}"
TAG="${PANEL_TAG:-latest}"
PREFIX="${PANEL_PREFIX:-/data/local/alice-panel}"
BINDIR="${PANEL_BINDIR:-/data/local/bin}"
LISTEN="${PANEL_LISTEN:-0.0.0.0:8080}"
APKG_URL="${APKG_URL:-https://raw.githubusercontent.com/initialfox/yandex-station-repo/pkg/apkg}"

# buildroot?
if [ -e /system/bin/linker64 ] || [ -e /system/bin/linker ]; then
  FLAVOR=android
else
  FLAVOR=buildroot
  BINDIR="${PANEL_BINDIR:-/usr/local/bin}"
  PREFIX="${PANEL_PREFIX:-/usr/local/alice-panel}"
  mount -o remount,rw / 2>/dev/null || true
fi

if [ "$TAG" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/alice-panel-station1.tar"
else
  URL="https://github.com/${REPO}/releases/download/${TAG}/alice-panel-station1.tar"
fi

echo "==> Alice Panel · Станция 1"
echo "    flavor=$FLAVOR repo=$REPO tag=$TAG"
echo "    url=$URL"

mkdir -p "$PREFIX" "$BINDIR" /data/local/tmp 2>/dev/null || true
TMP="/data/local/tmp/alice-panel-$$.tar"
[ -d /sdcard ] && TMP="/sdcard/alice-panel-$$.tar"
rm -f "$TMP"

download() {
  if command -v curl >/dev/null 2>&1; then
    curl -kfL --retry 3 -o "$TMP" "$URL"
  elif [ -x "$BINDIR/curl" ]; then
    "$BINDIR/curl" -kfL --retry 3 -o "$TMP" "$URL"
  elif command -v wget >/dev/null 2>&1; then
    wget --no-check-certificate -O "$TMP" "$URL"
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

# локальный repo/packages больше не нужен — сносим, если остался от старых релизов
rm -rf "$PREFIX/repo/packages" "$PREFIX/repo/packages.json" 2>/dev/null || true

ensure_apkg() {
  if [ -x "$BINDIR/apkg" ]; then
    return 0
  fi
  echo "==> ставлю apkg → $BINDIR/apkg"
  mkdir -p "$BINDIR"
  if command -v curl >/dev/null 2>&1; then
    curl -kfL --retry 3 -o "$BINDIR/apkg" "$APKG_URL"
  elif command -v wget >/dev/null 2>&1; then
    wget --no-check-certificate -O "$BINDIR/apkg" "$APKG_URL"
  else
    echo "    не удалось скачать apkg" >&2
    return 1
  fi
  chmod 755 "$BINDIR/apkg"
}

echo "==> apkg + zsh"
if ensure_apkg; then
  export PATH="$BINDIR:$PATH"
  "$BINDIR/apkg" update 2>/dev/null || true
  "$BINDIR/apkg" install zsh 2>/dev/null || echo "    zsh через apkg — пропуск" >&2
  if [ -x "$BINDIR/zsh" ] && [ -d /system/xbin ]; then
    mount -o remount,rw /system 2>/dev/null || true
    ln -sf "$BINDIR/zsh" /system/xbin/zsh 2>/dev/null || true
    mount -o remount,ro /system 2>/dev/null || true
  fi
  [ -x "$BINDIR/zsh" ] && echo "    ok $($BINDIR/zsh --version 2>/dev/null || echo zsh)"
fi

cat > start.sh <<EOF
#!/system/bin/sh
export ANDROID_DATA=/data
export ANDROID_ROOT=/system
export PATH=$BINDIR:\$PATH
cd "$PREFIX" || exit 1
pkill alice-httpd 2>/dev/null
sleep 1
setsid ./alice-httpd -listen $LISTEN -root . -bindir $BINDIR </dev/null >httpd.log 2>&1 &
echo \$!
EOF
chmod 755 start.sh

echo "==> запуск"
sh start.sh
sleep 2

if curl -kfsS "http://127.0.0.1:8080/api/health" >/dev/null 2>&1 || curl -fsS "http://127.0.0.1:8080/api/health" >/dev/null 2>&1; then
  IP=$(ip -4 addr show wlan0 2>/dev/null | sed -n 's/.*inet \([0-9.]*\).*/\1/p' | head -1)
  if [ -z "$IP" ]; then
    IP=$(ip -4 addr show 2>/dev/null | sed -n 's/.*inet \([0-9.]*\).*/\1/p' | grep -v '^127\.' | head -1)
  fi
  echo
  echo "==> панель запущена"
  echo "    открой в браузере (та же Wi‑Fi, что и колонка):"
  echo "    http://${IP:-<IP-колонки>}:8080"
  echo
  curl -ksS "http://127.0.0.1:8080/api/health" 2>/dev/null || curl -sS "http://127.0.0.1:8080/api/health"
  echo
else
  echo "==> не ответил /api/health — смотри $PREFIX/httpd.log" >&2
  tail -20 "$PREFIX/httpd.log" 2>/dev/null || true
  exit 1
fi
