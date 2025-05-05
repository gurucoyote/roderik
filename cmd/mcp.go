package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/go-rod/rod/lib/proto"
	"strings"
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

	s.AddTool(
		mcp.NewTool(
			"load_url",
			mcp.WithDescription("Load a webpage at the given URL and set it as the current page for subsequent tools"),
			mcp.WithString("url", mcp.Required(), mcp.Description("the URL of the webpage to load")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			url, _ := req.Params.Arguments["url"].(string)
			page, err := LoadURL(url)
			if err != nil {
				return nil, fmt.Errorf("load_url failed: %w", err)
			}
			CurrentElement = page.MustElement("html")
			msg := fmt.Sprintf("navigated to %s", page.MustInfo().URL)
			return mcp.NewToolResultText(msg), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"get_html",
			mcp.WithDescription("Get HTML of the current element, or load and return HTML from an optional URL"),
			mcp.WithString("url", mcp.Description("optional URL to load and get HTML from; if provided, overrides the current element")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if raw, ok := req.Params.Arguments["url"].(string); ok && raw != "" {
				// loading URL will reset current element to <html>
				page, err := LoadURL(raw)
				if err != nil {
					return nil, fmt.Errorf("get_html failed to load url %q: %w", raw, err)
				}
				CurrentElement = page.MustElement("html")
			}
			if CurrentElement == nil {
				return nil, fmt.Errorf("no page loaded – call load_url first or provide url")
			}
			html := CurrentElement.MustHTML()
			return mcp.NewToolResultText(html), nil
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
			// unwrap arguments
			q, _ := req.Params.Arguments["query"].(string)
			n := numResults
			if v, ok := req.Params.Arguments["num"].(float64); ok {
				n = int(v)
			}
			// invoke the same logic as the CLI
			out, err := searchDuck(q, n)
			if err != nil {
				return nil, fmt.Errorf("duck tool failed: %w", err)
			}
			return mcp.NewToolResultText(out), nil
		},
	)

	// === Convert page to Markdown document ===
	s.AddTool(
		mcp.NewTool(
			"to_markdown",
			mcp.WithDescription("Convert the current page/element into a full Markdown document"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			// Build human-readable outline
			var sb strings.Builder
			for _, node := range tree.Nodes {
				if node.Ignored {
					continue
				}
				role := node.Role.Value.String()
				switch role {
				case "LineBreak":
					sb.WriteString("\n")
				case "listitem":
					sb.WriteString("- ")
				case "link", "button", "textbox":
					sb.WriteString(role + "(" + fmt.Sprint(node.BackendDOMNodeID) + ") ")
				case "separator":
					sb.WriteString("---\n")
				default:
					sb.WriteString(role + ": ")
				}
				if node.Name != nil {
					sb.WriteString(node.Name.Value.String())
				}
				sb.WriteString("\n")
			}
			return mcp.NewToolResultText(sb.String()), nil
		},
	)

	if err := server.ServeStdio(s); err != nil {
		log.Printf("MCP server error: %v", err)
	}
}
