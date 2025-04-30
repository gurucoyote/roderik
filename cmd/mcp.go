package cmd

import (
	"fmt"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"github.com/spf13/cobra"
)

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
}

func runMCP(cmd *cobra.Command, args []string) {
	// 1) create the server over stdio
	server := mcp.NewServer(stdio.NewStdioServerTransport())

	// helper to panic on registration error
	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	// 2) register load_url
	must(server.RegisterTool(
		"load_url",
		"Navigate the browser to the given URL",
		func(a LoadURLArgs) (*mcp.ToolResponse, error) {
			page, err := LoadURL(a.URL)
			if err != nil {
				return nil, fmt.Errorf("load_url failed: %w", err)
			}
			// set the current element to <html>
			CurrentElement = page.MustElement("html")
			return mcp.NewToolResponse(
				mcp.NewTextContent(fmt.Sprintf("navigated to %s", page.MustInfo().URL)),
			), nil
		},
	))

	// 3) register get_html
	must(server.RegisterTool(
		"get_html",
		"Return the HTML of the current page",
		func(_ HTMLArgs) (*mcp.ToolResponse, error) {
			if CurrentElement == nil {
				return nil, fmt.Errorf("no page loaded – call load_url first")
			}
			html := CurrentElement.MustHTML()
			return mcp.NewToolResponse(
				mcp.NewTextContent(html),
			), nil
		},
	))

	// 4) start serving MCP requests (this will block)
	if err := server.Serve(); err != nil {
		panic(err)
	}
}
