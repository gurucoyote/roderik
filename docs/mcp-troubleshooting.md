# MCP Troubleshooting Notes

This page collects recurring MCP issues we have hit while driving browsers through the `roderik` server. Keep it updated as new failure modes appear.

## go-rod Helper Functions Missing

- **Symptom:** `TypeError: functions.selectable is not a function` (or any other `functions.<name>` is undefined) when running Rod-powered tools.
- **Why it happens:** Rod injects a JavaScript helper bundle into each browsing context. After navigation or frame swaps Chrome occasionally discards those helpers while Rod continues to call them.
- **Mitigations:**
  - Prefer Rod’s high-level selectors (`page.Elements("h1,h2")`, `el.Text()`, etc.) so Rod can re-inject helpers automatically.
  - Wrap custom JS calls in retry logic that refreshes the context when a helper is missing.
  - Re-initialise the helper bundle after navigation if you must keep using direct helper calls.

## Web Browser Tools Hang After Heavy Use

- **Observed:** October 7, 2025
- **Trigger:** Repeated `web-browser__to_markdown` calls on large sites caused every subsequent MPC tool to fail instantly with “tool call failed”.
- **What we saw:**
  - `roderik-mcp.log` stopped logging entirely after the last successful call.
  - Deleting the Chrome `user_data` profile did not help.
  - Even `shutdown` failed, which indicated that the bridge connection itself was dead.
- **Resolution:** Restarting the Codex CLI agent (the process hosting the MCP bridge) cleared the hang. We later fixed an unrelated charset bug for ISO‑8859‑1 responses, but that was not the root cause of the hang.
- **Takeaway:** When every tool call starts failing immediately, restart the hosting agent or bridge process first. Then inspect the MCP logs for the most recent successful run to confirm whether the server stopped responding.

## Markdown Heading Regression Test

Use this quick sanity check when confirming `to_markdown` renders heading levels correctly:

1. `load_url` with `https://traumwind.de/music/`.
2. `to_markdown` with no extra arguments.
3. `run_js` with  
   ```js
   (() => Array.from(document.querySelectorAll('h1,h2,h3,h4,h5,h6')).map(h => ({ tag: h.tagName, text: h.textContent.trim() })))()
   ```
4. Compare the markdown output: “The Joy of Noise” should appear as `##`, while tracks such as “Ecliptic 1972-B” render as `###`. If all headings collapse to a single `#`, the regression is back.

