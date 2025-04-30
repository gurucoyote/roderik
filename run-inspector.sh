#!/usr/bin/env bash
set -euo pipefail

# Build the roderik binary
go build -o roderik .

# Write Inspector configuration
cat << 'EOF' > inspector.config.json
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

# Launch the Model Context Protocol Inspector
npx @modelcontextprotocol/inspector --config=inspector.config.json --server=roderik-mcp
