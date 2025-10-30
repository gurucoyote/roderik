package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/go-rod/rod/lib/proto"
	"github.com/spf13/cobra"
)

var (
	netlogMIMEs                   []string
	netlogSuffixes                []string
	netlogStatuses                []int
	netlogContains                []string
	netlogMethods                 []string
	netlogDomains                 []string
	netlogTypes                   []string
	netlogRequestIDs              []string
	netlogSave                    bool
	netlogSaveAll                 bool
	netlogInteractive             bool
	netlogOutputDir               string
	netlogFilenamePrefix          string
	netlogFilenameSuffix          string
	netlogFilenameUseTimestamp    bool
	netlogFilenameTimestampFormat string
)

var netlogEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable network activity logging for the current session",
	RunE: func(cmd *cobra.Command, args []string) error {
		changed := setNetworkActivityEnabled(true)
		if changed {
			fmt.Fprintln(os.Stdout, "network activity logging enabled")
		} else {
			fmt.Fprintln(os.Stdout, "network activity logging already enabled")
		}
		return nil
	},
}

var netlogDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable network activity logging for the current session",
	RunE: func(cmd *cobra.Command, args []string) error {
		changed := setNetworkActivityEnabled(false)
		if changed {
			fmt.Fprintln(os.Stdout, "network activity logging disabled")
		} else {
			fmt.Fprintln(os.Stdout, "network activity logging already disabled")
		}
		return nil
	},
}

var netlogStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether network activity logging is currently enabled",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(os.Stdout, "network activity logging enabled: %t\n", isNetworkActivityEnabled())
		return nil
	},
}

var netlogCmd = &cobra.Command{
	Use:   "netlog",
	Short: "Inspect and save captured network activity",
	RunE: func(cmd *cobra.Command, args []string) error {
		log := getActiveEventLog()
		if log == nil {
			return fmt.Errorf("no active network log; load a page first")
		}

		filter := NetworkLogFilter{
			MIMESubstrings: normalizeStrings(netlogMIMEs),
			Suffixes:       normalizeStrings(netlogSuffixes),
			StatusCodes:    netlogStatuses,
			TextContains:   normalizeStrings(netlogContains),
			Methods:        normalizeStrings(netlogMethods),
			Domains:        normalizeStrings(netlogDomains),
		}

		if len(netlogTypes) > 0 {
			rts, err := parseResourceTypes(netlogTypes)
			if err != nil {
				return err
			}
			filter.ResourceTypes = rts
		}

		entries := log.FilterEntries(filter)
		if len(entries) == 0 {
			fmt.Fprintln(os.Stderr, "no network entries matched the specified filters")
			return nil
		}

		sort.Slice(entries, func(i, j int) bool {
			ti := entries[i].RequestTimestamp
			tj := entries[j].RequestTimestamp
			if ti.IsZero() || tj.IsZero() {
				return entries[i].RequestID < entries[j].RequestID
			}
			return ti.Before(tj)
		})

		printNetlogEntries(entries)

		if !netlogSave {
			return nil
		}

		selected, err := selectEntriesForSave(entries)
		if err != nil {
			return err
		}
		if len(selected) == 0 {
			fmt.Fprintln(os.Stderr, "no entries selected for saving")
			return nil
		}

		results, err := saveNetworkEntriesToDisk(selected, netlogOutputDir)
		if err != nil {
			return err
		}
		for _, res := range results {
			if res.Err != nil {
				fmt.Fprintf(os.Stderr, "failed to save %s: %v\n", res.Entry.URL, res.Err)
				continue
			}
			fmt.Fprintf(os.Stdout, "saved %d bytes to %s\n", res.Bytes, res.Path)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(netlogCmd)
	netlogCmd.AddCommand(netlogEnableCmd)
	netlogCmd.AddCommand(netlogDisableCmd)
	netlogCmd.AddCommand(netlogStatusCmd)

	netlogCmd.Flags().StringSliceVar(&netlogMIMEs, "mime", nil, "Filter by MIME substring (repeatable)")
	netlogCmd.Flags().StringSliceVar(&netlogSuffixes, "suffix", nil, "Filter by URL suffix (e.g. .mp3)")
	netlogCmd.Flags().IntSliceVar(&netlogStatuses, "status", nil, "Filter by HTTP status code")
	netlogCmd.Flags().StringSliceVar(&netlogContains, "contains", nil, "Filter by substring match in URL")
	netlogCmd.Flags().StringSliceVar(&netlogMethods, "method", nil, "Filter by HTTP method")
	netlogCmd.Flags().StringSliceVar(&netlogDomains, "domain", nil, "Filter by domain substring")
	netlogCmd.Flags().StringSliceVar(&netlogTypes, "type", nil, "Filter by resource type (Document, Image, Media, Script, XHR, etc.)")
	netlogCmd.Flags().StringSliceVar(&netlogRequestIDs, "request-id", nil, "Explicit request IDs to operate on")
	netlogCmd.Flags().BoolVar(&netlogSave, "save", false, "Save selected entries to disk")
	netlogCmd.Flags().BoolVar(&netlogSaveAll, "all", false, "When saving, include all filtered entries without prompting")
	netlogCmd.Flags().BoolVar(&netlogInteractive, "interactive", StdoutIsTerminal(), "When saving, prompt for entries to persist")
	netlogCmd.Flags().StringVar(&netlogOutputDir, "output", defaultDownloadsDir(), "Directory to write captured resources")
	netlogCmd.Flags().StringVar(&netlogFilenamePrefix, "filename-prefix", "", "Optional filename prefix when saving")
	netlogCmd.Flags().StringVar(&netlogFilenameSuffix, "filename-suffix", "", "Optional filename suffix when saving")
	netlogCmd.Flags().BoolVar(&netlogFilenameUseTimestamp, "filename-timestamp", false, "Include a timestamp in saved filenames")
	netlogCmd.Flags().StringVar(&netlogFilenameTimestampFormat, "filename-timestamp-format", "2006-01-02_150405", "Go time format for timestamps when --filename-timestamp is set")
}

func printNetlogEntries(entries []*NetworkLogEntry) {
	tw := tabwriter.NewWriter(os.Stdout, 4, 2, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintln(tw, "INDEX\tSTATUS\tMETHOD\tTYPE\tMIME\tURL")
	for idx, entry := range entries {
		status := "-"
		mime := ""
		if entry.Response != nil {
			status = fmt.Sprintf("%d", entry.Response.Status)
			mime = entry.Response.MIMEType
		}
		typ := string(entry.ResourceType)
		if typ == "" {
			typ = "(unknown)"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n", idx, status, entry.Method, typ, mime, entry.URL)
	}
}

func selectEntriesForSave(entries []*NetworkLogEntry) ([]*NetworkLogEntry, error) {
	if len(netlogRequestIDs) > 0 {
		selected := make([]*NetworkLogEntry, 0, len(netlogRequestIDs))
		missing := make([]string, 0)
		idSet := make(map[string]struct{}, len(netlogRequestIDs))
		for _, id := range netlogRequestIDs {
			idSet[strings.TrimSpace(id)] = struct{}{}
		}
		for _, entry := range entries {
			if _, ok := idSet[entry.RequestID]; ok {
				selected = append(selected, entry)
				delete(idSet, entry.RequestID)
			}
		}
		for missingID := range idSet {
			missing = append(missing, missingID)
		}
		if len(missing) > 0 {
			return selected, fmt.Errorf("request ids not found in filtered set: %s", strings.Join(missing, ", "))
		}
		return selected, nil
	}

	if netlogSaveAll || !netlogInteractive {
		return entries, nil
	}

	options := make([]string, len(entries))
	for i, entry := range entries {
		status := "-"
		mime := ""
		if entry.Response != nil {
			status = fmt.Sprintf("%d", entry.Response.Status)
			mime = entry.Response.MIMEType
		}
		options[i] = fmt.Sprintf("[%d] %s (%s, %s) %s", i, entry.URL, entry.Method, status, mime)
	}
	selectedIdx := []int{}
	prompt := &survey.MultiSelect{
		Message: "Select network entries to save",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selectedIdx, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}
	selected := make([]*NetworkLogEntry, 0, len(selectedIdx))
	for _, idx := range selectedIdx {
		if idx >= 0 && idx < len(entries) {
			selected = append(selected, entries[idx])
		}
	}
	return selected, nil
}

type networkSaveResult struct {
	Entry *NetworkLogEntry
	Path  string
	Bytes int
	Err   error
}

func saveNetworkEntriesToDisk(entries []*NetworkLogEntry, dir string) ([]networkSaveResult, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	var results []networkSaveResult
	res, err := withPage(func() ([]networkSaveResult, error) {
		if Page == nil {
			return nil, fmt.Errorf("no page loaded – cannot retrieve response bodies")
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create output directory: %w", err)
		}
		used := make(map[string]int)
		out := make([]networkSaveResult, 0, len(entries))
		nameOpts := FileNamingOptions{
			Prefix:           netlogFilenamePrefix,
			Suffix:           netlogFilenameSuffix,
			IncludeTimestamp: netlogFilenameUseTimestamp,
			TimestampFormat:  netlogFilenameTimestampFormat,
		}
		for idx, entry := range entries {
			data, err := retrieveNetworkBody(Page, entry)
			if err != nil {
				out = append(out, networkSaveResult{Entry: entry, Err: err})
				continue
			}
			name := ensureUniqueFilename(dir, buildFilenameForEntry(entry, idx, nameOpts), used)
			fullPath := filepath.Join(dir, name)
			if writeErr := os.WriteFile(fullPath, data, 0o644); writeErr != nil {
				out = append(out, networkSaveResult{Entry: entry, Err: writeErr})
				continue
			}
			out = append(out, networkSaveResult{Entry: entry, Path: fullPath, Bytes: len(data)})
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	results = append(results, res...)
	return results, nil
}

func suggestFilename(entry *NetworkLogEntry, index int) string {
	if entry == nil {
		return fmt.Sprintf("resource_%d", index)
	}
	parsed, err := url.Parse(entry.URL)
	var base string
	if err == nil {
		base = path.Base(parsed.Path)
	}
	base = strings.TrimSpace(base)
	base = strings.TrimSuffix(base, "/")
	base = strings.Split(base, "?")[0]
	base = strings.Split(base, "#")[0]
	if base == "" || base == "." || base == "/" {
		safeID := strings.ReplaceAll(entry.RequestID, ".", "_")
		base = fmt.Sprintf("%s_%s", strings.ToLower(entry.Method), safeID)
	}
	base = sanitizeFilename(base)
	if ext := filepath.Ext(base); ext == "" {
		if entry.Response != nil {
			if mapped := extensionForMIME(entry.Response.MIMEType); mapped != "" {
				base = base + "." + mapped
			}
		}
	}
	if base == "" {
		base = fmt.Sprintf("resource_%d", index)
	}
	return base
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
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
	clean := replacer.Replace(name)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "resource"
	}
	return clean
}

func ensureUniqueFilename(dir, base string, used map[string]int) string {
	baseName := strings.TrimSuffix(base, filepath.Ext(base))
	ext := filepath.Ext(base)
	if baseName == "" {
		baseName = "resource"
	}
	counter := used[base]
	for {
		name := base
		if counter > 0 {
			name = fmt.Sprintf("%s_%d%s", baseName, counter, ext)
		}
		full := filepath.Join(dir, name)
		if _, err := os.Stat(full); errors.Is(err, os.ErrNotExist) {
			used[base] = counter + 1
			return name
		}
		counter++
	}
}

func extensionForMIME(mime string) string {
	if mime == "" {
		return ""
	}
	lower := strings.ToLower(strings.TrimSpace(mime))
	switch {
	case strings.Contains(lower, "json"):
		return "json"
	case strings.Contains(lower, "html"):
		return "html"
	case strings.Contains(lower, "javascript"):
		return "js"
	case strings.Contains(lower, "css"):
		return "css"
	case strings.Contains(lower, "png"):
		return "png"
	case strings.Contains(lower, "jpeg") || strings.Contains(lower, "jpg"):
		return "jpg"
	case strings.Contains(lower, "gif"):
		return "gif"
	case strings.Contains(lower, "svg"):
		return "svg"
	case strings.Contains(lower, "pdf"):
		return "pdf"
	case strings.Contains(lower, "mp3"):
		return "mp3"
	case strings.Contains(lower, "mpeg") && strings.Contains(lower, "audio"):
		return "mp3"
	case strings.Contains(lower, "wav"):
		return "wav"
	case strings.Contains(lower, "mp4"):
		return "mp4"
	case strings.Contains(lower, "ogg"):
		return "ogg"
	case strings.Contains(lower, "zip"):
		return "zip"
	case strings.Contains(lower, "plain") || strings.Contains(lower, "text"):
		return "txt"
	default:
		return ""
	}
}

func parseResourceTypes(values []string) ([]proto.NetworkResourceType, error) {
	if len(values) == 0 {
		return nil, nil
	}
	var out []proto.NetworkResourceType
	for _, raw := range values {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if rt, ok := normalizeResourceType(s); ok {
			out = append(out, rt)
			continue
		}
		return nil, fmt.Errorf("unknown resource type: %s", s)
	}
	return out, nil
}

func normalizeResourceType(raw string) (proto.NetworkResourceType, bool) {
	lookup := map[string]proto.NetworkResourceType{
		"document":           proto.NetworkResourceTypeDocument,
		"stylesheet":         proto.NetworkResourceTypeStylesheet,
		"style":              proto.NetworkResourceTypeStylesheet,
		"image":              proto.NetworkResourceTypeImage,
		"media":              proto.NetworkResourceTypeMedia,
		"font":               proto.NetworkResourceTypeFont,
		"script":             proto.NetworkResourceTypeScript,
		"texttrack":          proto.NetworkResourceTypeTextTrack,
		"xhr":                proto.NetworkResourceTypeXHR,
		"fetch":              proto.NetworkResourceTypeFetch,
		"prefetch":           proto.NetworkResourceTypePrefetch,
		"eventsource":        proto.NetworkResourceTypeEventSource,
		"websocket":          proto.NetworkResourceTypeWebSocket,
		"manifest":           proto.NetworkResourceTypeManifest,
		"signedexchange":     proto.NetworkResourceTypeSignedExchange,
		"ping":               proto.NetworkResourceTypePing,
		"cspviolationreport": proto.NetworkResourceTypeCSPViolationReport,
		"preflight":          proto.NetworkResourceTypePreflight,
		"other":              proto.NetworkResourceTypeOther,
	}
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, " ", "")
	if rt, ok := lookup[s]; ok {
		return rt, true
	}
	return "", false
}
