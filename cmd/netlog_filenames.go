package cmd

import (
	"path/filepath"
	"strings"
	"time"
)

// FileNamingOptions controls how saved network resources are named on disk.
type FileNamingOptions struct {
	ExplicitName     string
	Prefix           string
	Suffix           string
	IncludeTimestamp bool
	TimestampFormat  string
}

var filenameSanitizer = strings.NewReplacer(
	"<", "_",
	">", "_",
	":", "_",
	"\"", "_",
	"/", "_",
	"\\", "_",
	"|", "_",
	"?", "_",
	"*", "_",
)

// sanitizeComponent removes filesystem-hostile characters but allows empty results.
func sanitizeComponent(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.TrimSpace(filenameSanitizer.Replace(trimmed))
}

func buildFilenameForEntry(entry *NetworkLogEntry, index int, opts FileNamingOptions) string {
	base := suggestFilename(entry, index)
	return applyFileNamingOptions(entry, base, opts)
}

func applyFileNamingOptions(entry *NetworkLogEntry, base string, opts FileNamingOptions) string {
	name := base
	if strings.TrimSpace(opts.ExplicitName) != "" {
		name = sanitizeFilename(opts.ExplicitName)
	}

	ext := filepath.Ext(name)
	if ext == "" {
		if entry != nil && entry.Response != nil {
			if mapped := extensionForMIME(entry.Response.MIMEType); mapped != "" {
				ext = "." + mapped
			}
		}
	}

	trimmed := strings.TrimSuffix(name, ext)
	components := make([]string, 0, 4)

	if prefix := sanitizeComponent(opts.Prefix); prefix != "" {
		components = append(components, prefix)
	}

	if opts.IncludeTimestamp {
		format := strings.TrimSpace(opts.TimestampFormat)
		if format == "" {
			format = "2006-01-02_150405"
		}
		ts := sanitizeComponent(resolveEntryTimestamp(entry).Format(format))
		if ts != "" {
			components = append(components, ts)
		}
	}

	if trimmed != "" {
		components = append(components, sanitizeComponent(trimmed))
	}

	if suffix := sanitizeComponent(opts.Suffix); suffix != "" {
		components = append(components, suffix)
	}

	if len(components) == 0 {
		components = append(components, "resource")
	}

	finalName := strings.Join(components, "_")
	return finalName + ext
}

func resolveEntryTimestamp(entry *NetworkLogEntry) time.Time {
	if entry == nil {
		return time.Now()
	}
	if entry.Response != nil && !entry.Response.ResponseTimestamp.IsZero() {
		return entry.Response.ResponseTimestamp
	}
	if entry.Finished != nil && !entry.Finished.FinishedTimestamp.IsZero() {
		return entry.Finished.FinishedTimestamp
	}
	if !entry.RequestTimestamp.IsZero() {
		return entry.RequestTimestamp
	}
	return time.Now()
}
