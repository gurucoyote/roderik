# Roderik AI Chat Planning

## Goals & Scope
- Add a first-party `roderik ai` CLI chat command (alias `chat`) with a built-in LLM conversation loop.
- Expose the full set of MCP tools already implemented in `cmd/mcp.go` so that the LLM can call them via standard tool/function calling.
- Support configurable conversation history retention (window length flag/config) with one in-memory session per CLI run (future-ready for persistence).
- Lay groundwork for named AI profiles (model, provider, credentials) but defer user-configurable profiles until after initial AI chat lands; default to OpenAI `gpt-5-codex` via environment variables.
- Build a dynamic system prompt that captures current CLI/browser context (active URL, focused element summary, flags) so the LLM can seamlessly pick up tasks.
- Restrict initial provider support to OpenAI-compatible APIs; ensure easy extension later.
- Re-use practical pieces from the `kai/` MCP host project when it accelerates development (history structures, tool adapters, provider implementations).
- Keep MCP server mode (`roderik mcp`) unchanged; avoid regressions in existing browsing commands.

## Constraints & Considerations
- Global CLI structure lives in `cmd/`; register the new `ai` command (alias `chat`) from `cmd/root.go`.
- Tool implementations live alongside MCP server helpers; refactor shared logic without breaking existing server behavior or MCP stdio mode.
- Conversation state stays in-memory for now; design `ChatSession` to optionally serialize in future.
- Profiles now live in `<config-base>/ai-profiles.json` (with `<config-base>` resolved via the shared appdirs helper—`~/.config/roderik` on Linux, `~/Library/Application Support/roderik` on macOS, `%AppData%\Roaming\roderik` on Windows, falling back to `~/.roderik`); ship a starter template in `docs/ai-profiles.example.json` and document the setup flow so users can copy it into place.
- Existing CLI uses `--profile` to pick the Chrome `user_data/` directory; AI chat will rely on a separate model-profile flag (`--model` / `-m`) for LLM configuration to avoid collisions.
- Respect existing logging/verbosity flags; ensure new command integrates with Cobra pattern and uses stderr for logs.
- Network access is required for OpenAI APIs—honor environment variables for keys; provide a mock/local provider hook for tests.
- Dynamic system prompt assembly must be efficient and accurate; avoid excess CDP calls when collecting session context.

## High-Level Architecture
- **Chat Command:** Cobra command `ai` (alias `chat`) remains the explicit entrypoint; users invoke `roderik ai <prompt>` (or run `roderik ai` interactively) rather than prefixed REPL shortcuts.
- **Profile Loader:** Layer that merges profile config + CLI flags + env overrides to produce a `ModelProfile` (OpenAI-compatible provider, model, base URL, API key, tool behavior).
- **Dynamic Context Builder:** Gathers live session state (current URL, focused element summary, navigation flags) and feeds it into the system prompt template before each LLM call.
- **History Manager:** Adapt or embed `kai/pkg/history` for message storage/pruning based on window size.
- **Provider Interface:** Reuse `kai/pkg/llm` abstractions or extract minimal subset into a shared package (e.g. `internal/ai/llm`) tailored to OpenAI-compatible APIs.
- **Tool Bridge:** Central registry under `internal/ai/tools` exposes tool metadata and handlers; chat and MCP layers consume adapters to sanitize names, list tools, and dispatch calls via shared helpers (wrapping `withPage`, etc.).
  - Focus-sensitive tools (e.g., `to_markdown`, `get_html`, `run_js`, `text`) should expose dynamic hints that reference the currently focused DOM element so models remember to adjust scope before scraping.
- **Execution Flow:** 
  1. Resolve model profile & load LLM provider.
  2. Instantiate tool registry (namespaced, sanitized names).
  3. Assemble base system prompt (static instruction blocks + dynamic context snapshot).
  4. Enter REPL loop: read user input, append to history, call provider with tool list + history window + system prompt.
  5. For each tool call returned, invoke dispatcher, append tool result messages, re-call LLM if needed until assistant text response is ready.
  6. Stream/print assistant text; loop until exit (`/quit`, EOF).
- **Config Surface (MVP):** 
  - Flags: `--model` (`-m`) and `--history-window`; consider `--system-prompt` later if profiles need runtime overrides.
  - Credentials & model: default model profile pulls from `<config-base>/ai-profiles.json` (see above for resolution) with env-variable overrides (`OPENAI_API_KEY`, `OPENAI_API_BASE`, `RODERIK_AI_MODEL`, etc.); ships with `gpt-5` default when unset.
- Future enhancement: reintroduce `<config-base>/config.toml` profiles once core chat loop is stable.

## Latest Status – October 27, 2025
- Implemented initial `roderik ai` / `roderik chat` Cobra command that wraps a `ChatSession` and reuses the shared MCP tool registry for tool dispatch; prompts require a single-turn message for now.
- Refactored the OpenAI provider to drop the `charmbracelet/log` dependency in favor of an injectable no-op logger and added `SetSystemPrompt`, letting the chat loop refresh instructions every tool iteration.
- System prompt builder now includes browser URL/title, focused-element metadata (with heading/link hints), current datetime, and guidance to stay on the loaded page unless the user explicitly requests external search.
- Chat session maintains in-memory history with a configurable `--history-window`; stored messages/tool results are trimmed to keep token usage lean while preserving the latest tool-call context.
- Chat loop now emits `[AI]` logs for each prompt, LLM iteration (with token counts), tool call, and tool result so operators can audit every step while watching stderr.
- Added `--logfile` flag so sessions can tee stdout, stderr, and user keystrokes into `roderik.log` (or a custom path) without breaking interactive mode.
- `roderik mcp` now advertises `tool_capabilities` support to connected MCP clients, letting the upcoming chat command invoke the shared tool registry without feature gating.
- The MCP server `--log` flag no longer has a short `-l` alias so future AI chat logging options can reuse the short form without conflicts.
- DuckDuckGo search tool is available for opt-in web lookups via the shared handler registry.
- `./cache-and-test.sh` passes, confirming the new command wires cleanly into existing tests and Windows cross-build.
- Inline `<tool_call>` markup from non-OpenAI models is now parsed into structured tool invocations so GLM-style models don't stall after emitting XML-ish text.
- When the loop hits the tool-iteration ceiling, we surface a user-facing fallback message summarizing the last tool result instead of crashing out of the REPL.
- Model profiles load from JSON (`<config-base>/ai-profiles.json`) with precedence `--model` flag → `RODERIK_AI_MODEL_PROFILE` → config default, and a tracked `docs/ai-profiles.example.json` bootstraps local setup alongside docs/ai-profiles.md.
- Inline or env-sourced API keys are supported (`api_key` wins over `api_key_env` / `OPENAI_API_KEY`); tests cover both flows.
- AI activity logs now mirror a human operator workflow (e.g., `AI ▶ duck query="…"`, `✔ duck → …`) while detailed iteration metrics remain behind `--verbose`.
- Each turn now ends with a concise summary of the tool chain plus running prompt/completion token totals (e.g., `tokens this turn 3.5k / 0.6k, total 128k`).

## System Prompt Outline
- **Static preamble:** brief description of Roderik’s purpose and workflow, lifted/adapted from existing MCP tool descriptions so the chat view aligns with MCP client expectations (emphasize browsing automation, careful navigation, safety).
- **Tool guidance:** compact list generated from the shared tool registry (`- tool_name: short description`), matching the wording exposed via MCP.
- **Dynamic context block:** minimal bullet list showing current URL/title, session flags, focused element summary, and last action; omit sections that are empty to save tokens.
- **Behavior reminders:** concise rules not already covered by individual tool descriptions (e.g., ask before destructive actions, keep answers tight when no action is needed).
- **Fallback instructions:** note how to proceed when no page is loaded or no element is focused (e.g., prompt user or call `load_url`).

## Implementation Plan (Phased)
1. **Repo Survey & Interfaces**
   - Catalogue current MCP tool registration (`cmd/mcp.go`) and identify extraction points (e.g., shared `ToolRegistry` struct).
   - Audit `kai/pkg` components for reusable bits (history, provider spec, tool name sanitizer).
   - Document chosen reuse/extraction approach (copy targeted files into `internal/ai/...` to keep Roderik self-contained; retain provenance notes).
2. **Shared MCP Tool Layer**
   - Extract tool metadata & handler glue from `cmd/mcp.go` into a shared internal registry (e.g., `internal/ai/tools`).
   - Each tool entry exposes metadata + a Go handler used by both the AI chat command and the MCP server.
   - Provide adapters: one that registers tools with the MCP server, and another that exposes sanitized tool specs/dispatch for the LLM call flow.
3. **AI Core Packages**
   - Introduce new internal packages for history management, profile handling, provider abstraction, and context building—reusing `kai/pkg/history` & `kai/pkg/llm` (copy or move to shared module while avoiding module import cycles).
   - Define `ai` package types: `Profile`, `ProfileManager`, `ChatSession`, `ContextBuilder` to encapsulate config/history/tool invocation/system prompt assembly.
4. **Profile Configuration**
   - Load from `<config-base>/ai-profiles.json`; seed from `docs/ai-profiles.example.json` if absent and add gitignore guidance (done).
   - Implement loader with precedence: CLI flag (`--model` / `-m`) → env override → config default.
   - Validate required fields (`provider`, `model`, and either `api_key` or `api_key_env`); allow optional `system_prompt` and `max_tokens`.
   - Add command `roderik ai profiles list` (optional, nice-to-have) or at least helpful errors.
5. **Chat Command Skeleton**
   - Create Cobra command `ai` (alias `chat`) with flags (`--model`, `--history-window`, verbosity reuse).
   - Keep the chat loop scoped to the dedicated Cobra subcommand; document the explicit invocation pattern instead of REPL-prefixed shortcuts.
   - Implement streaming output borrowing Kai’s line-buffered writer; support `/exit` or EOF to leave chat mode.
6. **Tool Invocation Loop**
   - Convert MCP tool schemas to provider tool definitions (namespacing + sanitization).
   - Implement dispatch that maps sanitized tool names back to actual MCP tool handlers.
   - Support multi-step tool/LLM loop until assistant yields text.
   - For focus-sensitive tools, append a runtime hint (current element summary + reminder to refocus on `<body>`/parent when full-page context is required).
7. **Dynamic Context & Prompt**
   - Expose helpers from existing browser state to capture: current URL/title, session flags (Desktop/Stealth, etc.), focused element summary (tag + key attrs + short text), and last user/browser action.
   - Compose a concise system prompt template that mirrors MCP tool guidance: static intro (summarise Roderik workflow, reuse wording from existing MCP tool descriptions), dynamic context block, and tool list derived from shared registry.
   - Ensure updates after navigation/element changes.
   - Provide hooks to refresh context each turn without excessive CDP calls.
8. **Testing & Validation (TDD)**
   - Write specs/tests first for new packages (history, context builder, tool registry adapters, chat command integration); add profile config tests once that feature returns.
   - Preserve existing CLI and MCP behavior: add regression tests or approval tests where feasible to ensure the shared tool registry continues to satisfy MCP expectations.
   - Provide integration smoke tests using a mock LLM provider that exercises tool calling and streaming output.
   - Run `./cache-and-test.sh` / `go test ./...` at each milestone to confirm no regressions.
9. **Docs & UX Polish**
   - Document new command in `README.md` / docs folder.
   - Update help text with usage, environment variables, default model, and sample system prompt structure.

## Implementation Roadmap (Incremental & Always Usable)
1. **Foundation Tests & Tool Registry Extraction**
   - Write failing tests covering tool registry adapter expectations (metadata parity, handler dispatch).
   - Extract existing MCP tool implementations into `internal/ai/tools` while keeping `cmd/mcp.go` functional; update MCP command to use the shared registry and confirm `go test ./cmd/...` passes.
2. **Minimal Chat Command Skeleton**
   - Introduce `roderik ai` command wiring into existing REPL with stubbed provider (mock returning canned responses).
   - Provide tests for command routing (`ai ...` vs normal commands) and streaming writer behavior.
   - Ship a basic dynamic prompt stub (static text) so we can manually trigger AI interactions while other components are still TODO.
3. **OpenAI Provider Integration**
   - Port/copy required provider/history code from `kai` with tests.
   - Replace mock provider in chat command with real OpenAI-compatible client; rely on `OPENAI_API_KEY` env var and `gpt-5-codex` default model.
   - Validate with a smoke test using a fake HTTP server (no real API calls in CI).
4. **Dynamic Context Builder**
   - Add tests for context snapshots (URL, flags, focused element, last action).
   - Wire into system prompt template; confirm streaming flow works end-to-end with mock provider.
5. **Tool Call Loop Enablement**
   - Implement tool sanitization/dispatch for AI chat using shared registry; add test covering a sample tool call path.
   - Run integration test ensuring AI command can invoke at least one tool and handle tool results.
6. **Polish & Regression Sweep**
   - Flesh out behavior reminders/system prompt wording.
   - Update docs, help text, and release notes with MVP usage guidance.
   - Final `./cache-and-test.sh`, `go test ./...`, and manual smoke to confirm CLI + MCP remain stable.

*Post-MVP:* Revisit profile loader/config file support and extend tests/documentation accordingly.

## Open Questions / Follow-ups
- 🔜 Revisit profile config schema once we reintroduce user-defined profiles post-MVP.
- ✅ Dynamic system prompt breadth: provide a concise context summary (URL + key element info + brief text) to stay mindful of token usage.
- ✅ Streaming output expectations: stream tokens; reuse Kai’s line-buffered word-wrap behavior so we flush full lines only.
- ✅ Session persistence commands: defer `/save`/`/load` until needed.
- ⚠️ Tool iteration heuristics: still need smarter detection (e.g., repeated zero-result selectors) to avoid hitting the fallback response.

## Risks & Mitigations
- **Operator visibility gaps**: current `[AI]` log lines are tuned for debugging (tool args/results). Provide a user-focused activity summary channel before promoting to broader testing.
- **Complex refactor of MCP tool definitions**: ensure incremental change, first add shared layer then adapt existing command with tests.
- **API credential handling**: rely on env vars (e.g., `OPENAI_API_KEY`) by default; document setup and warn against committing secrets when profile config returns.
- **Function-call mismatch**: validate tool schema, add integration test that runs end-to-end tool call using mock provider to prevent regressions.
- **Dependency duplication with `kai` module**: copy needed pieces into `internal/ai/...` with provenance notes to keep projects independent.
- **Dynamic prompt drift**: ensure session context snapshot stays fresh (especially after navigation) by subscribing to existing event hooks rather than polling.
- **Context completeness**: limit prompt metadata to URL/title, session flags, focused element summary, and last action to avoid bloat.
- **Instruction duplication**: keep system prompt guidance unique; rely on per-tool descriptions for detailed usage to avoid redundant tokens.
- **Iteration fallback complacency**: the new safety message prevents REPL crashes but may mask navigation loops; add heuristics or telemetry to detect repeated failing selectors.


## Work In Progress (Paused)
- Shared MCP tool dispatcher now handles navigation, inspection, markdown, run_js, and capture tools for both MCP and upcoming AI chat.
- `internal/ai/llm` and `internal/ai/history` copied from Kai; imports rewired to local packages.
- OpenAI provider is mid-update: need to remove `charmbracelet/log` usage in `internal/ai/llm/openai/provider.go` (replace with no-op debug) before continuing.
- Next steps: finish provider cleanup, hook `tools.LLMTools` into chat command, wire AI REPL routing (`ai`/`chat` prefix) into existing CLI loop.
