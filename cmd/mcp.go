package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	aitools "roderik/internal/ai/tools"
	"roderik/internal/appdirs"
)

// path to the MCP debug log file, override with --log
var mcpLogPath string

const inlineBinaryLimit = 512 * 1024

func defaultMCPLogPath() string {
	dir, err := appdirs.LogsDir()
	if err != nil {
		return "roderik-mcp.log"
	}
	return filepath.Join(dir, "roderik-mcp.log")
}

func ensureLogDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || strings.TrimSpace(dir) == "" {
		return nil
	}
	return appdirs.EnsureDir(dir)
}

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
	mcpCmd.Flags().StringVar(&mcpLogPath, "log", defaultMCPLogPath(), "path to the MCP debug log file")

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

	if err := ensureLogDir(mcpLogPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot prepare log directory for %q: %v\n", mcpLogPath, err)
	}

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
		server.WithToolCapabilities(true),
	)

	if loadURLToolEnabled() {
		s.AddTool(
			mcp.NewTool(
				"load_url",
				mcp.WithDescription("Load a webpage at the given URL and set it as the current page for subsequent tools"),
				mcp.WithString("url", mcp.Required(), mcp.Description("the URL of the webpage to load")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL load_url CALLED args=%#v", req.Params.Arguments)
				res, err := aitools.Call(ctx, "load_url", req.Params.Arguments)
				if err != nil {
					return nil, err
				}
				out, err := resultToMCP(res)
				if err != nil {
					return nil, err
				}
				log.Printf("[MCP] TOOL load_url RESULT: %q", res.Text)
				return out, nil
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
			log.Printf("[MCP] TOOL get_html CALLED args=%#v", req.Params.Arguments)
			res, err := aitools.Call(ctx, "get_html", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			out, err := resultToMCP(res)
			if err != nil {
				return nil, err
			}
			log.Printf("[MCP] TOOL get_html RESULT length=%d", len(res.Text))
			return out, nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"text",
			mcp.WithDescription("Print the text of the current element, optionally truncating to a specified length."),
			mcp.WithNumber("length", mcp.Description("optional maximum number of characters to return")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("[MCP] TOOL text CALLED args=%#v", req.Params.Arguments)
			res, err := aitools.Call(ctx, "text", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			out, err := resultToMCP(res)
			if err != nil {
				return nil, err
			}
			log.Printf("[MCP] TOOL text RESULT length=%d", len(res.Text))
			return out, nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"capture_screenshot",
			mcp.WithDescription("Capture a screenshot of the current page or an optional URL."),
			mcp.WithString("url", mcp.Description("optional URL to load before capturing the screenshot")),
			mcp.WithString("selector", mcp.Description("optional CSS selector to capture a specific element")),
			mcp.WithBoolean("full_page", mcp.Description("capture the entire page by resizing the viewport")),
			mcp.WithBoolean("scroll", mcp.Description("scroll and stitch the entire page without resizing the viewport")),
			mcp.WithString("format", mcp.Description("image format: png or jpeg (default png)"), mcp.Enum("png", "jpeg", "jpg"), mcp.DefaultString("png")),
			mcp.WithNumber("quality", mcp.Description("JPEG quality (0-100)")),
			mcp.WithString("return", mcp.Description("delivery mode: binary (inline) or file (writes to disk and returns resource)"), mcp.Enum("binary", "file"), mcp.DefaultString("binary")),
			mcp.WithString("output", mcp.Description("optional path to save the capture on disk when return=file")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("[MCP] TOOL capture_screenshot CALLED args=%#v", req.Params.Arguments)
			res, err := aitools.Call(ctx, "capture_screenshot", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			return resultToMCP(res)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"capture_pdf",
			mcp.WithDescription("Render the current page or an optional URL to PDF."),
			mcp.WithString("url", mcp.Description("optional URL to load before generating the PDF")),
			mcp.WithBoolean("landscape", mcp.Description("render pages in landscape orientation")),
			mcp.WithBoolean("header_footer", mcp.Description("display header and footer templates")),
			mcp.WithBoolean("background", mcp.Description("print background graphics")),
			mcp.WithNumber("scale", mcp.Description("scale factor for rendering (default 1.0)")),
			mcp.WithNumber("paper_width", mcp.Description("paper width in inches")),
			mcp.WithNumber("paper_height", mcp.Description("paper height in inches")),
			mcp.WithNumber("margin_top", mcp.Description("top margin in inches")),
			mcp.WithNumber("margin_bottom", mcp.Description("bottom margin in inches")),
			mcp.WithNumber("margin_left", mcp.Description("left margin in inches")),
			mcp.WithNumber("margin_right", mcp.Description("right margin in inches")),
			mcp.WithString("page_ranges", mcp.Description("page ranges to print, e.g. '1-5,8'")),
			mcp.WithString("header_template", mcp.Description("HTML template for the header")),
			mcp.WithString("footer_template", mcp.Description("HTML template for the footer")),
			mcp.WithBoolean("prefer_css_page_size", mcp.Description("prefer CSS-defined page size")),
			mcp.WithBoolean("tagged", mcp.Description("generate tagged (accessible) PDF")),
			mcp.WithBoolean("outline", mcp.Description("embed document outline in the PDF")),
			mcp.WithString("return", mcp.Description("delivery mode: binary (embedded) or file (writes to disk)"), mcp.Enum("binary", "file"), mcp.DefaultString("binary")),
			mcp.WithString("output", mcp.Description("optional path to save the PDF on disk when return=file")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("[MCP] TOOL capture_pdf CALLED args=%#v", req.Params.Arguments)
			res, err := aitools.Call(ctx, "capture_pdf", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			return resultToMCP(res)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"network_list",
			mcp.WithDescription("List captured network activity entries with optional filters."),
			mcp.WithArray("mime", mcp.Items(map[string]interface{}{"type": "string"})),
			mcp.WithArray("suffix", mcp.Items(map[string]interface{}{"type": "string"})),
			mcp.WithArray("status", mcp.Items(map[string]interface{}{"type": "integer"})),
			mcp.WithArray("contains", mcp.Items(map[string]interface{}{"type": "string"})),
			mcp.WithArray("method", mcp.Items(map[string]interface{}{"type": "string"})),
			mcp.WithArray("domain", mcp.Items(map[string]interface{}{"type": "string"})),
			mcp.WithArray("type", mcp.Items(map[string]interface{}{"type": "string"})),
			mcp.WithNumber("limit", mcp.Description("maximum number of entries to return (default 20, capped at 1000)")),
			mcp.WithNumber("offset", mcp.Description("number of matching entries to skip before returning results")),
			mcp.WithBoolean("tail", mcp.Description("when true (default) return the newest matching entries")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("[MCP] TOOL network_list CALLED args=%#v", req.Params.Arguments)
			res, err := aitools.Call(ctx, "network_list", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			out, err := resultToMCP(res)
			if err != nil {
				return nil, err
			}
			log.Printf("[MCP] TOOL network_list RESULT length=%d", len(res.Text))
			return out, nil
		},
	)

		s.AddTool(
			mcp.NewTool(
				"network_save",
				mcp.WithDescription("Retrieve or persist the response body for a captured network request."),
				mcp.WithString("request_id", mcp.Required(), mcp.Description("request identifier returned by network_list")),
				mcp.WithString("return", mcp.Description("delivery mode: file (default) saves on the server, binary streams the payload, save aliases file"), mcp.Enum("file", "binary", "save"), mcp.DefaultString("file")),
				mcp.WithString("save_dir", mcp.Description("optional directory to write the file when return=file")),
				mcp.WithString("filename", mcp.Description("optional filename override when saving to disk")),
				mcp.WithString("filename_prefix", mcp.Description("optional prefix prepended to generated filenames")),
				mcp.WithString("filename_suffix", mcp.Description("optional suffix appended before the file extension")),
				mcp.WithBoolean("filename_timestamp", mcp.Description("include a timestamp in the filename")),
				mcp.WithString("timestamp_format", mcp.Description("time format used when filename_timestamp is true (Go layout)")),
			),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("[MCP] TOOL network_save CALLED args=%#v", req.Params.Arguments)
			res, err := aitools.Call(ctx, "network_save", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			out, err := resultToMCP(res)
			if err != nil {
				return nil, err
			}
			if res.Binary != nil {
				log.Printf("[MCP] TOOL network_save RESULT binary length=%d", len(res.Binary))
			} else {
				log.Printf("[MCP] TOOL network_save RESULT %q", res.Text)
			}
			return out, nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"network_set_logging",
			mcp.WithDescription("Enable, disable, or query network activity logging without restarting Roderik."),
			mcp.WithBoolean("enabled", mcp.Description("optional flag; when provided sets logging state to the given value")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("[MCP] TOOL network_set_logging CALLED args=%#v", req.Params.Arguments)
			res, err := aitools.Call(ctx, "network_set_logging", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			out, err := resultToMCP(res)
			if err != nil {
				return nil, err
			}
			log.Printf("[MCP] TOOL network_set_logging RESULT %q", res.Text)
			return out, nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"box",
			mcp.WithDescription("Get the bounding box of the current element."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("[MCP] TOOL box CALLED")
			res, err := aitools.Call(ctx, "box", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			return resultToMCP(res)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"computedstyles",
			mcp.WithDescription("Output the computed styles of the current element in JSON format."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("[MCP] TOOL computedstyles CALLED")
			res, err := aitools.Call(ctx, "computedstyles", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			return resultToMCP(res)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"describe",
			mcp.WithDescription("Describe the current element as formatted JSON."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("[MCP] TOOL describe CALLED")
			res, err := aitools.Call(ctx, "describe", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			return resultToMCP(res)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"xpath",
			mcp.WithDescription("Get the optimized XPath of the current element."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("[MCP] TOOL xpath CALLED")
			res, err := aitools.Call(ctx, "xpath", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			return resultToMCP(res)
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
			log.Printf("[MCP] TOOL to_markdown CALLED args=%#v", req.Params.Arguments)
			res, err := aitools.Call(ctx, "to_markdown", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			return resultToMCP(res)
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
				log.Printf("[MCP] TOOL search CALLED args=%#v", req.Params.Arguments)
				res, err := aitools.Call(ctx, "search", req.Params.Arguments)
				if err != nil {
					return nil, err
				}
				return resultToMCP(res)
			},
		)

		s.AddTool(
			mcp.NewTool(
				"head",
				mcp.WithDescription("List page headings (optionally by level), focus the first match, and return a numbered index."),
				mcp.WithString("level", mcp.Description("Heading level number (1-6)")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL head CALLED args=%#v", req.Params.Arguments)
				res, err := aitools.Call(ctx, "head", req.Params.Arguments)
				if err != nil {
					return nil, err
				}
				return resultToMCP(res)
			},
		)

		s.AddTool(
			mcp.NewTool(
				"next",
				mcp.WithDescription("Advance to the next element in the active search/head list or jump to a specific index."),
				mcp.WithNumber("index", mcp.Description("optional index to jump to")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL next CALLED args=%#v", req.Params.Arguments)
				res, err := aitools.Call(ctx, "next", req.Params.Arguments)
				if err != nil {
					return nil, err
				}
				return resultToMCP(res)
			},
		)

		s.AddTool(
			mcp.NewTool(
				"prev",
				mcp.WithDescription("Move to the previous element in the active search/head list or jump to a specific index."),
				mcp.WithNumber("index", mcp.Description("optional index to jump to")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL prev CALLED args=%#v", req.Params.Arguments)
				res, err := aitools.Call(ctx, "prev", req.Params.Arguments)
				if err != nil {
					return nil, err
				}
				return resultToMCP(res)
			},
		)

		s.AddTool(
			mcp.NewTool(
				"elem",
				mcp.WithDescription("Match elements by selector (scoped to the current element, falling back to the page), focus the best match, and return a numbered list."),
				mcp.WithString("selector", mcp.Required(), mcp.Description("CSS selector to resolve")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL elem CALLED args=%#v", req.Params.Arguments)
				res, err := aitools.Call(ctx, "elem", req.Params.Arguments)
				if err != nil {
					return nil, err
				}
				return resultToMCP(res)
			},
		)

		s.AddTool(
			mcp.NewTool(
				"child",
				mcp.WithDescription("Focus the first child element of the current selection."),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL child CALLED")
				res, err := aitools.Call(ctx, "child", req.Params.Arguments)
				if err != nil {
					return nil, err
				}
				return resultToMCP(res)
			},
		)

		s.AddTool(
			mcp.NewTool(
				"parent",
				mcp.WithDescription("Focus the parent element of the current selection."),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL parent CALLED")
				res, err := aitools.Call(ctx, "parent", req.Params.Arguments)
				if err != nil {
					return nil, err
				}
				return resultToMCP(res)
			},
		)

		s.AddTool(
			mcp.NewTool(
				"html",
				mcp.WithDescription("Return the outer HTML of the current element that prior navigation selected."),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL html CALLED")
				res, err := aitools.Call(ctx, "html", req.Params.Arguments)
				if err != nil {
					return nil, err
				}
				return resultToMCP(res)
			},
		)

		s.AddTool(
			mcp.NewTool(
				"click",
				mcp.WithDescription("Click the currently focused element; falls back to href navigation or synthetic click on failure."),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL click CALLED")
				res, err := aitools.Call(ctx, "click", req.Params.Arguments)
				if err != nil {
					return nil, err
				}
				return resultToMCP(res)
			},
		)

		s.AddTool(
			mcp.NewTool(
				"type",
				mcp.WithDescription("Type text into the currently focused element; trims optional quotes and falls back to JavaScript value injection."),
				mcp.WithString("text", mcp.Required(), mcp.Description("Text to type")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				log.Printf("[MCP] TOOL type CALLED args=%#v", req.Params.Arguments)
				res, err := aitools.Call(ctx, "type", req.Params.Arguments)
				if err != nil {
					return nil, err
				}
				return resultToMCP(res)
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
			log.Printf("[MCP] TOOL run_js CALLED args=%#v", req.Params.Arguments)
			res, err := aitools.Call(ctx, "run_js", req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			return resultToMCP(res)
		},
	)

	if err := server.ServeStdio(s); err != nil {
		log.Printf("MCP server error: %v", err)
	}
}

func resultToMCP(res aitools.Result) (*mcp.CallToolResult, error) {
	if len(res.Binary) > 0 {
		if res.ContentType == "" {
			return nil, fmt.Errorf("binary result missing content type")
		}
		payload := base64.StdEncoding.EncodeToString(res.Binary)
		caption := res.Text
		if caption == "" {
			caption = fmt.Sprintf("Binary result (%s, %d bytes)", res.ContentType, len(res.Binary))
		}

		if res.FilePath != "" {
			resource := mcp.BlobResourceContents{
				URI:      toFileURI(res.FilePath),
				MIMEType: res.ContentType,
				Blob:     payload,
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: caption},
					mcp.EmbeddedResource{Type: "resource", Resource: resource},
				},
			}, nil
		}

		if strings.HasPrefix(res.ContentType, "image/") {
			return mcp.NewToolResultImage(caption, payload, res.ContentType), nil
		}

		uri := res.InlineURI
		if uri == "" {
			uri = fmt.Sprintf("inline:%s", res.ContentType)
		}
		resource := mcp.BlobResourceContents{
			URI:      uri,
			MIMEType: res.ContentType,
			Blob:     payload,
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: caption},
				mcp.EmbeddedResource{Type: "resource", Resource: resource},
			},
		}, nil
	}
	return mcp.NewToolResultText(res.Text), nil
}

// helper function for logging large results
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func boolArg(args map[string]interface{}, key string) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return false
}

func mcpHandleCaptureScreenshot(args map[string]interface{}) (*mcp.CallToolResult, error) {
	res, err := aitools.Call(context.Background(), "capture_screenshot", args)
	if err != nil {
		return nil, err
	}
	return resultToMCP(res)
}

func mcpHandleCapturePDF(args map[string]interface{}) (*mcp.CallToolResult, error) {
	res, err := aitools.Call(context.Background(), "capture_pdf", args)
	if err != nil {
		return nil, err
	}
	return resultToMCP(res)
}
