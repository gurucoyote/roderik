package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ModelProfile describes the connection settings required to invoke an LLM provider.
type ModelProfile struct {
	Name         string `json:"-"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key"`
	APIKeyEnv    string `json:"api_key_env"`
	MaxTokens    int    `json:"max_tokens"`
	SystemPrompt string `json:"system_prompt"`
}

// Config captures all available model profiles and their defaults.
type Config struct {
	Default  string                   `json:"default"`
	Profiles map[string]*ModelProfile `json:"profiles"`
}

// Loader resolves model profiles using an on-disk config file and environment overrides.
type Loader struct {
	// ConfigPath allows tests to point at an alternate config file. If empty the
	// loader falls back to DefaultConfigPath().
	ConfigPath string

	// Getenv is used to pull environment variables. Defaults to os.Getenv.
	Getenv func(string) string

	// ReadFile is used to read the config file. Defaults to os.ReadFile.
	ReadFile func(string) ([]byte, error)
}

// Default values used when neither config nor environment specify an option.
const (
	defaultProvider = "openai"
	defaultModel    = "gpt-5"
)

// DefaultConfigPath returns the standard location for AI model profiles.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".roderik", "ai-profiles.json")
}

// Load resolves the requested model profile. The selection precedence is:
//  1. Explicit selection (CLI flag argument).
//  2. Environment variable RODERIK_AI_MODEL_PROFILE.
//  3. Config file default field.
//  4. Empty profile using environment variables only.
func (l *Loader) Load(selection string) (ModelProfile, error) {
	getenv := l.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}

	readFile := l.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}

	configPath := l.ConfigPath
	if strings.TrimSpace(configPath) == "" {
		configPath = DefaultConfigPath()
	}

	cfg, err := readConfig(configPath, readFile)
	if err != nil {
		return ModelProfile{}, err
	}

	if strings.TrimSpace(selection) == "" {
		if envSel := strings.TrimSpace(getenv("RODERIK_AI_MODEL_PROFILE")); envSel != "" {
			selection = envSel
		}
	}

	var (
		profile ModelProfile
		found   bool
	)

	if selection != "" && cfg != nil {
		profile, found = cfg.profile(selection)
		if !found {
			return ModelProfile{}, fmt.Errorf("model profile %q not found in %s", selection, configPath)
		}
	}

	if !found && cfg != nil && cfg.Default != "" {
		profile, found = cfg.profile(cfg.Default)
		selection = cfg.Default
		if !found {
			return ModelProfile{}, fmt.Errorf("default model profile %q not found in %s", cfg.Default, configPath)
		}
	}

	if found {
		profile.Name = selection
	}

	// Apply environment overrides and defaults.
	applyEnvOverrides(&profile, getenv)
	applyDefaults(&profile)

	// If the profile originated purely from config/env, ensure we still propagate the name.
	if profile.Name == "" {
		profile.Name = selection
	}

	return profile, nil
}

func readConfig(path string, readFile func(string) ([]byte, error)) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	data, err := readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read model profile config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode model profile config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) profile(name string) (ModelProfile, bool) {
	if c == nil || c.Profiles == nil {
		return ModelProfile{}, false
	}

	prof, ok := c.Profiles[name]
	if !ok || prof == nil {
		return ModelProfile{}, false
	}

	clone := *prof
	clone.Name = name
	return clone, true
}

func applyEnvOverrides(profile *ModelProfile, getenv func(string) string) {
	if profile == nil {
		return
	}

	// API key precedence: explicit config value -> env referenced via api_key_env -> OPENAI_API_KEY
	if strings.TrimSpace(profile.APIKey) == "" {
		if envName := strings.TrimSpace(profile.APIKeyEnv); envName != "" {
			if val := strings.TrimSpace(getenv(envName)); val != "" {
				profile.APIKey = val
			}
		}

		if strings.TrimSpace(profile.APIKey) == "" {
			if val := strings.TrimSpace(getenv("OPENAI_API_KEY")); val != "" {
				profile.APIKey = val
			}
		}
	}

	// Base URL precedence: config -> OPENAI_API_BASE -> OPENAI_BASE_URL
	if val := strings.TrimSpace(getenv("OPENAI_API_BASE")); val != "" {
		profile.BaseURL = val
	} else if val := strings.TrimSpace(getenv("OPENAI_BASE_URL")); val != "" {
		profile.BaseURL = val
	}

	// Model precedence: config -> RODERIK_AI_MODEL
	if val := strings.TrimSpace(getenv("RODERIK_AI_MODEL")); val != "" {
		profile.Model = val
	}

	// Provider precedence: config -> RODERIK_AI_PROVIDER
	if val := strings.TrimSpace(getenv("RODERIK_AI_PROVIDER")); val != "" {
		profile.Provider = val
	}

	// Max tokens precedence: config -> RODERIK_AI_MAX_TOKENS
	if raw := strings.TrimSpace(getenv("RODERIK_AI_MAX_TOKENS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			profile.MaxTokens = parsed
		}
	}
}

func applyDefaults(profile *ModelProfile) {
	if profile == nil {
		return
	}

	if strings.TrimSpace(profile.Provider) == "" {
		profile.Provider = defaultProvider
	}

	if strings.TrimSpace(profile.Model) == "" {
		profile.Model = defaultModel
	}
}
