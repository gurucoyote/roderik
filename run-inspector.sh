#!/usr/bin/env bash
set -euo pipefail

# 1) make sure we're in the script's own directory
SCRIPT_DIR=$(dirname "$(realpath "${BASH_SOURCE[0]}")")
cd "$SCRIPT_DIR"

# 2) build the binary
echo "🛠 Building roderik…"
go build -o roderik .

# 3) write out the Inspector config
cat > inspector.config.json << 'EOF'
{
  "mcpServers": {
    "roderik-mcp": {
      "command": "./roderik",
      "args": ["mcp"],
      "env": {}
    }
  }
}
EOF
echo "✔ inspector.config.json written:"
cat inspector.config.json

# 4) launch the Inspector (telling it which server key to use)
echo "🚀 Starting MCP Inspector…"
npx @modelcontextprotocol/inspector \
  --config=inspector.config.json \
  --server=roderik-mcp
