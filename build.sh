#!/usr/bin/env bash
# FlyCast build entry point.
#
# Every compilation produces the version stored in ./VERSION and then advances
# it for the next build (1.0.0 -> 1.0.1 -> ... -> 1.0.10 -> 1.1.0 ...).
# The same version is embedded in the frontend, the backend binary, qpkg.cfg
# and the output file name (FlyCast_<version>.qpkg).
set -euo pipefail
cd "$(dirname "$0")"

VER="$(cat VERSION)"
export FLYCAST_VERSION="$VER"

echo "==> Building FlyCast $VER"
bash build/make_payload.sh
python3 build/build_qpkg.py

# Advance the version only at the very end, after the qpkg file was written.
if [ -f "FlyCast_${VER}.qpkg" ]; then
  python3 build/version.py bump > VERSION.next
  mv VERSION.next VERSION
  echo "==> Built FlyCast_${VER}.qpkg — next compilation will be $(cat VERSION)"
else
  echo "==> qpkg not written, VERSION unchanged"
fi
