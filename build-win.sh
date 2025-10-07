#!/usr/bin/env bash
set -euo pipefail

echo "Cross-compiling roderik for Windows..."

: "${CACHE_ROOT:=$(pwd)/.cache}"
export GOCACHE="${GOCACHE:-$CACHE_ROOT/go-build}"
export GOMODCACHE="${GOMODCACHE:-$CACHE_ROOT/go-mod}"

mkdir -p "$GOCACHE" "$GOMODCACHE"

GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o roderik.exe .
echo "Build complete: roderik.exe"
