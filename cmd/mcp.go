package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod/lib/proto"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

// path to the MCP debug log file, override with --log
var mcpLogPath string

// LoadURLArgs is the JSON schema for the load_url tool.
type LoadURLArgs struct {
	URL string `json:"url" jsonschema:"required,description=the URL to navigate to"`
}

// HTMLArgs is an empty struct because get_html takes no arguments.
type HTMLArgs struct{}

// mcpCmd is the cobra subcommand which will start our MCP server.
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run Roderik in MCP‐server mode over stdio",
	Run:   runMCP,
}

func init() {
	RootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().StringVarP(&mcpLogPath, "log", "l", "roderik-mcp.log", "path to the MCP debug log file")

	// ensure cobra’s own help/errors go to stderr
	mcpCmd.SetOut(os.Stderr)
	mcpCmd.SetErr(os.Stderr)
}

type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	ArgsSchema  json.RawMessage `json:"args_schema"`
}

func loadURLToolEnabled() bool {
	val := strings.TrimSpace(os.Getenv("RODERIK_ENABLE_LOAD_URL"))
	if val == "" {
		return true
	}
	val = strings.ToLower(val)
	switch val {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func navigationToolsEnabled() bool {
	return loadURLToolEnabled()
}

func runMCP(cmd *cobra.Command, args []string) {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("[MCP] starting server name=%q version=%q", "roderik", "1.0.0")

	f, err := os.OpenFile(mcpLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot open mcp log %q: %v\n", mcpLogPath, err)
	} else {
		defer f.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, f))
	}

	s := server.NewMCPServer(
		"roderik",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	if loadURLToolEnabled() {
		s.AddTool(
			mcp.NewTool(
				"load_url",
				mcp.WithDescription("Load a webpage at the given URL and set it as the current page for subsequent tools"),
				mcp.WithString("url", mcp.Required(), mcp.Description("the URL of the webpage to load")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return withPage(func() (*mcp.CallToolResult, error) {
					log.Printf("[MCP] TOOL load_url CALLED args=%#v", req.Params.Arguments)
					url, _ := req.Params.Arguments["url"].(string)
					page, err := LoadURL(url)
					if err != nil {
						return nil, fmt.Errorf("load_url failed: %w", err)
					}
					body, err := page.Element("body")
					if err != nil {
						return nil, fmt.Errorf("load_url failed to select <body>: %w", err)
					}
					CurrentElement = body
					msg := fmt.Sprintf("navigated to %s", page.MustInfo().URL)
					result := mcp.NewToolResultText(msg)
					log.Printf("[MCP] TOOL load_url RESULT: %q", msg)
					return result, nil
				})
			},
		)
	}

	s.AddTool(
		mcp.NewTool(
			"get_html",
			mcp.WithDescription(
				"Get the raw HTML of the current element (or an optional URL). "+
					"Beware: this returns the full source and can be very large. "+
					"In most cases, use \"to_markdown\" for a more concise, token-efficient output.",
			),
			mcp.WithString(
				"url",
				mcp.Description("optional URL to load first; overrides the current element"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return withPage(func() (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL get_html CALLED args=%#v", req.Params.Arguments)
				if raw, ok := req.Params.Arguments["url"].(string); ok && raw != "" {
					// First probe the resource – if it's not HTML we simply return its raw body.
					buf, ctype, looksHTML, err := probeURL(raw, 32*1024)
					if err != nil {
						return nil, fmt.Errorf("get_html probe error: %w", err)
					}
					if !looksHTML {
						data := buf
						if len(data) == 0 {
							resp, err := http.Get(raw)
							if err != nil {
								return nil, fmt.Errorf("get_html fetch error: %w", err)
							}
							defer resp.Body.Close()
							data, _ = io.ReadAll(resp.Body)
							ctype = resp.Header.Get("Content-Type")
						}
						text, err := decodeToUTF8(data, ctype)
						if err != nil {
							return nil, fmt.Errorf("get_html decode error: %w", err)
						}
						log.Printf("[MCP] TOOL get_html non-HTML (%s) returning %d bytes", ctype, len(text))
						return mcp.NewToolResultText(text), nil
					}

					// HTML → load with browser so we can access <html> element.
					page, err := LoadURL(raw)
					if err != nil {
						return nil, fmt.Errorf("get_html failed to load url %q: %w", raw, err)
					}
					el, err := page.Element("html")
					if err != nil {
						return nil, fmt.Errorf("get_html failed to select <html>: %w", err)
					}
					CurrentElement = el
				}
				if CurrentElement == nil {
					return nil, fmt.Errorf("no page loaded – call load_url first or provide url")
				}
				html, err := CurrentElement.HTML()
				if err != nil {
					return nil, fmt.Errorf("get_html failed to get HTML: %w", err)
				}
				result := mcp.NewToolResultText(html)
				log.Printf("[MCP] TOOL get_html RESULT length=%d", len(html))
				return result, nil
			})
		},
	)

	var shutdownOnce sync.Once

	s.AddTool(
		mcp.NewTool("shutdown", mcp.WithDescription("Shut down the MCP server")),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			shutdownOnce.Do(func() {
				go func() {
					time.Sleep(200 * time.Millisecond) // allow stdio response to flush
					log.Printf("[MCP] shutdown requested – exiting")
					os.Exit(0)
				}()
			})
			return mcp.NewToolResultText("shutting down"), nil
		},
	)

	// === DuckDuckGo keyword search ===
	s.AddTool(
		mcp.NewTool(
			"duck",
			mcp.WithDescription("Search DuckDuckGo and return top N results"),
			mcp.WithString("query", mcp.Required(), mcp.Description("the search terms")),
			mcp.WithNumber("num", mcp.Description("how many results to return (default 20)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("[MCP] TOOL duck CALLED args=%#v", req.Params.Arguments)
			// unwrap arguments
			q, _ := req.Params.Arguments["query"].(string)
			n := numResults
			if v, ok := req.Params.Arguments["num"].(float64); ok {
				n = int(v)
			}
			// invoke the same logic as the CLI
			out, err := searchDuck(q, n)
			if err != nil {
				log.Printf("[MCP] TOOL duck ERROR: %v", err)
				return nil, fmt.Errorf("duck tool failed: %w", err)
			}
			log.Printf("[MCP] TOOL duck RESULT first100=%q", out[:min(len(out), 100)])
			return mcp.NewToolResultText(out), nil
		},
	)

	// === Convert page to Markdown document ===
	s.AddTool(
		mcp.NewTool(
			"to_markdown",
			mcp.WithDescription(
				"Convert the current page/element (or an optional URL) into a structured Markdown document. "+
					"This produces a well-formatted, token-efficient summary. "+
					"Use this instead of \"get_html\" unless you specifically need raw HTML.",
			),
			mcp.WithString(
				"url",
				mcp.Description("optional URL to load first; overrides the current element"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return withPage(func() (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL to_markdown CALLED args=%#v", req.Params.Arguments)
				// if a URL was passed in, handle it first
				if raw, ok := req.Params.Arguments["url"].(string); ok && raw != "" {
					// Fast MIME probe to avoid loading non-HTML resources in the browser
					buf, ctype, looksHTML, err := probeURL(raw, 32*1024)
					if err != nil {
						return nil, fmt.Errorf("to_markdown probe error: %w", err)
					}
					if !looksHTML {
						// Non-HTML → just return raw body/markdown
						data := buf
						if len(data) == 0 {
							resp, err := http.Get(raw)
							if err != nil {
								return nil, fmt.Errorf("to_markdown fetch error: %w", err)
							}
							defer resp.Body.Close()
							data, _ = io.ReadAll(resp.Body)
							ctype = resp.Header.Get("Content-Type")
						}
						text, err := decodeToUTF8(data, ctype)
						if err != nil {
							return nil, fmt.Errorf("to_markdown decode error: %w", err)
						}
						log.Printf("[MCP] TOOL to_markdown non-HTML (%s) returning %d bytes", ctype, len(text))
						return mcp.NewToolResultText(text), nil
					}

					page, err := LoadURL(raw)
					if err != nil {
						return nil, fmt.Errorf("to_markdown failed to load url %q: %w", raw, err)
					}
					body, err := page.Element("body")
					if err != nil {
						return nil, fmt.Errorf("to_markdown failed to select <body>: %w", err)
					}
					CurrentElement = body
				}
				if CurrentElement == nil {
					return nil, fmt.Errorf("no element selected: use load_url and element-selection tools first")
				}
				// Describe current element to get backend node ID
				props, err := CurrentElement.Describe(0, false)
				if err != nil {
					return nil, fmt.Errorf("describe element failed: %w", err)
				}
				// Query the accessibility tree
				tree, err := proto.AccessibilityQueryAXTree{BackendNodeID: props.BackendNodeID}.Call(Page)
				if err != nil {
					return nil, fmt.Errorf("accessibility query failed: %w", err)
				}
				// Generate structured Markdown using shared converter
				md := convertAXTreeToMarkdown(tree, Page)
				log.Printf("[MCP] TOOL to_markdown RESULT length=%d", len(md))
				return mcp.NewToolResultText(md), nil
			})
		},
	)

	if navigationToolsEnabled() {
		// === DOM navigation helpers ===
		s.AddTool(
			mcp.NewTool(
				"search",
				mcp.WithDescription("Search for elements matching a CSS selector, focus the first match, and return a numbered list for subsequent navigation commands."),
				mcp.WithString("selector", mcp.Required(), mcp.Description("CSS selector to query")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return withPage(func() (*mcp.CallToolResult, error) {
					selector, _ := req.Params.Arguments["selector"].(string)
					msg, err := mcpSearch(selector)
					if err != nil {
						return nil, err
					}
					return mcp.NewToolResultText(msg), nil
				})
			},
		)

		s.AddTool(
			mcp.NewTool(
				"head",
				mcp.WithDescription("List page headings (optionally by level), focus the first match, and return a numbered index."),
				mcp.WithString("level", mcp.Description("Heading level number (1-6)")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return withPage(func() (*mcp.CallToolResult, error) {
					level, _ := req.Params.Arguments["level"].(string)
					msg, err := mcpHead(level)
					if err != nil {
						return nil, err
					}
					return mcp.NewToolResultText(msg), nil
				})
			},
		)

		s.AddTool(
			mcp.NewTool(
				"next",
				mcp.WithDescription("Advance to the next element in the active search/head list or jump to a specific index."),
				mcp.WithNumber("index", mcp.Description("optional index to jump to")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return withPage(func() (*mcp.CallToolResult, error) {
					var idxPtr *int
					if v, ok := req.Params.Arguments["index"].(float64); ok {
						i := int(v)
						idxPtr = &i
					}
					msg, err := mcpNext(idxPtr)
					if err != nil {
						return nil, err
					}
					return mcp.NewToolResultText(msg), nil
				})
			},
		)

		s.AddTool(
			mcp.NewTool(
				"prev",
				mcp.WithDescription("Move to the previous element in the active search/head list or jump to a specific index."),
				mcp.WithNumber("index", mcp.Description("optional index to jump to")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return withPage(func() (*mcp.CallToolResult, error) {
					var idxPtr *int
					if v, ok := req.Params.Arguments["index"].(float64); ok {
						i := int(v)
						idxPtr = &i
					}
					msg, err := mcpPrev(idxPtr)
					if err != nil {
						return nil, err
					}
					return mcp.NewToolResultText(msg), nil
				})
			},
		)

		s.AddTool(
			mcp.NewTool(
				"elem",
				mcp.WithDescription("Match elements by selector (scoped to the current element, falling back to the page), focus the best match, and return a numbered list."),
				mcp.WithString("selector", mcp.Required(), mcp.Description("CSS selector to resolve")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return withPage(func() (*mcp.CallToolResult, error) {
					selector, _ := req.Params.Arguments["selector"].(string)
					msg, err := mcpElem(selector)
					if err != nil {
						return nil, err
					}
					return mcp.NewToolResultText(msg), nil
				})
			},
		)

		s.AddTool(
			mcp.NewTool(
				"child",
				mcp.WithDescription("Focus the first child element of the current selection."),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return withPage(func() (*mcp.CallToolResult, error) {
					msg, err := mcpChild()
					if err != nil {
						return nil, err
					}
					return mcp.NewToolResultText(msg), nil
				})
			},
		)

		s.AddTool(
			mcp.NewTool(
				"parent",
				mcp.WithDescription("Focus the parent element of the current selection."),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return withPage(func() (*mcp.CallToolResult, error) {
					msg, err := mcpParent()
					if err != nil {
						return nil, err
					}
					return mcp.NewToolResultText(msg), nil
				})
			},
		)

		s.AddTool(
			mcp.NewTool(
				"html",
				mcp.WithDescription("Return the outer HTML of the current element that prior navigation selected."),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return withPage(func() (*mcp.CallToolResult, error) {
					html, err := mcpHTML()
					if err != nil {
						return nil, err
					}
					return mcp.NewToolResultText(html), nil
				})
			},
		)
	}

	// === Execute arbitrary JS on the page and return JSON results ===
	s.AddTool(
		mcp.NewTool(
			"run_js",
			mcp.WithDescription(
				`Execute JavaScript on the current page and return the result as JSON.
Wrap your code in an IIFE that returns a JSON‐serializable value. Example:

  (() => {
    // Extract all anchor links
    const links = Array.from(document.querySelectorAll('a')).map(a => ({
      href: a.href,
      text: a.textContent.trim(),
    }));
    return links;
  })()
`,
			),
			mcp.WithString(
				"script",
				mcp.Required(), mcp.Description("JavaScript code to execute in the page context"),
			),
			mcp.WithBoolean(
				"showErrors",
				mcp.Description("if true, return any evaluation errors in the tool result text"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return withPage(func() (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL run_js CALLED args=%#v", req.Params.Arguments)

				// extract showErrors flag
				var showErrors bool
				if v, ok := req.Params.Arguments["showErrors"].(bool); ok {
					showErrors = v
				}
				if CurrentElement == nil {
					msg := "run_js error: no element selected—call load_url and navigation tools first"
					if showErrors {
						return mcp.NewToolResultText(msg), nil
					}
					return nil, fmt.Errorf(msg)
				}
				script, _ := req.Params.Arguments["script"].(string)
				// wrap any JS snippet in an IIFE so Element.Eval sees a function literal
				wrapped := fmt.Sprintf("() => { return (%s); }", script)
				value, err := CurrentElement.Eval(wrapped)
				if err != nil {
					if showErrors {
						return mcp.NewToolResultText(fmt.Sprintf("run_js execution error: %v", err)), nil
					}
					return nil, fmt.Errorf("run_js execution error: %w", err)
				}
				resultJSON, err := json.Marshal(value.Value)
				if err != nil {
					if showErrors {
						return mcp.NewToolResultText(fmt.Sprintf("run_js JSON marshal error: %v", err)), nil
					}
					return nil, fmt.Errorf("run_js JSON marshal error: %w", err)
				}
				log.Printf("[MCP] TOOL run_js RESULT length=%d", len(resultJSON))
				return mcp.NewToolResultText(string(resultJSON)), nil
			})
		},
	)

	if err := server.ServeStdio(s); err != nil {
		log.Printf("MCP server error: %v", err)
	}
}

// helper function for logging large results
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
