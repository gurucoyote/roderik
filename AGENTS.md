# Repository Guidelines

## Project Structure & Module Organization
- `cmd/` holds Cobra commands and browser orchestration (`root.go`, `mcp.go`).
- `duckduck/` provides external API helpers; `test/` contains scenario fixtures.
- `assets/` stores static resources such as reference HTML and JSON.
- Executables are built from `main.go`; user data profiles live in `user_data/` during runtime.

## Build, Test, and Development Commands
- `go build ./...` — compile the CLI and verify module wiring.
- `GOCACHE=$(pwd)/.cache GOMODCACHE=$(pwd)/.modcache go test ./...` — run unit and integration tests using local caches (required when network egress is blocked).
- `./cache-and-test.sh` — populate `.cache`/`.modcache` and invoke `go test ./cmd/...` without external network access; **agents should default to this helper** so we avoid repeating manual cache setup instructions.
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

## MCP Messenger DOM Walk Playbook
- Load Facebook with `gui-browser/load_url` and let the user bring the desired Messenger thread into view.
- Confirm iframe content is reachable by running `gui-browser/head` (e.g., `{"level":"4"}`) and looking for timestamp headers like “Today at 3:24 PM”.
- Use `gui-browser/run_js` to iterate `div[role="row"]` nodes: treat rows with `h4` as timestamp markers, and for other rows capture the sender from the first direct `span`, message text from `div[dir="auto"]` spans, and audio durations via `row.innerText` regex checks.
- Return an array of `{sender, timestamp, text?, audioDuration?}` objects (slice as needed) to report the latest messages.
- We have not yet found a reliable way to load older items—previous mouse wheel and JS scroll attempts have failed—so take note when history is truncated and report back for follow-up. Use `row.outerHTML` captures when you need to inspect structure before extracting new fields.

## MCP Messenger Voice Capture Playbook
- Ensure network logging stays enabled (`network_set_logging` with `{"enabled":true}` or `roderik netlog enable`) before triggering playback so audio fetches land in the structured log.
- Reuse the DOM walk script to identify the target voice messages and jot down sender, timestamp, and duration for filename metadata.
- Play the clip to force a fresh `Media` request; cached clips sometimes require a second play to register.
- Call `network_list` with filters like `{"type":["Media"],"mime":["audio"],"limit":20}` to surface recent audio fetches, then match by `response_timestamp`/URL against the DOM data.
- Save each clip with `network_save`, supplying the matched `request_id` plus `filename_prefix` (sender), `filename_timestamp` (true), `timestamp_format` (e.g., `2006-01-02_150405`), and `filename_suffix` (e.g., `voice` or duration). Files drop into `user_data/downloads/`; repeat per clip to adjust labels.

### Bulk Download & Labeling Shortcut
- Run a DOM extraction (`run_js`) that lists voice bubbles with timestamp slug, order, and duration.
- Fetch the same count of audio entries via `network_list` (`{"mime":["audio"],"type":["Media"],"limit":N,"tail":false}`). If counts differ, replay clips and retry.
- Pair entries in order and call `network_save`, passing a full `filename` such as `pe_today-at-4-52pm_order06_00m56s.ogg`; skipping `filename` leaves the default `get_<requestID>` chunk in the middle. **Reminder:** replaying an older clip can push its request to the top of `network_list`, so redo the DOM scrape if anything was re-played before saving.
