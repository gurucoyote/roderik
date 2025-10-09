# Profile Management Specification

This document captures the planned improvements for handling Chrome/Chromium profiles when running Roderik, with emphasis on desktop sessions that attach to Windows Chrome.

## Goals

- Allow users to choose which browser profile Roderik should use, instead of hard-coding `WSL2`.
- Provide an interactive picker that lists available profiles and lets users create new ones.
- Support scripted usage via a `--profile` flag while still offering interactive selection.
- Automatically rename the Windows profile so Chrome windows show a recognizable title.
- Keep headless/Linux launches in sync with desktop behavior.

## User-Facing Changes

1. **Flags**
   - `--profile`: explicitly pick a profile directory (omit to be prompted).
   - `--profile-title`: friendly name to write into the Chrome profile metadata (Windows only); defaults to the `--profile` value when supplied.

2. **Interactive Picker**
   - Implemented with `github.com/AlecAivazis/survey/v2`.
   - Triggered automatically when no `--profile` is provided.
   - Lists existing profiles with friendly names (from `Local State`) and directory identifiers.
   - Includes an option to “Create new profile…”, prompting for name and directory.

3. **Profile Discovery**
   - Windows desktop: enumerate `%USERPROFILE%\AppData\Local\Google\Chrome\User Data/*`.
   - Linux/macOS: enumerate default Chrome config directories plus the current `user_data/<profile>`.
   - Profiles are considered valid when they contain `Preferences`, `Local State`, or `Default`-style content.

4. **Profile Renaming**
   - After selecting a profile and before launching Chrome, update the `profile.info_cache.<dir>.name` entry in `Local State` when `--profile-title` is supplied.
   - Mark `is_using_default_name = false`.
   - If the metadata file cannot be edited, issue a warning but proceed.

5. **Documentation**
   - Update `AGENTS.md` with an overview of profile selection.
   - Add user-facing guidance on how to select, create, and rename profiles.

## Implementation Outline

1. **Flag Wiring**
   - Add the new persistent flags in `cmd/root.go`.
   - Store the resolved selection in a shared struct that both `PrepareBrowser` and `connectToWindowsDesktopChrome` can access.

2. **Profile Enumeration Helpers**
   - Shared helper to load `Local State` (JSON) and map directory → friendly name.
   - Functions to validate existence and create directories when requested.
   - Unit tests covering parsing of sample `Local State` files (`test/` fixtures).

3. **Interactive Picker**
   - Survey-based prompt that displays `Friendly Name (Directory)` entries.
   - “Create new profile…” choice prompts for:
     - Friendly name (required).
     - Directory slug (default derived from name).
   - Created profiles get their directory scaffolded where appropriate; Roderik’s launcher will initialize the rest.

4. **Profile Metadata Update**
   - Windows-only: mutate `Local State` so Chrome surfaces the friendly name.
   - Ensure the write happens before the `cmd.exe /C start` invocation.
   - Consider using a small helper package to encapsulate read/modify/write logic; include tests with sample JSON.

5. **Integration**
   - Both `PrepareBrowser` (Linux headless) and `connectToWindowsDesktopChrome` (desktop) consume the resolved path.
   - Update log messages to include the chosen profile for troubleshooting.
   - Extend `cache-and-test.sh` to ensure the Windows build still compiles with the new dependencies.

6. **Future Enhancements (optional)**
   - `roderik profiles list`: CLI subcommand to list available profiles without launching Chrome.
   - Persist “last used” profile per host in a config file (`~/.config/roderik/config.json`) and auto-select unless overridden.
   - Autocomplete for `--profile` based on discovered directories.

## Risks & Mitigations

- **Metadata corruption**: Always back up `Local State` before writing, or write atomically via temporary files.
- **Permission issues**: Warn if the process lacks rights to edit the profile directory; skip renaming rather than failing the launch.
- **Survey dependency**: Ensure the dependency is vendored or documented, and the picker degrades gracefully when stdin is non-TTY.

## Testing Strategy

- Unit tests for profile enumeration and `Local State` parsing.
- Tests for new flag parsing logic.
- Integration test (tagged) that simulates the survey prompt via injected stdin.
- Manual validation: run `./cache-and-test.sh`, then `roderik --desktop` on both Linux (headless) and Windows builds to exercise the interactive prompt.
