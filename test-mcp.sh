#!/usr/bin/env bash
set -euo pipefail

# Point this at your binary if it's not on PATH:
MCP_CMD="./roderik mcp"

echo "🚀 Starting Roderik MCP server test…"

# Feed three MCP calls in sequence:
#  1) load_url     → navigate to example.com
#  2) get_html     → dump the HTML
#  3) shutdown     → stop the server
printf '%s\n' \
  '{"id":1,"tool":"list_tools","args":{}}' \
  '{"id":2,"tool":"load_url","args":{"url":"https://example.com"}}' \
  '{"id":3,"tool":"get_html","args":{}}' \
  '{"id":4,"tool":"shutdown","args":{}}' \
| $MCP_CMD | tee /dev/stderr | jq .

echo "✅ Done."
