#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
OUT="$(cd "$HERE/.." && pwd)/dist"
mkdir -p "$OUT"

build() {
  local goos="$1" goarch="$2" name="$3"
  echo "building $name ..."
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags="-s -w" -o "$OUT/$name" "$HERE"
  ls -lh "$OUT/$name"
}

build linux arm64 alice-httpd-linux-arm64
build darwin arm64 alice-httpd-darwin-arm64

echo "done → $OUT"
