#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
esac

BINARY="${SCRIPT_DIR}/serper-${OS}-${ARCH}"

if [ ! -x "$BINARY" ]; then
    echo "error: no binary found for ${OS}/${ARCH} at ${BINARY}" >&2
    exit 1
fi

exec "$BINARY" "$@"
