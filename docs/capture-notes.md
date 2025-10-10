# Capture & Print Feature Notes

_Last updated: October 10, 2025_

This document tracks the ongoing work to let Roderik grab screenshots and PDF prints from the active Chrome session. The implementation is split into a shared Rod helper layer, CLI commands, and MCP tool exposure.

---

## Goals

- One capture pipeline usable from both the interactive CLI and the MCP server.
- Sensible defaults (local `captures/` folder, PNG screenshots, letter-sized PDFs).
- Configurable but concise flag surfaces—avoid option sprawl while surfacing the key knobs people expect (selector clips, full-page, paper size, margins, etc.).
- Allow MCP clients to decide whether they want inline base64 blobs or on-disk artifacts.

## Current Implementation

### Shared Helpers (`browser/capture.go`)

- `CaptureScreenshot(page, opts)` wraps `Page.Screenshot`, `ScrollScreenshot`, and `Element.Screenshot`.
  - Supports selector, full-page, scroll, custom clip, PNG/JPEG with optional quality.
- `CapturePDF(page, opts)` wraps `Page.PDF` and streams bytes back with paper size, scale, margin configuration.
- Returns a `Result` `{Data []byte, MimeType string}` so callers can choose to save or embed.

### CLI Commands (`cmd/capture.go`)

Two cobra subcommands hook into the shared helpers:

```
roderik screenshot [url]
roderik pdf [url]
```

Key flags (all optional):

- `--output, -o`: full path. Overrides other path flags.
- `--dir`: destination directory (`./captures` default).
- `--name`: file name stem; timestamps otherwise.
- `--format`: `png` (default) or `jpeg`.
- `--selector`: CSS selector capture (mutually exclusive with `--full-page/--scroll`).
- `--full-page`: expand viewport to full document height.
- `--scroll`: scroll & stitch capture without resizing.
- `--quality`: JPEG quality (default 90, auto-ignored for PNG).

PDF-only:

- `--size`: preset (letter, legal, tabloid, executive, A3, A4, A5) or custom `WIDTHxHEIGHT` inches (e.g., `210mmx297mm` is also accepted).
- `--margin`: uniform inch margin (defaults to 0.5").
- `--scale`: render scale multiplier (default 1.0).
- `--landscape`, `--header-footer`, `--background`, `--css-size`, `--tagged`, `--outline`.

Both commands will load the provided URL (if any) before capture; otherwise they operate on the currently loaded page. Output is printed as an absolute path plus size summary.

#### Tests

- `cmd/capture_test.go` exercises flag validation, option wiring, and file path resolution under fake capture functions.

### MCP Server Tools (`cmd/mcp.go`)

Tools `capture_screenshot` and `capture_pdf` mirror the CLI behaviour:

- Accept `url` to trigger navigation inside the tool.
- Reuse the shared helper structs.
- Parameters largely match the CLI flags but in snake_case (e.g., `full_page`, `quality`, `size`).
- `return` argument chooses `binary` (inline base64) vs. `file` (write locally + embedded resource reference). Large captures auto-switch to `file`.
- `output` lets the caller dictate the file path when `return="file"`.

Each tool always returns a companion text message describing where the artifact lives and the output size. When stored to disk the server includes a `resource` content item pointing at the local file URI with a base64 copy for clients that want a direct download.

#### Tests

- `cmd/mcp_capture_test.go` covers binary vs. file delivery, payload encoding, and Rod option propagation via fake capture functions.

## Open Items / Next Steps

- **User prompts for directories**: we currently rely on flags. We may add an interactive prompt (only when tty is interactive) for missing `--dir` if users request it.
- **Element clipping options**: we support selector capture; explicit viewport clips may need a flag if requested.
- **Compression controls**: consider surfacing width/height resizing for screenshots to keep payloads small.
- **Testing**: add fixture-driven tests once module downloads are available inside CI (current sandbox blocks `proxy.golang.org`).
- **Documentation surfacing**: integrate a shortened form of this doc into the main README once the feature stabilises.

Please extend this file as the capture pipeline evolves. Keep flag names in sync across CLI help, MCP schema, and docs.
