package client

import (
	"fmt"
	"net/http"
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
	baseUrl    string
	MaxRetries int
	Backoff    time.Duration
}

func NewDuckDuckGoSearchClient() *DuckDuckGoSearchClient {
	return &DuckDuckGoSearchClient{
		baseUrl:    "https://duckduckgo.com/html/",
		MaxRetries: 3,
		Backoff:    1 * time.Second,
	}
}
func (c *DuckDuckGoSearchClient) Search(query string) ([]Result, error) {
	return c.SearchLimited(query, 0)
}

func (c *DuckDuckGoSearchClient) SearchLimited(query string, limit int) ([]Result, error) {
	queryUrl := c.baseUrl + "?q=" + url.QueryEscape(query)

	var resp *http.Response
	var err error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		resp, err = http.Get(queryUrl)
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
			time.Sleep(c.Backoff)
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
