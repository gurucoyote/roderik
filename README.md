Roderik is a command-line tool that allows you to navigate, inspect, and interact with elements on a webpage. It uses the Go Rod library for web scraping and automation. You can use it to walk through the DOM, get information about elements, and perform actions like clicking or typing.

Usage:
  roderik [flags]
  roderik [command]

Available Commands:
  body        Navigate to the document's body
  box         Get the box of the current element
  child       Navigate to the first child of the current element
  click       Click on the current element
  completion  Generate the autocompletion script for the specified shell
  elem        Navigate to the first element that matches the CSS selector
  head        Navigate to the first heading of the specified level, or any level if none is specified
  help        Help about any command
  html        Print the HTML of the current element
  next        Navigate to the next element
  parent      Navigate to the parent of the current element
  prev        Navigate to the previous element
  rclick      Right click on the current element
  text        Print the text of the current element
  type        Type text into the current element
  walk        Walk to the next element for a number of steps

Flags:
  -h, --help   help for roderik

Use "roderik [command] --help" for more information about a command.

##Status
As of now, this is very muc a WiP.
It kida already works, with most basic interaction and inspection commands present.
It very much needs refining and better error handling etc.

Recent reliability tweaks:
- Heading discovery (`head`, initial load) now evaluates the DOM through an inline function, preventing Rod's cached helper from occasionally disappearing and halting navigation.
- Multiple `roderik` instances can now run side by side by falling back to disposable Chrome user-data profiles when the shared profile is locked, avoiding singleton panics.

## Data Directories
- Persistent state now defaults to the system config directory (e.g. `$XDG_CONFIG_HOME/roderik` on Linux/macOS, `%AppData%\Roaming\roderik` on Windows) or `~/.roderik` when the config path is unavailable.
- Browser profiles live under `<base>/user_data`, temporary fallbacks are created inside the same directory, and network captures default to `<base>/user_data/downloads`.
- MCP logs are written to `<base>/logs/roderik-mcp.log` unless you override the path via `--log`.
- Override the defaults with `RODERIK_HOME` (sets the base), or the more specific `RODERIK_USER_DATA_DIR`, `RODERIK_LOG_DIR`, and `RODERIK_DOWNLOAD_DIR` environment variables when you need per-project storage.

## MCP Server Overview

Roderik ships with an MCP server (`go run ./cmd/mcp.go`) that mirrors the CLI commands so agents can drive a shared browser session over stdio. Recent behaviour to keep in mind when wiring a client:

- `load_url` is now enabled by default and should be called before any DOM work. When disabled via `RODERIK_ENABLE_LOAD_URL=0`, the navigation helpers are also withheld so clients don't attempt stale operations.
- The element discovery tools (`search`, `head`, `elem`) return numbered summaries of the matches and highlight the currently focused index. Follow-up navigation commands can jump directly to the `n`th element by passing `index` to `next`/`prev`.
- `child`/`parent` reuse the same focus list, so the numbered summaries stay in sync as you traverse the DOM.
- `html` emits the outer HTML of the focused node; use this after narrowing to the desired index.
- `computedstyles` returns the focused element’s computed CSS as JSON, matching the `roderik computedstyles` CLI output.
- `click` and `type` mirror the CLI behaviour, reuse the shared focus list, and report whether fallbacks were needed (href navigation or JS value injection).
- `run_js` now requires an already-selected element—it no longer accepts a `url` parameter. Clients should `load_url` and navigate before running scripts.
- When the MCP server is started with `--desktop`, the Windows Chrome session is launched lazily: the GUI only appears once a tool actually needs the browser, avoiding unnecessary pop-ups for non-browsing sessions.

## Similar

This is a successor to my earlier attempt, called willbrowser, which was written in nodejs and the playwright framework. https://github.com/gurucoyote/willbrowser
