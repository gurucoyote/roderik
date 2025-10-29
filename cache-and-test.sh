#!/usr/bin/env bash

set -euo pipefail

project_root="$(pwd)"
cache_root="${CACHE_ROOT:-"$project_root/.cache"}"

export GOMODCACHE="${GOMODCACHE:-"$cache_root/go-mod"}"
export GOCACHE="${GOCACHE:-"$cache_root/go-build"}"

mkdir -p "$GOMODCACHE" "$GOCACHE"

if [[ ! -f "$project_root/go.mod" ]]; then
  echo "go.mod not found in $project_root" >&2
  exit 1
fi

echo "Populating module cache in $GOMODCACHE"
go mod download

echo "Running go test with GOCACHE=$GOCACHE"
go test ./... "$@"

echo "compiling roderik executable"
go build .
echo "Cross-compiling Windows binary (GOOS=windows GOARCH=amd64)"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$project_root/roderik.exe" .
echo "Windows build complete: $project_root/roderik.exe"
