# Repository Guidelines

## Project Structure & Module Organization
- `cmd/` holds Cobra commands and browser orchestration (`root.go`, `mcp.go`).
- `duckduck/` provides external API helpers; `test/` contains scenario fixtures.
- `assets/` stores static resources such as reference HTML and JSON.
- Executables are built from `main.go`; user data profiles live in `user_data/` during runtime.

## Build, Test, and Development Commands
- `go build ./...` — compile the CLI and verify module wiring.
- `GOCACHE=$(pwd)/.cache GOMODCACHE=$(pwd)/.modcache go test ./...` — run unit and integration tests using local caches (required when network egress is blocked).
- `./roderik <url>` — launch the interactive browser session against a target URL.
- `go run ./cmd/mcp.go` — start the MCP server variant for agent integrations.

## Coding Style & Naming Conventions
- Follow idiomatic Go style with tabs for indentation; run `gofmt -w <files>` before committing.
- Keep exported identifiers descriptive (`PrepareBrowser`, `LoadURL`) and prefer package-scoped helpers for shared logic.
- Log messages should include subsystem prefixes, e.g., `[MCP] shutdown requested`.
- Keep comments high-signal: explain intent, not syntax.

## Testing Guidelines
- Primary framework is Go’s standard `testing` package; use `_test.go` suffixes colocated with source files.
- Structure test names as `Test<Component><Behavior>` (e.g., `TestPrepareBrowserFallback`).
- Use temporary directories for browser profile tests to avoid clobbering `user_data/`.
- Record observed flakes in `docs/` or issue tracker with reproduction commands.

## Commit & Pull Request Guidelines
- Use short imperative commit subjects (`Handle Chrome profile locking for concurrent runs`).
- Include a brief body when touching multiple areas: list rationale, risk mitigation, and follow-up tasks.
- Pull requests should link relevant issues, summarize user-visible impact, and attach logs or screenshots for UI/browsing changes.
- Verify `go test ./...` (or document cache/network blockers) before requesting review; note any skipped tests explicitly.

## Chromium Launch Profiles
- Prefer the shared `user_data/` profile; the launcher now auto-falls back to disposable profiles for parallel sessions—retain this behavior in future changes.
- Always remove temporary profile directories in deferred cleanup paths to prevent lock conflicts.
