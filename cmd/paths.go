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
	dir, err := appdirs.DownloadsDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		return filepath.Join("user_data", "downloads")
	}
	return dir
}
