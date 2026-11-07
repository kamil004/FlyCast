#!/usr/bin/env python3
"""FlyCast version helper.

`VERSION` holds the version that will be used by the NEXT compilation.

Bump rules (as requested):
  - start at 1.0.0
  - every build increments the patch component (0.0.1)
  - after 1.0.10 the next version is 1.1.0
  - after 1.1.10 the next version is 1.2.0

Usage: python3 build/version.py bump   -> prints the next version
"""
import sys

MAX_PATCH = 10


def bump(v: str) -> str:
    major, minor, patch = (int(x) for x in v.strip().split("."))
    patch += 1
    if patch > MAX_PATCH:
        patch = 0
        minor += 1
    return f"{major}.{minor}.{patch}"


def main() -> int:
    if len(sys.argv) < 2 or sys.argv[1] != "bump":
        print(f"usage: {sys.argv[0]} bump", file=sys.stderr)
        return 2
    print(bump(open("VERSION").read()))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
