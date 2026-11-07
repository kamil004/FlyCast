#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT="$(pwd)"
PAYLOAD="build/payload"

# Version for this build (set by build.sh); fall back to the VERSION file.
VER="${FLYCAST_VERSION:-$(cat VERSION 2>/dev/null || echo 0.0.0)}"
export FLYCAST_VERSION="$VER"
export VITE_FLYCAST_VERSION="$VER"
echo "make_payload: version $VER"

rm -rf "$PAYLOAD"
mkdir -p "$PAYLOAD"

# 1) Go backend -> Linux ELF binaries at payload root
pushd backend >/dev/null
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=$VER" -o "$ROOT/$PAYLOAD/flycast_amd64" ./cmd/flycast
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w -X main.version=$VER" -o "$ROOT/$PAYLOAD/flycast_arm64" ./cmd/flycast
popd >/dev/null

cp "$PAYLOAD/flycast_amd64" "$PAYLOAD/flycast"
chmod +x "$PAYLOAD/flycast" "$PAYLOAD/flycast_amd64" "$PAYLOAD/flycast_arm64"

# 1b) Bundled static ffmpeg (self-contained transcoding/thumbnails)
bash build/fetch_ffmpeg.sh
cp "build/vendor/ffmpeg/ffmpeg_amd64" "$PAYLOAD/ffmpeg_amd64"
cp "build/vendor/ffmpeg/ffmpeg_arm64" "$PAYLOAD/ffmpeg_arm64"
chmod +x "$PAYLOAD/ffmpeg_amd64" "$PAYLOAD/ffmpeg_arm64"

# 2) Frontend -> payload/web
pushd frontend >/dev/null
[ -d node_modules ] || npm install --no-audit --no-fund
npm run build -- --outDir "../$PAYLOAD/web" --emptyOutDir
popd >/dev/null

# 3) Service script at payload root (required by qinstall.sh)
cp "build/control/FlyCast.sh" "$PAYLOAD/FlyCast.sh"
chmod +x "$PAYLOAD/FlyCast.sh"

# 4) QNAP App Center icons at payload root
python3 build/make_icons.py "$PAYLOAD"
cp "QPKG/icons/FlyCast.svg" "$PAYLOAD/.qpkg_icon.svg"
echo "payload ready:"; ls -la "$PAYLOAD"
