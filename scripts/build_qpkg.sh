#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
QPKG_DIR=QPKG
PKG=FlyCast_0.1.0.qpkg

chmod +x "$QPKG_DIR/CONTROL/control"

TMP=$(mktemp -d)
cp -r "$QPKG_DIR"/* "$TMP/"
tar -C "$TMP" -czf "$PKG" .
rm -rf "$TMP"
echo "Created $PKG"
