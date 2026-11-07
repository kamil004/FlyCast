#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
APP=flycast
OUT=QPKG/app

mkdir -p "$OUT"

# backend
pushd backend
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "../$OUT/${APP}_amd64" ./cmd/flycast
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o "../$OUT/${APP}_arm64" ./cmd/flycast
popd

# frontend
pushd frontend
npm ci || npm install
npm run build
popd

cp "$OUT/${APP}_amd64" "$OUT/${APP}" 2>/dev/null || cp "$OUT/${APP}_arm64" "$OUT/${APP}"
chmod +x "$OUT/${APP}" "$OUT/${APP}_amd64" "$OUT/${APP}_arm64"
echo "BUILD DONE"
