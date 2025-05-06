package client

import (
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)

func TestSearchLimited_RetriesOn202(t *testing.T) {
    wantAttempts := 3
    calls := 0

    htmlBody := `<div class="result__url">` +
        `http://example.com` +
        `</div><div class="result__a">Example</div>` +
        `<div class="result__snippet">Snippet</div>`

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        calls++
        if calls < wantAttempts {
            w.WriteHeader(202)
            return
        }
        w.WriteHeader(200)
        fmt.Fprintln(w, `<html><body>`+htmlBody+`</body></html>`)
    }))
    defer srv.Close()

    client := NewDuckDuckGoSearchClient()
    client.baseUrl = srv.URL
    client.MaxRetries = wantAttempts
    client.Backoff = 0 * time.Millisecond

    results, err := client.SearchLimited("anything", 1)
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }
    if len(results) != 1 {
        t.Fatalf("expected 1 result, got %d", len(results))
    }
    if calls != wantAttempts {
        t.Errorf("expected %d attempts, got %d", wantAttempts, calls)
    }

    r := results[0]
    if !strings.Contains(r.FormattedUrl, "example.com") {
        t.Errorf("unexpected URL: %q", r.FormattedUrl)
    }
    if r.Title != "Example" {
        t.Errorf("unexpected Title: %q", r.Title)
    }
    if r.Snippet != "Snippet" {
        t.Errorf("unexpected Snippet: %q", r.Snippet)
    }
}
