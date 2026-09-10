#!/bin/bash
# Rebuilds style.css from index.html, then cross-compiles the single-file
# Go binaries for macOS (Apple Silicon + Intel) and Windows (x64 + ARM64).
set -euo pipefail
cd "$(dirname "$0")/.."

# Use a writable build cache (useful under sandboxed shells where the
# default ~/Library/Caches/go-build isn't writable).
export GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/gocache}"
export GOENV="${GOENV:-${TMPDIR:-/tmp}/goenv}"
mkdir -p "$GOCACHE"

echo "Compiling Tailwind CSS from index.html..."
./tools/tailwindcss-v3 -i tools/input.css -o style.css --content "./index.html" --minify

mkdir -p build
rm -f build/*

APP=GurmatSangeet

build() {
  local goos=$1 goarch=$2 out=$3
  echo "Building $out ($goos/$goarch)..."
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath -ldflags="-s -w" -o "build/$out" .
}

build darwin  arm64 "${APP}-macOS-AppleSilicon"
build darwin  amd64 "${APP}-macOS-Intel"
build windows amd64 "${APP}-Windows-x64.exe"
build windows arm64 "${APP}-Windows-ARM64.exe"

chmod +x build/${APP}-macOS-AppleSilicon build/${APP}-macOS-Intel

echo ""
echo "Done. Binaries in build/:"
ls -la build/
