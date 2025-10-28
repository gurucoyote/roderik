package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod/lib/proto"
	"github.com/mark3labs/mcp-go/mcp"
	"roderik/browser"
	duckduck "roderik/duckduck"
	aitools "roderik/internal/ai/tools"
	"roderik/internal/appdirs"
)

var registerHandlersOnce sync.Once

func toolDebug(format string, args ...interface{}) {
	debugAI(format, args...)
}

func init() {
	registerHandlers()
}

func registerHandlers() {
	registerHandlersOnce.Do(func() {
		aitools.RegisterHandler("load_url", loadURLHandler)
		aitools.RegisterHandler("text", textHandler)
		aitools.RegisterHandler("get_html", getHTMLHandler)
		aitools.RegisterHandler("capture_screenshot", captureScreenshotHandler)
		aitools.RegisterHandler("capture_pdf", capturePDFHandler)
		aitools.RegisterHandler("to_markdown", toMarkdownHandler)
		aitools.RegisterHandler("run_js", runJSHandler)
		aitools.RegisterHandler("search", searchHandler)
		aitools.RegisterHandler("head", headHandler)
		aitools.RegisterHandler("next", nextHandler)
		aitools.RegisterHandler("prev", prevHandler)
		aitools.RegisterHandler("elem", elemHandler)
		aitools.RegisterHandler("child", childHandler)
		aitools.RegisterHandler("parent", parentHandler)
		aitools.RegisterHandler("html", htmlHandler)
		aitools.RegisterHandler("click", clickHandler)
		aitools.RegisterHandler("type", typeHandler)
		aitools.RegisterHandler("box", boxHandler)
		aitools.RegisterHandler("computedstyles", computedStylesHandler)
		aitools.RegisterHandler("describe", describeHandler)
		aitools.RegisterHandler("xpath", xpathHandler)
		aitools.RegisterHandler("duck", duckHandler)
		aitools.RegisterHandler("network_list", networkListHandler)
		aitools.RegisterHandler("network_save", networkSaveHandler)
		aitools.RegisterHandler("network_set_logging", networkSetLoggingHandler)
		// Additional tool handlers will be registered here as they migrate.
	})
}

func loadURLHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	var url string
	if args != nil {
		if v, ok := args["url"].(string); ok {
			url = v
		}
	}
	url = strings.TrimSpace(url)
	if url == "" {
		return aitools.Result{}, fmt.Errorf("load_url: url argument is required")
	}

	toolDebug("[TOOLS] load_url CALLED args=%#v", args)

	res, err := withPage(func() (aitools.Result, error) {
		page, err := LoadURL(url)
		if err != nil {
			return aitools.Result{}, fmt.Errorf("load_url failed: %w", err)
		}
		body, err := page.Element("body")
		if err != nil {
			return aitools.Result{}, fmt.Errorf("load_url failed to select <body>: %w", err)
		}
		CurrentElement = body
		info, err := page.Info()
		if err != nil {
			return aitools.Result{}, fmt.Errorf("load_url failed to read page info: %w", err)
		}
		msg := fmt.Sprintf("navigated to %s", info.URL)
		return aitools.Result{Text: msg}, nil
	})
	if err != nil {
		return aitools.Result{}, err
	}

	toolDebug("[TOOLS] load_url RESULT: %q", res.Text)
	return res, nil
}

func textHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] text CALLED args=%#v", args)

	var lengthPtr *int
	if args != nil {
		if v, ok := args["length"]; ok {
			if length, ok := toInt(v); ok {
				lengthPtr = &length
			}
		}
	}

	res, err := withPage(func() (aitools.Result, error) {
		text, err := mcpText(lengthPtr)
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: text}, nil
	})
	if err != nil {
		return aitools.Result{}, err
	}

	toolDebug("[TOOLS] text RESULT length=%d", len(res.Text))
	return res, nil
}

func getHTMLHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] get_html CALLED args=%#v", args)

	res, err := withPage(func() (aitools.Result, error) {
		var rawURL string
		if args != nil {
			if v, ok := args["url"].(string); ok {
				rawURL = strings.TrimSpace(v)
			}
		}

		if rawURL != "" {
			buf, ctype, looksHTML, err := probeURL(rawURL, 32*1024)
			if err != nil {
				return aitools.Result{}, fmt.Errorf("get_html probe error: %w", err)
			}
			if !looksHTML {
				data := buf
				if len(data) == 0 {
					resp, err := http.Get(rawURL)
					if err != nil {
						return aitools.Result{}, fmt.Errorf("get_html fetch error: %w", err)
					}
					defer resp.Body.Close()
					data, _ = io.ReadAll(resp.Body)
					ctype = resp.Header.Get("Content-Type")
				}
				text, err := decodeToUTF8(data, ctype)
				if err != nil {
					return aitools.Result{}, fmt.Errorf("get_html decode error: %w", err)
				}
				toolDebug("[TOOLS] get_html non-HTML (%s) returning %d bytes", ctype, len(text))
				return aitools.Result{Text: text}, nil
			}

			page, err := LoadURL(rawURL)
			if err != nil {
				return aitools.Result{}, fmt.Errorf("get_html failed to load url %q: %w", rawURL, err)
			}
			el, err := page.Element("html")
			if err != nil {
				return aitools.Result{}, fmt.Errorf("get_html failed to select <html>: %w", err)
			}
			CurrentElement = el
		}

		if CurrentElement == nil {
			return aitools.Result{}, fmt.Errorf("no page loaded – call load_url first or provide url")
		}
		html, err := CurrentElement.HTML()
		if err != nil {
			return aitools.Result{}, fmt.Errorf("get_html failed to get HTML: %w", err)
		}
		return aitools.Result{Text: html}, nil
	})
	if err != nil {
		return aitools.Result{}, err
	}

	toolDebug("[TOOLS] get_html RESULT length=%d", len(res.Text))
	return res, nil
}

func captureScreenshotHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] capture_screenshot CALLED args=%#v", args)

	return withPage(func() (aitools.Result, error) {
		rawURL := mcp.ExtractString(args, "url")
		if strings.TrimSpace(rawURL) != "" {
			if _, err := LoadURL(rawURL); err != nil {
				return aitools.Result{}, fmt.Errorf("capture_screenshot load url %q: %w", rawURL, err)
			}
		}
		if Page == nil {
			return aitools.Result{}, fmt.Errorf("capture_screenshot: no page loaded – call load_url first or provide url")
		}

		selector := mcp.ExtractString(args, "selector")
		fullPage := boolArg(args, "full_page")
		scroll := boolArg(args, "scroll")
		if selector != "" && scroll {
			return aitools.Result{}, fmt.Errorf("capture_screenshot: selector capture cannot be combined with scroll")
		}
		if selector != "" && fullPage {
			return aitools.Result{}, fmt.Errorf("capture_screenshot: selector capture cannot be combined with full_page")
		}
		if scroll && fullPage {
			return aitools.Result{}, fmt.Errorf("capture_screenshot: choose either scroll or full_page, not both")
		}

		format := strings.TrimSpace(strings.ToLower(mcp.ExtractString(args, "format")))
		if format == "" {
			format = "png"
		}

		var qualityPtr *int
		if v, ok := args["quality"].(float64); ok {
			q := int(v)
			qualityPtr = &q
		}

		opts := browser.ScreenshotOptions{
			Selector: selector,
			FullPage: fullPage,
			Scroll:   scroll,
			Format:   format,
			Quality:  qualityPtr,
		}
		result, err := captureScreenshotFunc(Page, opts)
		if err != nil {
			return aitools.Result{}, err
		}

		delivery := strings.TrimSpace(strings.ToLower(mcp.ExtractString(args, "return")))
		if delivery == "" {
			delivery = "binary"
		}
		if delivery == "binary" && len(result.Data) > inlineBinaryLimit {
			delivery = "file"
		}

		formatExt := "png"
		if isJPEGFormat(format) {
			formatExt = "jpg"
		}

		caption := fmt.Sprintf("Captured screenshot (%s, %d bytes).", result.MimeType, len(result.Data))
		if selector != "" {
			caption = fmt.Sprintf("Captured screenshot of %q (%s, %d bytes).", selector, result.MimeType, len(result.Data))
		} else if rawURL != "" {
			caption = fmt.Sprintf("Captured screenshot of %s (%s, %d bytes).", rawURL, result.MimeType, len(result.Data))
		}

		switch delivery {
		case "file":
			output := mcp.ExtractString(args, "output")
			path, err := resolveOutputPath(output, "", "", "screenshot", formatExt)
			if err != nil {
				return aitools.Result{}, err
			}
			if err := os.WriteFile(path, result.Data, 0644); err != nil {
				return aitools.Result{}, fmt.Errorf("capture_screenshot write file: %w", err)
			}
			toolDebug("[TOOLS] capture_screenshot RESULT saved=%s bytes=%d", path, len(result.Data))
			return aitools.Result{
				Text:        fmt.Sprintf("%s Saved to %s.", caption, path),
				Binary:      result.Data,
				ContentType: result.MimeType,
				FilePath:    path,
			}, nil
		default:
			toolDebug("[TOOLS] capture_screenshot RESULT inline bytes=%d", len(result.Data))
			return aitools.Result{
				Text:        caption,
				Binary:      result.Data,
				ContentType: result.MimeType,
			}, nil
		}
	})
}

func capturePDFHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] capture_pdf CALLED args=%#v", args)

	return withPage(func() (aitools.Result, error) {
		rawURL := mcp.ExtractString(args, "url")
		if strings.TrimSpace(rawURL) != "" {
			if _, err := LoadURL(rawURL); err != nil {
				return aitools.Result{}, fmt.Errorf("capture_pdf load url %q: %w", rawURL, err)
			}
		}
		if Page == nil {
			return aitools.Result{}, fmt.Errorf("capture_pdf: no page loaded – call load_url first or provide url")
		}

		opts := browser.PDFOptions{
			Landscape:               boolArg(args, "landscape"),
			DisplayHeaderFooter:     boolArg(args, "header_footer"),
			PrintBackground:         boolArg(args, "background"),
			PreferCSSPageSize:       boolArg(args, "prefer_css_page_size"),
			GenerateTaggedPDF:       boolArg(args, "tagged"),
			GenerateDocumentOutline: boolArg(args, "outline"),
			PageRanges:              mcp.ExtractString(args, "page_ranges"),
			HeaderTemplate:          mcp.ExtractString(args, "header_template"),
			FooterTemplate:          mcp.ExtractString(args, "footer_template"),
		}

		if v, ok := args["scale"].(float64); ok {
			val := v
			opts.Scale = &val
		}
		if v, ok := args["paper_width"].(float64); ok {
			val := v
			opts.PaperWidth = &val
		}
		if v, ok := args["paper_height"].(float64); ok {
			val := v
			opts.PaperHeight = &val
		}
		if v, ok := args["margin_top"].(float64); ok {
			val := v
			opts.MarginTop = &val
		}
		if v, ok := args["margin_bottom"].(float64); ok {
			val := v
			opts.MarginBottom = &val
		}
		if v, ok := args["margin_left"].(float64); ok {
			val := v
			opts.MarginLeft = &val
		}
		if v, ok := args["margin_right"].(float64); ok {
			val := v
			opts.MarginRight = &val
		}

		result, err := capturePDFFunc(Page, opts)
		if err != nil {
			return aitools.Result{}, err
		}

		delivery := strings.TrimSpace(strings.ToLower(mcp.ExtractString(args, "return")))
		if delivery == "" {
			delivery = "binary"
		}
		if delivery == "binary" && len(result.Data) > inlineBinaryLimit {
			delivery = "file"
		}

		caption := fmt.Sprintf("Captured PDF (%d bytes).", len(result.Data))
		if rawURL != "" {
			caption = fmt.Sprintf("Captured PDF of %s (%d bytes).", rawURL, len(result.Data))
		}

		switch delivery {
		case "file":
			output := mcp.ExtractString(args, "output")
			path, err := resolveOutputPath(output, "", "", "document", "pdf")
			if err != nil {
				return aitools.Result{}, err
			}
			if err := os.WriteFile(path, result.Data, 0644); err != nil {
				return aitools.Result{}, fmt.Errorf("capture_pdf write file: %w", err)
			}
			toolDebug("[TOOLS] capture_pdf RESULT saved=%s bytes=%d", path, len(result.Data))
			return aitools.Result{
				Text:        fmt.Sprintf("%s Saved to %s.", caption, path),
				Binary:      result.Data,
				ContentType: result.MimeType,
				FilePath:    path,
			}, nil
		default:
			toolDebug("[TOOLS] capture_pdf RESULT inline bytes=%d", len(result.Data))
			return aitools.Result{
				Text:        caption,
				Binary:      result.Data,
				ContentType: result.MimeType,
				InlineURI:   "inline:pdf",
			}, nil
		}
	})
}

func toMarkdownHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] to_markdown CALLED args=%#v", args)

	return withPage(func() (aitools.Result, error) {
		var rawURL string
		if args != nil {
			if v, ok := args["url"].(string); ok {
				rawURL = strings.TrimSpace(v)
			}
		}

		if rawURL != "" {
			buf, ctype, looksHTML, err := probeURL(rawURL, 32*1024)
			if err != nil {
				return aitools.Result{}, fmt.Errorf("to_markdown probe error: %w", err)
			}
			if !looksHTML {
				data := buf
				if len(data) == 0 {
					resp, err := http.Get(rawURL)
					if err != nil {
						return aitools.Result{}, fmt.Errorf("to_markdown fetch error: %w", err)
					}
					defer resp.Body.Close()
					data, _ = io.ReadAll(resp.Body)
					ctype = resp.Header.Get("Content-Type")
				}
				text, err := decodeToUTF8(data, ctype)
				if err != nil {
					return aitools.Result{}, fmt.Errorf("to_markdown decode error: %w", err)
				}
				toolDebug("[TOOLS] to_markdown non-HTML (%s) returning %d bytes", ctype, len(text))
				return aitools.Result{Text: text}, nil
			}

			page, err := LoadURL(rawURL)
			if err != nil {
				return aitools.Result{}, fmt.Errorf("to_markdown failed to load url %q: %w", rawURL, err)
			}
			body, err := page.Element("body")
			if err != nil {
				return aitools.Result{}, fmt.Errorf("to_markdown failed to select <body>: %w", err)
			}
			CurrentElement = body
		}

		if CurrentElement == nil {
			return aitools.Result{}, fmt.Errorf("no element selected: use load_url and element-selection tools first")
		}

		if Page != nil {
			Page.Timeout(10 * time.Second).WaitLoad()
		}

		if err := (proto.AccessibilityEnable{}).Call(Page); err != nil {
			return aitools.Result{}, fmt.Errorf("accessibility enable failed: %w", err)
		}

		props, err := CurrentElement.Describe(0, false)
		if err != nil {
			return aitools.Result{}, fmt.Errorf("describe element failed: %w", err)
		}

		tree, err := proto.AccessibilityQueryAXTree{BackendNodeID: props.BackendNodeID}.Call(Page)
		if err != nil {
			return aitools.Result{}, fmt.Errorf("accessibility query failed: %w", err)
		}

		md := convertAXTreeToMarkdown(tree, Page)
		toolDebug("[TOOLS] to_markdown RESULT length=%d", len(md))
		return aitools.Result{Text: md}, nil
	})
}

func runJSHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] run_js CALLED args=%#v", args)

	return withPage(func() (aitools.Result, error) {
		var showErrors bool
		if args != nil {
			if v, ok := args["showErrors"].(bool); ok {
				showErrors = v
			}
		}

		if CurrentElement == nil {
			msg := "run_js error: no element selected—call load_url and navigation tools first"
			if showErrors {
				return aitools.Result{Text: msg}, nil
			}
			return aitools.Result{}, fmt.Errorf(msg)
		}

		script, _ := args["script"].(string)
		script = strings.TrimSpace(script)
		if script == "" {
			return aitools.Result{}, fmt.Errorf("run_js: script argument is required")
		}

		wrapped := fmt.Sprintf("() => { return (%s); }", script)
		value, err := CurrentElement.Eval(wrapped)
		if err != nil {
			if showErrors {
				return aitools.Result{Text: fmt.Sprintf("run_js execution error: %v", err)}, nil
			}
			return aitools.Result{}, fmt.Errorf("run_js execution error: %w", err)
		}

		resultJSON, err := json.Marshal(value.Value)
		if err != nil {
			if showErrors {
				return aitools.Result{Text: fmt.Sprintf("run_js JSON marshal error: %v", err)}, nil
			}
			return aitools.Result{}, fmt.Errorf("run_js JSON marshal error: %w", err)
		}

		toolDebug("[TOOLS] run_js RESULT length=%d", len(resultJSON))
		return aitools.Result{Text: string(resultJSON)}, nil
	})
}

var (
	duckClientOnce sync.Once
	duckClient     duckduck.SearchClient
)

func ensureDuckClient() duckduck.SearchClient {
	duckClientOnce.Do(func() {
		client := duckduck.NewDuckDuckGoSearchClient()
		client.InitialDelay = 0
		client.Backoff = 2 * time.Second
		client.MaxRetries = 2
		duckClient = client
	})
	return duckClient
}

func duckHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] duck CALLED args=%#v", args)

	query := strings.TrimSpace(mcp.ExtractString(args, "query"))
	if query == "" {
		return aitools.Result{}, fmt.Errorf("duck: query argument is required")
	}

	limit := 20
	if v, ok := args["num"].(float64); ok {
		if n := int(v); n > 0 {
			limit = n
		}
	}

	client := ensureDuckClient()
	results, err := client.SearchLimited(query, limit)
	if err != nil {
		return aitools.Result{}, fmt.Errorf("duck search failed: %w", err)
	}
	if len(results) == 0 {
		return aitools.Result{Text: fmt.Sprintf("duck search found no results for %q.", query)}, nil
	}

	var b strings.Builder
	for i, res := range results {
		if i >= limit && limit > 0 {
			break
		}
		url := strings.TrimSpace(res.FormattedUrl)
		title := strings.TrimSpace(res.Title)
		if title == "" {
			title = url
		}
		snippet := strings.TrimSpace(res.Snippet)
		if snippet != "" {
			snippet = truncateContextText(snippet, 220)
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, title)
		if url != "" {
			fmt.Fprintf(&b, "   %s\n", url)
		}
		if snippet != "" {
			fmt.Fprintf(&b, "   %s\n", snippet)
		}
	}

	return aitools.Result{Text: b.String()}, nil
}

func searchHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] search CALLED args=%#v", args)

	return withPage(func() (aitools.Result, error) {
		selector := strings.TrimSpace(mcp.ExtractString(args, "selector"))
		if selector == "" {
			return aitools.Result{}, fmt.Errorf("search: selector is required")
		}
		msg, err := mcpSearch(selector)
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func headHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] head CALLED args=%#v", args)

	return withPage(func() (aitools.Result, error) {
		level := mcp.ExtractString(args, "level")
		msg, err := mcpHead(level)
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func nextHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] next CALLED args=%#v", args)

	return withPage(func() (aitools.Result, error) {
		var idxPtr *int
		if args != nil {
			if v, ok := args["index"]; ok {
				if idx, ok := toInt(v); ok {
					idxPtr = &idx
				}
			}
		}
		msg, err := mcpNext(idxPtr)
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func prevHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] prev CALLED args=%#v", args)

	return withPage(func() (aitools.Result, error) {
		var idxPtr *int
		if args != nil {
			if v, ok := args["index"]; ok {
				if idx, ok := toInt(v); ok {
					idxPtr = &idx
				}
			}
		}
		msg, err := mcpPrev(idxPtr)
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func elemHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] elem CALLED args=%#v", args)

	return withPage(func() (aitools.Result, error) {
		selector := strings.TrimSpace(mcp.ExtractString(args, "selector"))
		if selector == "" {
			return aitools.Result{}, fmt.Errorf("elem: selector is required")
		}
		msg, err := mcpElem(selector)
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func childHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] child CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpChild()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func parentHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] parent CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpParent()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func htmlHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] html CALLED")

	return withPage(func() (aitools.Result, error) {
		html, err := mcpHTML()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: html}, nil
	})
}

func clickHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] click CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpClick()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func typeHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] type CALLED args=%#v", args)

	return withPage(func() (aitools.Result, error) {
		text := strings.TrimSpace(mcp.ExtractString(args, "text"))
		if text == "" {
			return aitools.Result{}, fmt.Errorf("type: text argument is required")
		}
		msg, err := mcpType(text)
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func boxHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] box CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpBox()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func computedStylesHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] computedstyles CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpComputedStyles()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func describeHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] describe CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpDescribe()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func xpathHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] xpath CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpXPath()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

type networkEntrySummary struct {
	RequestID        string `json:"request_id"`
	URL              string `json:"url"`
	Method           string `json:"method"`
	Status           *int   `json:"status,omitempty"`
	MIMEType         string `json:"mime_type,omitempty"`
	ResourceType     string `json:"resource_type,omitempty"`
	EncodedBytes     *int   `json:"encoded_bytes,omitempty"`
	FinishedBytes    *int   `json:"finished_bytes,omitempty"`
	Failure          string `json:"failure,omitempty"`
	Canceled         bool   `json:"canceled,omitempty"`
	HasBody          bool   `json:"has_body"`
	Retrieved        bool   `json:"retrieved,omitempty"`
	RequestTimestamp string `json:"request_time,omitempty"`
	ResponseTime     string `json:"response_time,omitempty"`
}

func networkListHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] network_list CALLED args=%#v", args)

	log := getActiveEventLog()
	if log == nil {
		return aitools.Result{}, fmt.Errorf("network_list: no active network log")
	}

	filter, err := networkFilterFromArgs(args)
	if err != nil {
		return aitools.Result{}, err
	}

	entries := log.FilterEntries(filter)
	summaries := make([]networkEntrySummary, 0, len(entries))
	for _, entry := range entries {
		summaries = append(summaries, summarizeNetworkEntry(entry))
	}

	payload, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return aitools.Result{}, fmt.Errorf("network_list: marshal summaries: %w", err)
	}

	toolDebug("[TOOLS] network_list RESULT count=%d", len(summaries))
	return aitools.Result{Text: string(payload)}, nil
}

func networkSaveHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] network_save CALLED args=%#v", args)

	reqID := strings.TrimSpace(mcp.ExtractString(args, "request_id"))
	if reqID == "" {
		return aitools.Result{}, fmt.Errorf("network_save: request_id is required")
	}
	saveDir := strings.TrimSpace(mcp.ExtractString(args, "save_dir"))
	returnMode := strings.TrimSpace(strings.ToLower(mcp.ExtractString(args, "return")))
	if returnMode == "" {
		returnMode = "binary"
	}
	filenameOverride := strings.TrimSpace(mcp.ExtractString(args, "filename"))

	log := getActiveEventLog()
	if log == nil {
		return aitools.Result{}, fmt.Errorf("network_save: no active network log")
	}

	entry, ok := log.EntryByID(reqID)
	if !ok {
		return aitools.Result{}, fmt.Errorf("network_save: request %s not found", reqID)
	}

	data, err := withPage(func() ([]byte, error) {
		if Page == nil {
			return nil, fmt.Errorf("network_save: no page loaded – call load_url first")
		}
		bytes, err := retrieveNetworkBody(Page, entry)
		if err != nil {
			return nil, err
		}
		return bytes, nil
	})
	if err != nil {
		return aitools.Result{}, err
	}

	updatedEntry, ok := log.EntryByID(reqID)
	if ok {
		entry = updatedEntry
	}

	mimeType := ""
	if entry.Response != nil {
		mimeType = entry.Response.MIMEType
	}
	baseName := suggestFilename(entry, 0)
	if filenameOverride != "" {
		baseName = sanitizeFilename(filenameOverride)
	}

	switch returnMode {
	case "file":
		if saveDir == "" {
			saveDir = defaultDownloadsDir()
		}
		if err := appdirs.EnsureDir(saveDir); err != nil {
			return aitools.Result{}, fmt.Errorf("network_save: create directory: %w", err)
		}
		name := ensureUniqueFilename(saveDir, baseName, make(map[string]int))
		fullPath := filepath.Join(saveDir, name)
		if err := os.WriteFile(fullPath, data, 0o644); err != nil {
			return aitools.Result{}, fmt.Errorf("network_save: write file: %w", err)
		}
		toolDebug("[TOOLS] network_save saved bytes=%d path=%s", len(data), fullPath)
		return aitools.Result{
			Text:        fmt.Sprintf("saved %d bytes to %s", len(data), fullPath),
			FilePath:    fullPath,
			ContentType: mimeType,
		}, nil
	case "binary":
		fallthrough
	default:
		toolDebug("[TOOLS] network_save returning binary size=%d", len(data))
		return aitools.Result{
			Text:        fmt.Sprintf("retrieved %d bytes for %s", len(data), entry.URL),
			Binary:      data,
			ContentType: mimeType,
		}, nil
	}
}

func networkSetLoggingHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	toolDebug("[TOOLS] network_set_logging CALLED args=%#v", args)

	stateChanged := false
	if raw, ok := args["enabled"]; ok {
		enabled, okBool := toBool(raw)
		if !okBool {
			return aitools.Result{}, fmt.Errorf("network_set_logging: enabled must be boolean")
		}
		if setNetworkActivityEnabled(enabled) {
			stateChanged = true
		}
	}

	current := isNetworkActivityEnabled()
	msg := fmt.Sprintf("network logging enabled: %t", current)
	if stateChanged {
		toolDebug("[TOOLS] network_set_logging updated state=%t", current)
	} else {
		toolDebug("[TOOLS] network_set_logging status=%t", current)
	}
	return aitools.Result{Text: msg}, nil
}

func summarizeNetworkEntry(entry *NetworkLogEntry) networkEntrySummary {
	summary := networkEntrySummary{
		RequestID:    entry.RequestID,
		URL:          entry.URL,
		Method:       entry.Method,
		ResourceType: string(entry.ResourceType),
		HasBody:      entry.Response != nil,
	}
	if !entry.RequestTimestamp.IsZero() {
		summary.RequestTimestamp = entry.RequestTimestamp.Format(time.RFC3339)
	}
	if entry.Response != nil {
		status := entry.Response.Status
		summary.Status = &status
		summary.MIMEType = entry.Response.MIMEType
		if entry.Response.EncodedDataLength > 0 {
			encoded := int(entry.Response.EncodedDataLength)
			summary.EncodedBytes = &encoded
		}
		if !entry.Response.ResponseTimestamp.IsZero() {
			summary.ResponseTime = entry.Response.ResponseTimestamp.Format(time.RFC3339)
		}
	}
	if entry.Finished != nil && entry.Finished.EncodedDataLength > 0 {
		finished := int(entry.Finished.EncodedDataLength)
		summary.FinishedBytes = &finished
	}
	if entry.Failure != nil {
		summary.Failure = entry.Failure.ErrorText
		summary.Canceled = entry.Failure.Canceled
	}
	if entry.Body != nil {
		summary.HasBody = true
		summary.Retrieved = true
	}
	return summary
}

func networkFilterFromArgs(args map[string]interface{}) (NetworkLogFilter, error) {
	filter := NetworkLogFilter{}
	filter.MIMESubstrings = normalizeStrings(extractStringSlice(args, "mime"))
	filter.Suffixes = normalizeStrings(extractStringSlice(args, "suffix"))
	filter.TextContains = normalizeStrings(extractStringSlice(args, "contains"))
	filter.Methods = normalizeStrings(extractStringSlice(args, "method"))
	filter.Domains = normalizeStrings(extractStringSlice(args, "domain"))
	filter.StatusCodes = extractIntSlice(args, "status")
	if types := extractStringSlice(args, "type"); len(types) > 0 {
		rts, err := parseResourceTypes(types)
		if err != nil {
			return NetworkLogFilter{}, err
		}
		filter.ResourceTypes = rts
	}
	return filter, nil
}

func extractStringSlice(args map[string]interface{}, key string) []string {
	if args == nil {
		return nil
	}
	val, ok := args[key]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			switch sv := item.(type) {
			case string:
				if strings.TrimSpace(sv) != "" {
					out = append(out, sv)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func extractIntSlice(args map[string]interface{}, key string) []int {
	if args == nil {
		return nil
	}
	val, ok := args[key]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case []int:
		return v
	case []interface{}:
		out := make([]int, 0, len(v))
		for _, item := range v {
			if n, ok := toInt(item); ok {
				out = append(out, n)
			}
		}
		return out
	default:
		if n, ok := toInt(v); ok {
			return []int{n}
		}
		return nil
	}
}

func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int32:
		return int(val), true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case float32:
		return int(val), true
	case string:
		if s := strings.TrimSpace(val); s != "" {
			if parsed, err := strconv.Atoi(s); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func toBool(v interface{}) (bool, bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	case string:
		s := strings.TrimSpace(strings.ToLower(val))
		switch s {
		case "true", "1", "yes", "on", "enabled":
			return true, true
		case "false", "0", "no", "off", "disabled":
			return false, true
		}
	case float64:
		return val != 0, true
	case float32:
		return val != 0, true
	case int:
		return val != 0, true
	case int32:
		return val != 0, true
	case int64:
		return val != 0, true
	}
	return false, false
}
