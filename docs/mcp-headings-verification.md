# MCP Markdown Heading Verification

## Summary
- Checked MCP `to_markdown` output for the Traumwind music page to confirm heading levels are preserved.
- Interactive browser run still downgraded headings to `#`, revealing a missing Accessibility domain init.
- Headless MCP server produced `##` and `###` as expected, matching the DOM's `H2`/`H3` structure.

## Verification Steps
1. Load https://traumwind.de/music/ via the interactive CLI MCP tools.
2. Capture Markdown output with `to_markdown`; observe every heading rendered as a single `#`.
3. Run `run_js` with `h1`–`h6` selectors; confirm source markup reports an `H2` followed by `H3`s.
4. Repeat steps 1–3 using the headless MCP server; the exported Markdown now mirrors the DOM hierarchy.

## Follow-Up
- Keep the Accessibility domain enabled before inspecting nodes so heading semantics remain available to Markdown exporters.
- Extend regression coverage to ensure future refactors preserve heading depth.
