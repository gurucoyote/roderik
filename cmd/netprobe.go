package cmd

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// probeURL downloads up to maxBytes from the given URL and returns:
//   • the partial body that was read
//   • the Content‐Type header
//   • whether the content looks like HTML
//   • any error encountered
//
// The heuristic for “looks like HTML” is:
//   1. Content‐Type header contains the word “html”, OR
//   2. The first chunk of the body contains the string “<html”.
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
