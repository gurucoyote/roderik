# AI Model Profile Setup

Roderik looks for model configuration in `<config-base>/ai-profiles.json`, where `<config-base>` is the platform configuration directory (for example, `~/.config/roderik` on Linux, `~/Library/Application Support/roderik` on macOS, `%AppData%\Roaming\roderik` on Windows). If the configuration directory cannot be determined, it falls back to `~/.roderik`. Copy the sample file and adjust the values for your environment:

```bash
CONFIG_BASE="${XDG_CONFIG_HOME:-$HOME/.config}/roderik"
mkdir -p "$CONFIG_BASE"
cp docs/ai-profiles.example.json "$CONFIG_BASE/ai-profiles.json"
```

Each profile bundles together the provider choice, model ID, base URL, API key (inline via `api_key` or indirectly via `api_key_env`), optional system prompt, and max token limit. Select a profile at runtime with `roderik ai --model <profile-name>` (short form `-m`).

If both `api_key` and environment overrides are present, the inline key wins; keep using `api_key_env` when you prefer not to embed secrets in the file.
