# Repository Guidelines

## Project Structure & Module Organization
- `cmd/` holds Cobra commands and browser orchestration (`root.go`, `mcp.go`).
- `duckduck/` provides external API helpers; `test/` contains scenario fixtures.
- `assets/` stores static resources such as reference HTML and JSON.
- Executables are built from `main.go`; user data profiles live in `user_data/` during runtime.

## Build, Test, and Development Commands
- `go build ./...` — compile the CLI and verify module wiring.
- `GOCACHE=$(pwd)/.cache GOMODCACHE=$(pwd)/.modcache go test ./...` — run unit and integration tests using local caches (required when network egress is blocked).
- `./cache-and-test.sh` — populate `.cache`/`.modcache` and invoke `go test ./cmd/...` without external network access; prefer this helper when running the suite in restricted environments.
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

## WSL2 ↔ Windows Chrome Attachment
- Use the `--desktop` flag (`roderik --desktop <url>`) to attach to the Windows Chrome instance. Running the native `roderik.exe` on Windows avoids WSL firewall issues; see `docs/profile-management-plan.md` for profile selection and naming plans, and `docs/wsl-windows-legacy.md` for the older WSL-only workflow.
- `--profile` selects (or, if omitted, interactively prompts for) the Chrome profile; when provided it also seeds the default window title, while `--profile-title` can override the label written into Chrome’s `Local State`.
- Navigation hooks reset the active element after each page load; commands like `search`, `elem`, and `click` now track the desktop browser when you navigate in the GUI. If a real click times out, the shell falls back to the element’s `href` so link traversal continues.
- `type` accepts multiple words and strips optional wrapping quotes before sending keys, matching common CLI usage (e.g., `type "roderik browser"`).
- When native typing stalls (common in desktop attach mode), the CLI falls back to injecting the value via JavaScript and dispatching `input`/`change` events, keeping form fields in sync.
- When native clicks or typing time out, the CLI retries in page JavaScript: anchors trigger `href` navigation; other controls synthesize `click`/`input` events so desktop sessions stay responsive.
- Avoid calling DevTools tools that assume “network idle” (e.g., `Page.MustWaitIdle`) against long-lived pages; they now wait only for the initial load event to prevent `context deadline exceeded` errors.

## Windows Builds
- Cross-compile the CLI with `GOOS=windows GOARCH=amd64 GOCACHE=$(pwd)/.cache GOMODCACHE=$(pwd)/.modcache go build -o roderik.exe .`; the cached module directories avoid sandbox DNS restrictions.
- Run the resulting `roderik.exe --desktop <url>` from PowerShell or Command Prompt. Because it attaches directly to the Windows Chrome DevTools socket, no WSL tunneling is required—just ensure port 9222 stays open as described above.
