package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"io"
	"log"
	"os"

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

	if os.Getenv("RODERIK_ENABLE_LOAD_URL") == "1" || strings.ToLower(os.Getenv("RODERIK_ENABLE_LOAD_URL")) == "true" {
		s.AddTool(
			mcp.NewTool(
				"load_url",
			mcp.WithDescription("Load a webpage at the given URL and set it as the current page for subsequent tools"),
			mcp.WithString("url", mcp.Required(), mcp.Description("the URL of the webpage to load")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			if raw, ok := req.Params.Arguments["url"].(string); ok && raw != "" {
				// loading URL will reset current element to <html>
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
		},
	)

	s.AddTool(
		mcp.NewTool("shutdown", mcp.WithDescription("Shut down the MCP server")),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText("shutting down"), context.Canceled
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
			// if a URL was passed in, load it first (and reset CurrentElement to <body>)
			if raw, ok := req.Params.Arguments["url"].(string); ok && raw != "" {
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
		},
	)

	// === Execute arbitrary JS on the page and return JSON results ===
	s.AddTool(
		mcp.NewTool(
			"run_js",
			mcp.WithDescription(
				`Execute JavaScript on the current page (or an optional URL) and return the result as JSON.
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
				"url",
				mcp.Description("optional URL to load first; overrides the current element"),
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

			// extract showErrors flag
			var showErrors bool
			if v, ok := req.Params.Arguments["showErrors"].(bool); ok {
				showErrors = v
			}
			if raw, ok := req.Params.Arguments["url"].(string); ok && raw != "" {
				page, err := LoadURL(raw)
				if err != nil {
					if showErrors {
						return mcp.NewToolResultText(fmt.Sprintf("run_js load_url error: %v", err)), nil
					}
					return nil, fmt.Errorf("run_js load_url error: %w", err)
				}
				body, err := page.Element("body")
				if err != nil {
					if showErrors {
						return mcp.NewToolResultText(fmt.Sprintf("run_js failed to select <body>: %v", err)), nil
					}
					return nil, fmt.Errorf("run_js failed to select <body>: %w", err)
				}
				CurrentElement = body
			}
			script, _ := req.Params.Arguments["script"].(string)
			if CurrentElement == nil {
				if showErrors {
					return mcp.NewToolResultText("run_js error: no element selected—call load_url first or provide url"), nil
				}
				return nil, fmt.Errorf("run_js error: no element selected—call load_url first or provide url")
			}
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
