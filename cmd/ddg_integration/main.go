package main

import (
	"log"
	"math/rand"
	"net/http"
	"time"

	client "roderik/duckduck"
)

func init() {
	// wrap the default transport so we can print every status code
	http.DefaultTransport = &loggingRoundTripper{rt: http.DefaultTransport}
}

type loggingRoundTripper struct{ rt http.RoundTripper }

func (l *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := l.rt.RoundTrip(req)
	if err == nil {
		log.Printf("[HTTP] %s -> %d", req.URL, resp.StatusCode)
	}
	return resp, err
}

func main() {
	// a few sample keywords; you can modify or extend this list
	keywords := []string{
		"golang", "cobra", "privacy", "security",
	}

	ddg := client.NewDuckDuckGoSearchClient()
	ddg.MaxRetries = 5
	ddg.Backoff = 500 * time.Millisecond

	for i, kw := range keywords {
		// to really vary your queries you can append a random suffix
		if i%2 == 0 {
			kw = kw + "-" + randSeq(4)
		}
		log.Printf("=== Searching for %q ===", kw)
		start := time.Now()
		res, err := ddg.SearchLimited(kw, 1)
		dur := time.Since(start)
		if err != nil {
			log.Printf(" ❌  error: %v (took %v)", err, dur)
		} else {
			log.Printf(" ✅  got %d result(s) in %v. first URL=%q", len(res), dur,
				func() string {
					if len(res) > 0 {
						return res[0].FormattedUrl
					}
					return ""
				}())
		}
		// tiny pause so you don’t overwhelm the server
		time.Sleep(200 * time.Millisecond)
	}
}

// randSeq returns a random alphanumeric string of length n.
func randSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
