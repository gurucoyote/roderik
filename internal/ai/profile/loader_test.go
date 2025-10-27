package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeEnv map[string]string

func (f fakeEnv) Get(key string) string {
	return f[key]
}

type fakeFS struct {
	files map[string]string
	err   error
}

func (f fakeFS) ReadFile(path string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	if data, ok := f.files[path]; ok {
		return []byte(data), nil
	}
	return nil, os.ErrNotExist
}

func TestLoaderUsesEnvWhenNoConfig(t *testing.T) {
	env := fakeEnv{
		"OPENAI_API_KEY":        "env-key",
		"OPENAI_API_BASE":       "https://api.example.com/v1",
		"RODERIK_AI_MODEL":      "env-model",
		"RODERIK_AI_PROVIDER":   "openai-env",
		"RODERIK_AI_MAX_TOKENS": "2048",
	}

	loader := Loader{
		Getenv:   env.Get,
		ReadFile: fakeFS{}.ReadFile,
	}

	prof, err := loader.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if prof.Provider != "openai-env" {
		t.Fatalf("expected provider override, got %q", prof.Provider)
	}
	if prof.Model != "env-model" {
		t.Fatalf("expected model override, got %q", prof.Model)
	}
	if prof.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("expected base URL override, got %q", prof.BaseURL)
	}
	if prof.APIKey != "env-key" {
		t.Fatalf("expected API key override, got %q", prof.APIKey)
	}
	if prof.MaxTokens != 2048 {
		t.Fatalf("expected max tokens override, got %d", prof.MaxTokens)
	}
}

func TestLoaderReadsConfigDefaultProfile(t *testing.T) {
	configPath := filepath.FromSlash("/tmp/config.json")
	fs := fakeFS{files: map[string]string{
		configPath: `{
            "default": "openai-dev",
            "profiles": {
                "openai-dev": {
                    "provider": "openai",
                    "model": "gpt-5.1",
                    "base_url": "https://dev.example.com/v1",
                    "api_key_env": "DEV_OPENAI_KEY",
                    "max_tokens": 1024
                }
            }
        }`,
	}}

	env := fakeEnv{
		"DEV_OPENAI_KEY": "config-env-key",
	}

	loader := Loader{
		ConfigPath: configPath,
		Getenv:     env.Get,
		ReadFile:   fs.ReadFile,
	}

	prof, err := loader.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if prof.Name != "openai-dev" {
		t.Fatalf("expected profile name openai-dev, got %q", prof.Name)
	}
	if prof.Model != "gpt-5.1" {
		t.Fatalf("expected model gpt-5.1, got %q", prof.Model)
	}
	if prof.APIKey != "config-env-key" {
		t.Fatalf("expected API key from env ref, got %q", prof.APIKey)
	}
	if prof.BaseURL != "https://dev.example.com/v1" {
		t.Fatalf("expected base URL from config, got %q", prof.BaseURL)
	}
	if prof.MaxTokens != 1024 {
		t.Fatalf("expected max tokens from config, got %d", prof.MaxTokens)
	}
}

func TestLoaderEnvOverridesConfig(t *testing.T) {
	configPath := filepath.FromSlash("/tmp/config.json")
	fs := fakeFS{files: map[string]string{
		configPath: `{
            "profiles": {
                "alpha": {
                    "provider": "openai",
                    "model": "gpt-4",
                    "api_key_env": "ALPHA_API_KEY"
                }
            }
        }`,
	}}

	env := fakeEnv{
		"RODERIK_AI_MODEL_PROFILE": "alpha",
		"ALPHA_API_KEY":            "env-key",
		"RODERIK_AI_MODEL":         "env-model",
	}

	loader := Loader{
		ConfigPath: configPath,
		Getenv:     env.Get,
		ReadFile:   fs.ReadFile,
	}

	prof, err := loader.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if prof.Model != "env-model" {
		t.Fatalf("expected model env-model, got %q", prof.Model)
	}
	if prof.APIKey != "env-key" {
		t.Fatalf("expected API key env-key, got %q", prof.APIKey)
	}
}

func TestLoaderMissingSelectionErrors(t *testing.T) {
	configPath := filepath.FromSlash("/tmp/config.json")
	fs := fakeFS{files: map[string]string{
		configPath: `{"profiles": {"alpha": {"model": "gpt-4"}}}`,
	}}

	loader := Loader{
		ConfigPath: configPath,
		Getenv:     fakeEnv{}.Get,
		ReadFile:   fs.ReadFile,
	}

	_, err := loader.Load("beta")
	if err == nil {
		t.Fatalf("expected error for missing profile")
	}
	if !strings.Contains(err.Error(), "beta") {
		t.Fatalf("expected error to reference profile name, got %v", err)
	}
}

func TestLoaderUsesInlineAPIKeyWhenProvided(t *testing.T) {
	configPath := filepath.FromSlash("/tmp/config.json")
	fs := fakeFS{files: map[string]string{
		configPath: `{
            "profiles": {
                "alpha": {
                    "provider": "openai",
                    "model": "gpt-4",
                    "api_key": "inline-key"
                }
            }
        }`,
	}}

	env := fakeEnv{
		"RODERIK_AI_MODEL_PROFILE": "alpha",
		"OPENAI_API_KEY":           "env-key",
	}

	loader := Loader{
		ConfigPath: configPath,
		Getenv:     env.Get,
		ReadFile:   fs.ReadFile,
	}

	prof, err := loader.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if prof.APIKey != "inline-key" {
		t.Fatalf("expected inline api key to win, got %q", prof.APIKey)
	}
}
