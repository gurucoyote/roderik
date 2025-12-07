package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var (
	ytLanguage     string
	ytOutputFolder string

	// YTTransCmd downloads YouTube transcripts via yt-dlp and prints cleaned text.
	YTTransCmd = &cobra.Command{
		Use:   "yttrans [flags] <youtube url>",
		Short: "Download and convert a YouTube transcript to plain text",
		Args:  cobra.ExactArgs(1),
		RunE:  runYTTrans,
		Long: `yttrans uses yt-dlp to fetch the auto-generated or uploaded transcript for a
YouTube video, converts it to plain text, and stores both the original VTT and
cleaned TXT inside a local cache directory. The resulting text is also written to
stdout so other commands can capture it.`,
	}
)

func init() {
	RootCmd.AddCommand(YTTransCmd)
	flags := YTTransCmd.Flags()
	flags.StringVarP(&ytLanguage, "language", "l", "en", "language code to request (e.g. en, de)")
	flags.StringVarP(&ytOutputFolder, "output-folder", "o", "./yttrans-cache", "folder for cached transcripts")
}

// TranscriptOptions controls how a transcript download is performed.
type TranscriptOptions struct {
	Language     string
	URL          string
	OutputFolder string
}

// TranscriptResult provides metadata about downloaded transcripts.
type TranscriptResult struct {
	VideoID  string
	Language string
	VTTPath  string
	TextPath string
	Text     string
}

func runYTTrans(cmd *cobra.Command, args []string) error {
	opts := TranscriptOptions{Language: ytLanguage, URL: args[0], OutputFolder: ytOutputFolder}
	res, err := DownloadAndProcessTranscript(opts)
	if err != nil {
		return err
	}
	cmd.Printf("Transcript saved to %s (language=%s)\n\n%s\n", res.TextPath, res.Language, res.Text)
	return nil
}

// DownloadAndProcessTranscript downloads a YouTube transcript via yt-dlp, converts it to text, and saves it alongside the VTT file.
func DownloadAndProcessTranscript(opts TranscriptOptions) (TranscriptResult, error) {
	result := TranscriptResult{Language: opts.Language}
	if strings.TrimSpace(opts.URL) == "" {
		return result, errors.New("yttrans: URL is required")
	}
	ytBinary, err := exec.LookPath("yt-dlp")
	if err != nil {
		return result, fmt.Errorf("yttrans: yt-dlp not found in PATH: %w", err)
	}
	videoID := extractVideoID(opts.URL)
	if videoID == "" {
		return result, fmt.Errorf("yttrans: could not extract video id from %q", opts.URL)
	}
	result.VideoID = videoID
	if err := os.MkdirAll(opts.OutputFolder, 0o755); err != nil {
		return result, fmt.Errorf("yttrans: create output dir: %w", err)
	}

	// Use yt-dlp to download subtitles without media; template ensures unique names per video title+id.
	// We convert subtitles to VTT so downstream parsing remains stable.
	cmd := exec.Command(
		ytBinary,
		"--write-auto-subs",
		"--skip-download",
		"--convert-subs", "vtt",
		"--write-info-json",
		"--sub-langs", opts.Language+",-live_chat",
		"-o", filepath.Join(opts.OutputFolder, "%(title)s-%(id)s.%(ext)s"),
		opts.URL,
	)
	if err := cmd.Run(); err != nil {
		return result, fmt.Errorf("yttrans: yt-dlp failed: %w", err)
	}

	vttPattern := filepath.Join(opts.OutputFolder, fmt.Sprintf("*%s*.vtt", videoID))
	matches, err := filepath.Glob(vttPattern)
	if err != nil {
		return result, fmt.Errorf("yttrans: glob vtt: %w", err)
	}
	if len(matches) == 0 {
		return result, fmt.Errorf("yttrans: no VTT transcript found for %s", videoID)
	}
	result.VTTPath = matches[0]
	raw, err := os.ReadFile(result.VTTPath)
	if err != nil {
		return result, fmt.Errorf("yttrans: read %s: %w", result.VTTPath, err)
	}

	text := vttToText(string(raw))
	result.TextPath = strings.TrimSuffix(result.VTTPath, filepath.Ext(result.VTTPath)) + ".txt"
	if err := os.WriteFile(result.TextPath, []byte(text), 0o644); err != nil {
		return result, fmt.Errorf("yttrans: write %s: %w", result.TextPath, err)
	}
	result.Text = text
	return result, nil
}

var (
	timestampRe    = regexp.MustCompile(`(?m)^(\d{2}:\d{2})(:\d{2}\.\d{3})?\s+-->.*$`)
	markupRe       = regexp.MustCompile(`</c>|<c(\.color\w+)?>|<\d{2}:\d{2}:\d{2}\.\d{3}>`)
	multiSpaceRe   = regexp.MustCompile(` +`)
	shortLineLimit = 80
)

func vttToText(vtt string) string {
	cleaned := markupRe.ReplaceAllString(vtt, "")
	cleaned = timestampRe.ReplaceAllString(cleaned, "$1")
	lines := strings.Split(cleaned, "\n")

	var out []string
	var lastTimestamp, buffer string

	flush := func() {
		if strings.TrimSpace(buffer) == "" {
			buffer = ""
			return
		}
		out = append(out, multiSpaceRe.ReplaceAllString(strings.TrimSpace(buffer), " "))
		buffer = ""
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) == 5 && line[2] == ':' { // timestamp HH:MM
			if line != lastTimestamp {
				flush()
				out = append(out, "\n"+line)
				lastTimestamp = line
			}
			continue
		}
		if len(buffer)+len(line)+1 <= shortLineLimit {
			if buffer == "" {
				buffer = line
			} else {
				buffer += " " + line
			}
		} else {
			flush()
			buffer = line
		}
	}
	flush()

	return strings.Join(out, "\n")
}

var videoIDPattern = regexp.MustCompile(`(?:v=|/)([0-9A-Za-z_-]{11})`)

func extractVideoID(url string) string {
	matches := videoIDPattern.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
