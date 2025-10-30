package cmd

import (
	"path/filepath"
	"strings"

	"roderik/internal/appdirs"
)

func ensureUserDataRoot() (string, error) {
	root, err := appdirs.UserDataDir()
	if err != nil {
		return "", err
	}
	if err := appdirs.EnsureDir(root); err != nil {
		return "", err
	}
	return root, nil
}

func defaultDownloadsDir() string {
	root, err := ensureUserDataRoot()
	if err == nil && strings.TrimSpace(root) != "" {
		return filepath.Join(root, "downloads")
	}

	dir, dirErr := appdirs.DownloadsDir()
	if dirErr == nil && strings.TrimSpace(dir) != "" {
		return dir
	}

	if abs, absErr := filepath.Abs(filepath.Join("user_data", "downloads")); absErr == nil {
		return abs
	}
	return filepath.Join("user_data", "downloads")
}
