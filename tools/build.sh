#!/bin/bash
# Rebuilds style.css from index.html, then cross-compiles the single-file
# Go binaries for macOS (Apple Silicon + Intel) and Windows (x64 + ARM64).
#
# The macOS binaries get zipped (one binary per zip, nothing else inside)
# because browsers strip the executable bit from any file they download --
# a raw downloaded binary is never runnable no matter how it's named. Zip
# is one of the few download formats whose unzip restores Unix permission
# bits, so double-click-to-run keeps working once the user unzips it.
# Windows .exe files don't have this problem, so they ship raw.
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
echo "Zipping macOS binaries (preserves the executable bit through download)..."
( cd build && zip -q "${APP}-macOS-AppleSilicon.zip" "${APP}-macOS-AppleSilicon" && rm "${APP}-macOS-AppleSilicon" )
( cd build && zip -q "${APP}-macOS-Intel.zip" "${APP}-macOS-Intel" && rm "${APP}-macOS-Intel" )

echo ""
echo "Done. Files in build/:"
ls -la build/
