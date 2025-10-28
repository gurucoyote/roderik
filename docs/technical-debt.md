# Technical Debt & Follow-Ups

This document tracks outstanding cleanups and improvement ideas that surfaced while hardening the MCP and CLI experience. When you address an item, update this list or delete the entry entirely so we keep the debt register current.

## Replace Remaining Rod `Must*` Calls

The original `TodoMustElimination.txt` enumerated lingering `Must*` helpers that will panic on error. Each needs a defensive wrapper so the CLI/MCP server returns an actionable error instead of crashing.

- [ ] `cmd/css.go`: `CurrentElement.MustEval(...)` inside `computedstyles`
- [ ] `cmd/a11y.go`:
  - `Page.MustElement("body")` in `quax` URL loading path
  - `Page.MustElement("html")` in the Markdown exporter path
- [ ] `cmd/inspect.go`: `Page.MustElement("html")` when the `html` command loads a URL
- [ ] `cmd/mcp.go` tool handlers:
  - `load_url`: `page.MustElement("body")`, `page.MustInfo().URL`
  - `to_markdown`: `page.MustElement("body")`
- [ ] `cmd/navigate.go`: `CurrentElement.MustElement(":first-child")` in `child`
- [ ] `cmd/root.go`:
  - `stealth.MustPage(Browser)`
  - `Browser.MustPage("about:blank")`
  - `Page.MustWaitLoad().MustWaitIdle()`
  - `Page.MustInfo().URL`
  - `ReportElement`: `el.MustEval(...)`, `el.MustElements("*")`, `el.MustText()`

When removing a `Must*`, propagate the resulting error back through the command/tool so the user sees which operation failed and why.

## MCP Navigation Backlog

Captured originally in `mcp-navigation.txt` (2025‑10‑07).

- [ ] Guard against duplicate `Page.EachEvent` listeners when `load_url` is called repeatedly from MCP clients.
- [ ] Expose the CLI DOM navigation helpers (`search`, `head`, `next`, `prev`, `elem`, etc.) as MCP tools while holding `pageMu` via `withPage`.
- [ ] Revisit the `RODERIK_ENABLE_LOAD_URL` gate—consider enabling `load_url` by default (or removing the environment flag entirely) for MCP-first workflows.

## Code Review Follow-Ups

Highlights from the Gemini review (2025‑10‑07):

- [ ] Finish the `Must*` de-risking noted above (already itemised).
- [ ] Add unit and integration tests (`go test ./...`) so future MCP changes are covered by automation.
- [ ] Introduce structured logging (e.g. `zap`, `zerolog`, or a structured `log/slog` setup) to simplify triage when the MCP server is embedded in other agents.
