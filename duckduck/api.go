package client

import (
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

func NewDuckDuckGoSearchClient() *DuckDuckGoSearchClient {
	jar, _ := cookiejar.New(nil)
	httpClient := &http.Client{Jar: jar}
	return &DuckDuckGoSearchClient{
		baseUrl:      "https://duckduckgo.com/html/",
		MaxRetries:   3,
		InitialDelay: 5 * time.Second,
		Backoff:      4 * time.Second,
		client:       httpClient,
		UserAgent:    "Mozilla/5.0 (compatible; RoderikBot/1.0; +https://example.com)",
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
