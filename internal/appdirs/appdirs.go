package appdirs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	envHomeOverride      = "RODERIK_HOME"
	envUserDataOverride  = "RODERIK_USER_DATA_DIR"
	envLogsOverride      = "RODERIK_LOG_DIR"
	envDownloadsOverride = "RODERIK_DOWNLOAD_DIR"
)

func BaseDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(envHomeOverride)); dir != "" {
		return filepath.Clean(dir), nil
	}

	if cfgDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(cfgDir) != "" {
		return filepath.Join(cfgDir, "roderik"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		if err == nil {
			err = errors.New("empty home directory")
		}
		return "", fmt.Errorf("determine roderik base dir: %w", err)
	}

	return filepath.Join(home, ".roderik"), nil
}

func UserDataDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(envUserDataOverride)); dir != "" {
		return filepath.Clean(dir), nil
	}

	base, err := BaseDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "user_data"), nil
}

func LogsDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(envLogsOverride)); dir != "" {
		return filepath.Clean(dir), nil
	}

	base, err := BaseDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "logs"), nil
}

func DownloadsDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(envDownloadsOverride)); dir != "" {
		return filepath.Clean(dir), nil
	}

	userData, err := UserDataDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(userData, "downloads"), nil
}

func EnsureDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("ensure dir: empty path")
	}
	return os.MkdirAll(path, 0o755)
}
