# AI Model Profile Setup

Roderik looks for model configuration in `<config-base>/ai-profiles.json`. The `<config-base>` directory is resolved via the shared appdirs helper:

- Linux: `${XDG_CONFIG_HOME:-$HOME/.config}/roderik`
- macOS: `~/Library/Application Support/roderik`
- Windows: `%AppData%\Roaming\roderik`

If the configuration directory cannot be determined, Roderik falls back to `~/.roderik`.

## Create the config directory

### Linux / macOS (bash, zsh)

```bash
CONFIG_BASE="${XDG_CONFIG_HOME:-$HOME/.config}/roderik"
mkdir -p "$CONFIG_BASE"
cp docs/ai-profiles.example.json "$CONFIG_BASE/ai-profiles.json"
```

### Windows (PowerShell)

```powershell
$configBase = Join-Path $env:APPDATA "roderik"
New-Item -ItemType Directory -Path $configBase -Force | Out-Null
Copy-Item "docs/ai-profiles.example.json" (Join-Path $configBase "ai-profiles.json") -Force
```

Each profile bundles together the provider choice, model ID, base URL, API key (inline via `api_key` or indirectly via `api_key_env`), optional system prompt, and max token limit. Select a profile at runtime with `roderik ai --model <profile-name>` (short form `-m`). To confirm the exact path your build is using, run `roderik ai --print-config-path`.

If both `api_key` and environment overrides are present, the inline key wins; keep using `api_key_env` when you prefer not to embed secrets in the file.
