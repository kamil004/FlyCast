#!/usr/bin/env bash
# Downloads statically-linked ffmpeg binaries (Linux amd64 + arm64) into
# build/vendor/ffmpeg/. These are johnvansickle.com "static" builds
# (FFmpeg GPL, includes libx264/libx265/libvpx, h264/hevc/aac decoders).
#
# Static linking matters on QNAP: its glibc is old (2.21) and only fully
# static binaries are guaranteed to run across models.
set -euo pipefail
cd "$(dirname "$0")/.."
VENDOR="build/vendor/ffmpeg"
mkdir -p "$VENDOR"

# Pinned release of eugeneware/ffmpeg-static (mirrors johnvansickle builds).
TAG="b6.1.1"
BASE="https://github.com/eugeneware/ffmpeg-static/releases/download/$TAG"

fetch() {
  local name="$1" url="$2" out="$3"
  if [ -s "$VENDOR/$out" ]; then
    echo "ffmpeg: $out already present"
    return
  fi
  echo "ffmpeg: downloading $name ..."
  curl -fSL -o "$VENDOR/$out.gz" "$url"
  gunzip -f "$VENDOR/$out.gz"
  chmod +x "$VENDOR/$out"
}

fetch amd64 "$BASE/ffmpeg-linux-x64.gz"   ffmpeg_amd64
fetch arm64 "$BASE/ffmpeg-linux-arm64.gz" ffmpeg_arm64

echo "ffmpeg binaries ready:"
ls -la "$VENDOR"
