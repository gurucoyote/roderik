# User Flow Recording & Playback – Roderik CLI & MCP

## User Story
As an automation engineer using Roderik in GUI mode, I want to open a scripted baseline session and then hand control to a human operator so they can demonstrate a workflow inside the browser. While the human completes the task, I want Roderik to record each meaningful action (navigation, clicks, typing, form submissions, console/network artifacts). Later I can replay those steps or inspect them in detail to derive an automated scenario and diagnose issues, without having to manually transcribe the operator’s actions.

### Acceptance Criteria
- Operator can trigger “record mode” from both the CLI (`roderik --record`) and MCP (`start_recording`).
- Recording captures ordered actions with metadata: tool name, arguments, DOM context (selectors/XPath), timestamps, and outcomes (success/error, console/network deltas).
- Operator can stop recording and persist a JSON/YAML artifact under `user_data/flows/` with a discoverable name.
- Recorded flows are listable (`list_recordings`), inspectable (`show_recording <id>`), and executable (`play_recording <id>`) from CLI and MCP.
- Replay runs headless or attached to desktop sessions, honoring waits and navigation timing collected during recording.
- Network requests, console messages, and screenshots are optionally bundled with each step for debugging.

## Implementation Brainstorm (Leveraging Chrome Recorder)

### 1. Bridge to Chrome Recorder
- **CDP Control:** Use DevTools Protocol commands exposed in `Page`/`Runtime` domains to launch the Recorder panel programmatically (`Runtime.evaluate` on `RecorderApp.instance()`) and to control recording lifecycle (`startRecording`, `stopRecording`, `getUserFlows`).
- **Recorder Session Wrapper:** Introduce a `recorder` package that encapsulates CDP calls, normalizes the Recorder JSON output, and translates it into Roderik’s flow format.
- **Desktop & Headless Support:** When attached to Windows Chrome, connect via existing DevTools socket; in headless mode, launch Chrome with `--auto-open-devtools-for-tabs` to enable Recorder backend even without UI, falling back to scripted simulation if Recorder API unavailable.

### 2. Data Capture & Harmonization
- **Native Artifact Import:** Store the raw Recorder JSON (including steps, assertions, selectors) alongside a Roderik-friendly projection that maps steps to MCP tool equivalents.
- **Augmented Metadata:** Merge Recorder events with Roderik’s network/console logs (from `registerPageEvents` in `cmd/root.go:120`) so each step references associated requests, console messages, or screenshots.
- **Schema Strategy:** Maintain dual files—`<flow>.recorder.json` (verbatim Chrome output) and `<flow>.roderik.json` (enriched, replay-ready data with schema versioning).

### 3. CLI Workflow
- **Commands:** Extend the CLI with `roderik recorder start`, `roderik recorder stop`, `roderik recorder list`, `roderik recorder show`, and `roderik recorder play`.
- **Guided Session:** When `start` is invoked, Roderik ensures the target page is prepared, switches focus to user control, and displays status banners (including Recorder availability checks).
- **Playback Runner:** Implement replay that reads the enriched flow and issues corresponding MCP tool calls; provide `--source=recorder|roderik` and `--speed` flags to control playback.

### 4. MCP Extensions
- **Control Tools:** Add `recorder_start`, `recorder_stop`, `recorder_get`, `recorder_list`, and `recorder_play` so external agents can drive the workflow.
- **Macro Execution:** Implement a `macro` tool that consumes the enriched Roderik flow JSON; Recorder output is first translated via the bridge and then executed through existing actions (`click`, `type`, `run_js`).
- **Event Access:** Expose `recorder_artifacts` to fetch network/console/screenshot data linked to specific Recorder steps.

### 5. Replay & Analysis
- **Selector Fidelity:** Use Recorder-provided selectors (prefer `aria`/`text` fallbacks) and validate them during playback; report mismatches with suggested updates.
- **Timing Controls:** Respect Recorder timing cues (waitForNavigation, pauseAfter, etc.), but allow overriding durations for faster or slower replays.
- **Reporting:** Generate Markdown/JSON summaries that include step outcomes, timing, network metrics, and console logs for debugging.

### 6. Testing & Validation
- **Unit Tests:** Mock the Recorder bridge to verify command handling, JSON translation, and persistence logic.
- **Integration Tests:** Use `./cache-and-test.sh` with headless Chrome to execute a miniature recording (via simulated CDP commands), persist it, and replay it end-to-end.
- **Manual QA:** Document a desktop Chrome flow: script baseline navigation → run `roderik recorder start` → perform actions manually → stop → replay; capture screenshots for docs.

### 7. Incremental Delivery
1. Build the Recorder bridge (CDP wrappers) and `recorder_start/stop/get` primitives.
2. Persist raw Recorder output and basic metadata; expose listing/show commands.
3. Implement Recorder→Roderik translation plus `macro` playback for a limited subset (navigate/click/type).
4. Expand translation coverage (form submissions, key events, assertions) and attach network/console artifacts.
5. Polish CLI/MCP UX, add reporting, and harden selector/timing reconciliation.
