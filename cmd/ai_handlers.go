package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/go-rod/rod/lib/proto"
	"github.com/mark3labs/mcp-go/mcp"
	"roderik/browser"
	aitools "roderik/internal/ai/tools"
)

var registerHandlersOnce sync.Once

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

	log.Printf("[TOOLS] load_url CALLED args=%#v", args)

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
		msg := fmt.Sprintf("navigated to %s", page.MustInfo().URL)
		return aitools.Result{Text: msg}, nil
	})
	if err != nil {
		return aitools.Result{}, err
	}

	log.Printf("[TOOLS] load_url RESULT: %q", res.Text)
	return res, nil
}

func textHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] text CALLED args=%#v", args)

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

	log.Printf("[TOOLS] text RESULT length=%d", len(res.Text))
	return res, nil
}

func getHTMLHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] get_html CALLED args=%#v", args)

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
				log.Printf("[TOOLS] get_html non-HTML (%s) returning %d bytes", ctype, len(text))
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

	log.Printf("[TOOLS] get_html RESULT length=%d", len(res.Text))
	return res, nil
}

func captureScreenshotHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] capture_screenshot CALLED args=%#v", args)

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
			log.Printf("[TOOLS] capture_screenshot RESULT saved=%s bytes=%d", path, len(result.Data))
			return aitools.Result{
				Text:        fmt.Sprintf("%s Saved to %s.", caption, path),
				Binary:      result.Data,
				ContentType: result.MimeType,
				FilePath:    path,
			}, nil
		default:
			log.Printf("[TOOLS] capture_screenshot RESULT inline bytes=%d", len(result.Data))
			return aitools.Result{
				Text:        caption,
				Binary:      result.Data,
				ContentType: result.MimeType,
			}, nil
		}
	})
}

func capturePDFHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] capture_pdf CALLED args=%#v", args)

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
			log.Printf("[TOOLS] capture_pdf RESULT saved=%s bytes=%d", path, len(result.Data))
			return aitools.Result{
				Text:        fmt.Sprintf("%s Saved to %s.", caption, path),
				Binary:      result.Data,
				ContentType: result.MimeType,
				FilePath:    path,
			}, nil
		default:
			log.Printf("[TOOLS] capture_pdf RESULT inline bytes=%d", len(result.Data))
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
	log.Printf("[TOOLS] to_markdown CALLED args=%#v", args)

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
				log.Printf("[TOOLS] to_markdown non-HTML (%s) returning %d bytes", ctype, len(text))
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
		log.Printf("[TOOLS] to_markdown RESULT length=%d", len(md))
		return aitools.Result{Text: md}, nil
	})
}

func runJSHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] run_js CALLED args=%#v", args)

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

		log.Printf("[TOOLS] run_js RESULT length=%d", len(resultJSON))
		return aitools.Result{Text: string(resultJSON)}, nil
	})
}

func searchHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] search CALLED args=%#v", args)

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
	log.Printf("[TOOLS] head CALLED args=%#v", args)

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
	log.Printf("[TOOLS] next CALLED args=%#v", args)

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
	log.Printf("[TOOLS] prev CALLED args=%#v", args)

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
	log.Printf("[TOOLS] elem CALLED args=%#v", args)

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
	log.Printf("[TOOLS] child CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpChild()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func parentHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] parent CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpParent()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func htmlHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] html CALLED")

	return withPage(func() (aitools.Result, error) {
		html, err := mcpHTML()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: html}, nil
	})
}

func clickHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] click CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpClick()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func typeHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] type CALLED args=%#v", args)

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
	log.Printf("[TOOLS] box CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpBox()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func computedStylesHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] computedstyles CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpComputedStyles()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func describeHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] describe CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpDescribe()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
}

func xpathHandler(ctx context.Context, args map[string]interface{}) (aitools.Result, error) {
	log.Printf("[TOOLS] xpath CALLED")

	return withPage(func() (aitools.Result, error) {
		msg, err := mcpXPath()
		if err != nil {
			return aitools.Result{}, err
		}
		return aitools.Result{Text: msg}, nil
	})
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
