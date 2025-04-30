package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

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
			mcp.WithDescription("Navigate the browser to a URL"),
			mcp.WithString("url", mcp.Required(), mcp.Description("the URL to navigate to")),
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
			mcp.WithDescription("Get HTML of the current element"),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if CurrentElement == nil {
				return nil, fmt.Errorf("no page loaded – call load_url first")
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

	if err := server.ServeStdio(s); err != nil {
		log.Printf("MCP server error: %v", err)
	}
}
