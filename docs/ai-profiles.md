# AI Model Profile Setup

Roderik looks for model configuration in `~/.roderik/ai-profiles.json`. Copy the sample file and adjust the values for your environment:

```bash
mkdir -p ~/.roderik
cp docs/ai-profiles.example.json ~/.roderik/ai-profiles.json
```

Each profile bundles together the provider choice, model ID, base URL, API key (inline via `api_key` or indirectly via `api_key_env`), optional system prompt, and max token limit. Select a profile at runtime with `roderik ai --model <profile-name>` (short form `-m`).

If both `api_key` and environment overrides are present, the inline key wins; keep using `api_key_env` when you prefer not to embed secrets in the file.
