# Network Activity Logging Follow-Up

- 2025-10-28: `-n/--net-activity` is parsed once at startup. The Rod network event hooks always stay active and write to the in-memory event log, but stderr output only occurs while `ShowNetActivity` is true. Future work: add a runtime command/tool to flip the flag so operators can enable or disable network logging mid-session without restarting.

## User Story: Capture And Persist Network Artifacts Mid-Session

**As** an operator navigating with Roderik
**I want** to reach a target page, enable network activity logging on demand, and monitor for specific request URLs or MIME types
**So that** I can confirm expected resources load, capture their status codes, and optionally download or save the payloads to disk for later analysis.

### Notes
- Runtime controls should include: toggle logging on/off, filter for URL patterns or content types, and dump matching responses to a file or workspace directory.
- Downloads may require handling `Network.responseReceived` + `Network.getResponseBody` to extract payloads after the fact without re-issuing requests.

## Investigation: Filtering And Persisting Network Artifacts

- **Structured log model**: The current `EventLog` stores strings only. To filter by MIME type or filename we need a structured store keyed by request ID that captures URL, method, status, response headers, and timestamps. Rod events expose `e.Request.Headers`, `e.Response.MimeType`, and `e.Response.Headers` that we can persist alongside references to the original `RequestID`.
- **Response body capture**: Use `proto.NetworkGetResponseBody{RequestID: e.RequestID}` after `NetworkResponseReceived` (or `LoadingFinished`) to retrieve payloads. Responses can be base64-encoded; large bodies may need streaming to disk to avoid keeping everything in memory.
- **Filters & UI**: Build a `net activity` cobra subcommand (or MCP tool) that:
  - lists captured entries with optional filters (`--mime`, `--suffix`, `--status`, `--domain`) applied server-side;
  - optionally invokes `survey` to present a selectable list when run interactively. We’ll need to add `github.com/AlecAivazis/survey/v2` to `go.mod`.
- **Persistence workflow**: Upon selection, prompt for an output directory (default to the configured downloads directory, e.g. `<roderik base>/user_data/downloads/`) and write each payload to disk. Use the original filename if the URL path has one or derive from content-type + timestamp. Ensure we flush to disk and log the saved path for auditability.
- **Session toggle integration**: When the runtime toggle flips on, start storing bodies for future responses. For earlier requests recorded without body capture, fallback behavior should warn that no payload is available.
- **MCP considerations**: Expose equivalent tooling through MCP (`capture_network`, `filter_network`) so headless agent clients can request artifacts programmatically, returning metadata plus a resource reference for binary payloads.
- **Already-fetched media/assets**: Even when Chrome doesn’t fire a traditional download (e.g., streaming audio/video, inline JSON), the combination of the structured log plus `NetworkGetResponseBody`/`NetworkTakeResponseBodyAsStream` lets us persist those resources post-load. Hijacking (`rod.HijackRequests`) can duplicate or rewrite payloads in-flight so we capture playable media without interrupting playback.
- **HAR-style export**: Evaluate writing filtered records to disk (JSON or HAR) so operators can re-process the log offline. That archive should include metadata + pointers to saved payload files.

## Implementation Snapshot (2025-10-28)

- `roderik netlog` lists the structured log and supports filters via `--mime`, `--suffix`, `--status`, `--domain`, `--method`, and `--type`. Pass `--save` to persist matching entries; by default we prompt with a `survey` multi-select unless `--all` or `--interactive=false` is provided. Outputs are written to the configured downloads directory (defaults to `<roderik base>/user_data/downloads/`, override with `--output` or `RODERIK_DOWNLOAD_DIR`).
- `network_list` / `network_save` MCP tools expose the same functionality to agent clients. `network_list` returns JSON summaries for filtered entries; `network_save` streams the response body back (binary) or writes it to disk when `return=file`.
- Runtime toggles now work in-session: `roderik netlog enable|disable|status` flips or reports the logging flag without restarting, and the `network_set_logging` MCP tool mirrors the same capability for remote clients (omit `enabled` to query the current state).
- Response bodies are fetched lazily via `Network.getResponseBody` and cached in-memory per request. Existing stderr output for `-n/--net-activity` remains unchanged while the structured log accumulates metadata for filtering and persistence.
