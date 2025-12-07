# Messenger Voice Messages – GraphQL R&D Notes

## User Story

- As an operator using the MCP Messenger playbooks, I want a more reliable way to label and download voice messages from a Messenger thread so that:
  - Timestamps and sender labels come from stable, structured data rather than brittle DOM heuristics.
  - The mapping between each audio file and its chat bubble is deterministic, even if Messenger reshuffles network requests or changes its DOM layout.
  - We only reuse data that the browser has already fetched during a normal, interactive session (no new API calls, no headless scraping).

## Constraints & Principles

- **Reuse existing traffic only**
  - Do not introduce any new HTTP requests beyond what the Messenger web app already issues.
  - Treat DevTools network logging as an observation tool over the live browser, not as a client.
- **Stay within the current MCP model**
  - Continue to rely on:
    - `gui-browser/load_url`, `head`, and `run_js` for DOM‑side inspection.
    - `network_set_logging`, `network_list`, and `network_save` for network capture and persistence.
  - Do not introduce separate headless clients, custom access tokens, or direct GraphQL POSTs.
- **Schema and operations can change**
  - Messenger’s GraphQL schema and operation names are private and unstable.
  - Any experiment must:
    - Discover relevant operations dynamically where possible.
    - Rely on robust, defensive parsing of JSON rather than hard‑coding a specific query shape.
- **Audio bytes remain in `Media`/audio responses**
  - Raw audio should still be downloaded via existing `Media` / `audio/*` responses and `network_save`.
  - GraphQL is a metadata source (message IDs, timestamps, sender, attachment metadata), not a replacement for the audio fetch path.

## High‑Level Idea

Today’s playbooks:

- Use the DOM to infer `{sender, timestamp, audioDuration}` for each voice bubble.
- Use DevTools network logs to find `Media` / `audio/*` responses when a clip is played.
- Pair DOM entries to network entries (by order and approximate duration), then call `network_save` with a derived filename.

Proposed refinement:

- Continue using DevTools logging, but:
  - Treat **Messenger GraphQL responses** as the primary truth for:
    - Message IDs and server timestamps.
    - Sender IDs / names.
    - Attachment type and duration (where present).
  - Treat **`Media` / `audio/*` responses** as the primary source of bytes.
  - Join them using URLs or attachment IDs that appear in both the GraphQL payloads and the network entries.
- The DOM remains useful for:
  - Sanity‑checking a few samples.
  - Helping an operator visually confirm which bubble corresponds to which audio file.

## R&D Experiment – Step‑By‑Step

This is intended as a manual/assisted R&D script using existing tools (no new code required at first). The goal is to learn the actual shapes of Messenger’s GraphQL payloads for the current account and browser.

### 1. Prepare the Session

- Start a normal roderik session against Facebook:
  - `roderik https://www.facebook.com/messages`
- Ensure:
  - The desired Messenger thread is open and visible.
  - You can see at least a few voice messages in the current viewport.

### 2. Enable Network Logging

- Turn on network logging using the existing playbook commands:
  - `network_set_logging` with `{"enabled":true}` or
  - CLI helper: `roderik netlog enable`
- Confirm that:
  - Requests and responses start appearing for page navigation, static assets, and Messenger background traffic.

### 3. Capture GraphQL Traffic While Scrolling

- With the thread open:
  - Slowly scroll up and down enough to trigger Messenger’s normal “load older / newer messages” behavior.
  - Do not inject any new requests; just use the GUI scroll.
- After a short scroll session:
  - Use `network_list` to find likely GraphQL endpoints, for example:
    - Filter by `{"type":["Fetch","XHR"],"limit":50}`.
    - Then manually inspect entries whose URLs include `/api/graphql` or similar.
- For 1–3 representative entries:
  - Capture:
    - `url`
    - `request_headers` and `request_body` (to see operation names and variables).
    - `response_body` (JSON).
  - Store these locally (e.g., paste snippets into a scratchpad or issue‑specific notes) so we can study field names and structures.

### 4. Identify Message & Attachment Fields

- In the captured GraphQL response body:
  - Locate the section representing messages in the active thread, typically:
    - An array of message nodes.
    - Or edges with `node` objects.
  - For a small subset of messages (especially ones with visible voice clips), record:
    - Message ID / key (e.g., `message_id`, `id`).
    - Sender ID or name field(s).
    - Timestamp field (often in ms or seconds).
    - Any attachment or blob object associated with the message:
      - Attachment type (voice vs image vs file).
      - Duration for audio/voice messages, if present.
      - URL or attachment ID that looks like it could correspond to the audio fetch.
- Goal of this step:
  - Produce a **minimal field map** for voice messages, something like:
    - `message_id`
    - `timestamp_ms`
    - `sender_name` / `sender_id`
    - `attachment_type`
    - `voice_duration_seconds`
    - `voice_cdn_url` or `voice_attachment_id`

### 5. Correlate with `Media` / Audio Responses

- In the same session (no refresh):
  - Play each of a few known voice messages in the thread.
- After or during playback:
  - Use `network_list` to filter for likely audio responses:
    - `{"mime":["audio"],"type":["Media"],"limit":20}` or similar.
  - For each matching entry, inspect:
    - `url` (host, path, query params).
    - Any identifiers that look like attachment IDs or message IDs.
    - `response_timestamp` and size (an approximate check for duration).
- Attempt to match:
  - For each voice message in the GraphQL data from step 4, see if:
    - The `voice_cdn_url` (or similar field) matches or closely resembles one of the `Media` URLs.
    - Or, a specific attachment ID from GraphQL appears as a parameter in the `Media` URL.
- Record one or more concrete mappings like:
  - `message_id` → `voice_cdn_url` → `network request_id`
- Even if we only find a partial or approximate mapping, document it (this will guide later automation).

### 6. Prototype a Labeling Scheme (Manual)

- Using only the discovered fields:
  - For each successfully matched voice message, manually propose a filename of the form:
    - `<sender>_<timestamp>_<messageId[short]>_<duration>.ogg`
    - Where:
      - `sender` comes from GraphQL.
      - `timestamp` uses the server timestamp (normalized, e.g., `YYYY-MM-DD_hhmmss`).
      - `messageId[short]` can be a truncated ID for debugging.
      - `duration` comes from GraphQL if available, else omitted.
- For each tested clip:
  - Call `network_save` with:
    - `request_id` from the audio `Media` entry.
    - A full `filename` string derived from the scheme above.
- Verify:
  - Saved files land in `user_data/downloads/`.
  - Names contain reliable sender + timestamp info independent of DOM formatting (“Today at 3:24 PM” vs absolute times).

### 7. Record Findings & Open Questions

- At the end of the experiment, summarize:
  - Which GraphQL fields reliably identify voice messages.
  - How stable operation names and response shapes appeared across:
    - A single session.
    - (If possible) multiple reloads.
  - Whether a deterministic mapping from message objects to `Media` URLs is achievable.
- List open questions for a future implementation:
  - Do we need to track pagination cursors to support older messages?
  - Are there cases where the audio fetch happens via a different endpoint than the CDN URL in GraphQL?
  - How often do operation names and field paths change in practice?

## Future Automation Sketch (Non‑Blocking)

If the above R&D experiment is promising, a later implementation could:

- Add a helper that:
  - Reads relevant GraphQL responses from `network_list` (no new outbound calls).
  - Extracts a compact `[]VoiceMessageMeta` slice for the active thread.
  - Joins that slice with recent `Media` / `audio/*` entries in the log.
  - Emits a structured list of `{sender, timestamp, duration, request_id, suggestedFilename}` for consumption by higher‑level tools.
- The key design rule remains:
  - The helper only **mines** data that Messenger has already fetched in the browser session; it never issues its own GraphQL or audio requests.

This file is intentionally high‑level and exploratory. The next time we revisit this, we can:

- Add concrete JSON field paths based on actual captured payloads.
- Decide where to embed the logic (standalone MCP helper vs. CLI command).
- Update `docs/mcp-playbooks.md` with a shorter “production” variant once we trust the approach.

