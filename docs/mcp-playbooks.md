# MCP Playbooks

## Messenger DOM Walk
- Load Facebook via `gui-browser/load_url` and have the operator open the target Messenger conversation.
- Verify the iframe is accessible by running `gui-browser/head` (level `4` works well) and watching for timestamp headers such as “Today at 3:24 PM”.
- Use `gui-browser/run_js` to scan `div[role="row"]` elements:
  - Rows containing an `h4` are timestamp separators; cache their text for the messages that follow.
  - For other rows, read the sender from the first direct `span`, collect any message bodies from `div[dir="auto"]` spans, and detect audio clips by matching `row.innerText` against a `\d+:\d{2}` duration or by checking for audio controls.
  - Return an array of entries shaped like `{sender, timestamp, text?, audioDuration?}` and slice/filter as needed before presenting to the user.
- If the desired message hasn’t rendered yet, note that our scrolling attempts (`mouse wheel` or small JS `scrollBy`) currently fail to load older history; a working fallback is still outstanding.
- When the DOM layout is unclear, capture `row.outerHTML` with `run_js` to inspect the structure before refining selectors.

## Messenger Voice Message Capture
- Before playback, make sure network logging is on (`network_set_logging` with `{"enabled":true}` or the CLI `roderik netlog enable`) so audio fetches land in the structured event log.
- Run the Messenger DOM walk (`gui-browser/run_js`) to locate the target voice rows and note their `{sender, timestamp, audioDuration}` so you can label the downloads later.
- Trigger Messenger to stream the clip (e.g., press the play button). The first fetch usually arrives as a `Media` resource with an `audio/*` MIME type; if it was cached earlier, replaying forces Chrome to re-request it and emit a fresh log entry.
- List matching requests with `network_list` — typical filter payload: `{"type":["Media"],"mime":["audio"],"tail":true,"limit":20}`. Verify the entry’s `response_timestamp` and duration align with the DOM metadata you captured.
- Persist each clip via `network_save`, passing the request ID from the listing plus naming options: set `filename_prefix` to the sender (sanitized), enable `filename_timestamp` with a format such as `2006-01-02_150405`, and use `filename_suffix` like `voice` or the duration (e.g., `00m32s`). Example args: `{"request_id":"1234.5","filename_prefix":"alice","filename_suffix":"voice","filename_timestamp":true,"timestamp_format":"2006-01-02_150405"}`.
- Saved files default to `user_data/downloads/`; repeat the `network_save` call per voice message so each gets the correct sender/time label. If multiple clips share the same sender, rerun with updated suffixes rather than relying on one bulk save.

> For a deeper R&D experiment that mines Messenger’s existing GraphQL traffic (still only observing what the browser already fetched) to improve labeling and timestamping of voice messages, see `docs/messenger-graphql-rnd.md`.

### Messenger Voice Message Bulk Download & Labeling
- With Messenger open on the desired thread, run a DOM sweep (`run_js`) that collects voice bubbles in page order, capturing the timestamp label (e.g. “Today at 4:52PM”), derived slug (`timestampKey`), and duration (seconds + `00m00s` format). Store this list for pairing.
- Call `network_list` with `{"mime":["audio"],"type":["Media"],"limit":N,"tail":false}` (set `N` to the number of voice bubbles you saw) to retrieve the audio requests in chronological order. Confirm the count matches your DOM list; if not, re-play clips until it does.
- Zip the DOM list with the network entries top-to-bottom. For each pair, build a full filename like `pe_today-at-4-52pm_order06_00m56s.ogg` and pass it via the `filename` argument to `network_save`; otherwise the default base (`get_<requestID>`) will stay in the middle of the name. Keep `filename_timestamp` off, since the timestamp is already encoded in the slug. **Caution:** Messenger can re-request older clips when you replay them, reshuffling their appearance in `network_list`; re-run the DOM extraction after any playback and double-check pairing before saving.
- Saved clips land under `user_data/downloads/` (e.g., `C:\Users\<user>\AppData\Roaming\roderik\user_data\downloads`). Verify file sizes roughly match expectations (longer clips → larger files). Re-run `network_save` if any were missing bodies.
- Record the mapping (order, request ID, timestamp, duration, file path) so operators can trace each audio file back to its chat bubble.
