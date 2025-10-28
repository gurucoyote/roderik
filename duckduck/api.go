package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type SearchClient interface {
	Search(query string) ([]Result, error)
	SearchLimited(query string, limit int) ([]Result, error)
}

type DuckDuckGoSearchClient struct {
	baseUrl      string
	MaxRetries   int
	InitialDelay time.Duration
	Backoff      time.Duration
	client       *http.Client
	UserAgent    string
}

var ErrBotChallenge = errors.New("duckduckgo bot challenge detected")

type ChallengeError struct {
	message string
}

func (e *ChallengeError) Error() string {
	return e.message
}

func (e *ChallengeError) Unwrap() error {
	return ErrBotChallenge
}

func NewChallengeError() error {
	return &ChallengeError{message: "DuckDuckGo returned a bot challenge; results are unavailable until the challenge is completed manually."}
}

func IsChallengeError(err error) bool {
	return errors.Is(err, ErrBotChallenge)
}

func NewDuckDuckGoSearchClient() *DuckDuckGoSearchClient {
	jar, _ := cookiejar.New(nil)
	httpClient := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Return ErrUseLastResponse so we see 3xx responses instead of following redirects
			return http.ErrUseLastResponse
		},
	}
	return &DuckDuckGoSearchClient{
		// use DuckDuckGo's HTML‐only subdomain
		baseUrl:      "https://html.duckduckgo.com/html/",
		MaxRetries:   3,
		InitialDelay: 5 * time.Second,
		Backoff:      4 * time.Second,
		client:       httpClient,
		// a realistic Chrome user‐agent to reduce rate‐limiting
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
			"AppleWebKit/537.36 (KHTML, like Gecko) " +
			"Chrome/114.0.0.0 Safari/537.36",
	}
}
func (c *DuckDuckGoSearchClient) Search(query string) ([]Result, error) {
	return c.SearchLimited(query, 0)
}

func (c *DuckDuckGoSearchClient) SearchLimited(query string, limit int) ([]Result, error) {
	queryUrl := c.baseUrl + "?q=" + url.QueryEscape(query)

	if c.InitialDelay > 0 {
		time.Sleep(c.InitialDelay)
	}

	var resp *http.Response
	var err error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		req, _ := http.NewRequest("GET", queryUrl, nil)
		req.Header.Set("User-Agent", c.UserAgent)
		resp, err = c.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusOK {
			break
		}
		resp.Body.Close()
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < 300 {
			if attempt == c.MaxRetries {
				return []Result{}, nil
			}
			time.Sleep(c.Backoff * (1 << attempt))
			continue
		}
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	if isChallengePage(doc) {
		return nil, NewChallengeError()
	}
	results := make([]Result, 0)
	doc.Find(".results .web-result").Each(func(i int, s *goquery.Selection) {
		if i > limit-1 && limit > 0 {
			return
		}
		results = append(results, c.collectResult(s))
	})
	if limit > 0 && len(results) == 0 {
		// fallback for simple HTML structures without wrapper elements
		results = append(results, c.collectResult(doc.Selection))
	}
	return results, nil
}

func (c *DuckDuckGoSearchClient) collectResult(s *goquery.Selection) Result {
	resUrlHtml := html(s.Find(".result__url").Html())
	resUrl := clean(s.Find(".result__url").Text())
	titleHtml := html(s.Find(".result__a").Html())
	title := clean(s.Find(".result__a").Text())
	snippetHtml := html(s.Find(".result__snippet").Html())
	snippet := clean(s.Find(".result__snippet").Text())
	icon := s.Find(".result__icon__img")
	src, _ := icon.Attr("src")
	width, _ := icon.Attr("width")
	height, _ := icon.Attr("height")
	return Result{
		HtmlFormattedUrl: resUrlHtml,
		HtmlTitle:        titleHtml,
		HtmlSnippet:      snippetHtml,
		FormattedUrl:     resUrl,
		Title:            title,
		Snippet:          snippet,
		Icon: Icon{
			Src:    src,
			Width:  toInt(width),
			Height: toInt(height),
		},
	}
}

func html(html string, err error) string {
	if err != nil {
		return ""
	}
	return clean(html)
}

func clean(text string) string {
	return strings.TrimSpace(strings.ReplaceAll(text, "\n", ""))
}

func toInt(n string) int {
	res, err := strconv.Atoi(n)
	if err != nil {
		return 0
	}
	return res
}

func isChallengePage(doc *goquery.Document) bool {
	if doc == nil {
		return false
	}
	if doc.Find("#challenge-form").Length() > 0 {
		return true
	}
	if doc.Find(".anomaly-modal__modal").Length() > 0 {
		return true
	}
	text := doc.Text()
	if strings.Contains(text, "Unfortunately, bots use DuckDuckGo too.") {
		return true
	}
	challengeDetected := false
	doc.Find("script").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if strings.Contains(s.Text(), "anomalyDetectionBlock") {
			challengeDetected = true
			return false
		}
		return true
	})
	return challengeDetected
}
