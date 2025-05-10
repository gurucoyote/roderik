#!/usr/bin/env bash
set -euo pipefail

echo "Cross-compiling roderik for Windows..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o roderik.exe .
echo "Build complete: roderik.exe"
