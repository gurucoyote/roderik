# Stealth Mode Follow-Up

- 2025-10-28: `--stealth` only applies during process startup. MCP tool calls reuse the already initialized page, so there is no per-request toggle. Future work: allow the MCP server to request stealth mode dynamically (or switch profiles) without restarting the CLI.
