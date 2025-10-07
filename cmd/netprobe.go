package cmd

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// probeURL downloads up to maxBytes from the given URL and returns:
//   - the partial body that was read
//   - the Content‐Type header
//   - whether the content looks like HTML
//   - any error encountered
//
// The heuristic for “looks like HTML” is:
//  1. Content‐Type header contains the word “html”, OR
//  2. The first chunk of the body contains the string “<html”.
func probeURL(u string, maxBytes int) ([]byte, string, bool, error) {
	resp, err := http.Get(u)
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if maxBytes <= 0 {
		maxBytes = 32768
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)))
	if err != nil {
		return nil, ct, false, err
	}

	looksHTML := strings.Contains(strings.ToLower(ct), "html") ||
		bytes.Contains(bytes.ToLower(body), []byte("<html"))

	return body, ct, looksHTML, nil
}

// decodeToUTF8 best-effort decodes an HTTP response body into UTF-8 so that
// MCP tool responses always marshal cleanly even when the upstream server uses
// a legacy charset.
func decodeToUTF8(data []byte, contentType string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	if utf8.Valid(data) {
		return string(data), nil
	}

	enc, _, _ := charset.DetermineEncoding(data, contentType)
	reader := transform.NewReader(bytes.NewReader(data), enc.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	if utf8.Valid(decoded) {
		return string(decoded), nil
	}

	// As a last resort replace invalid runes with the Unicode replacement
	// character to keep the payload JSON-safe.
	safe := bytes.ToValidUTF8(decoded, []byte("�"))
	return string(safe), nil
}
