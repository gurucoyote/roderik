package cmd

import (
	"fmt"
	"io"
	"log"
	"os"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
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
}

func runMCP(cmd *cobra.Command, args []string) {
	// 0) open log file
	f, err := os.OpenFile(mcpLogPath,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot open mcp log %q: %v\n", mcpLogPath, err)
	} else {
		mw := io.MultiWriter(os.Stderr, f)
		log.SetOutput(mw)
		defer f.Close()
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("=== starting MCP server ===")

	// 1) create the server over stdio
	server := mcp.NewServer(stdio.NewStdioServerTransport())

	// helper to log and fatal on registration error
	must := func(err error) {
		if err != nil {
			log.Fatalf("failed to register tool: %v", err)
		}
	}

	// 2) register load_url with logging
	must(server.RegisterTool(
		"load_url",
		"Navigate the browser to the given URL",
		func(a map[string]any) (*mcp.ToolResponse, error) {
			log.Printf("→ tool=load_url args=%+v", a)
			urlVal, ok := a["url"].(string)
			if !ok {
				err := fmt.Errorf("load_url: missing or invalid url arg: %v", a["url"])
				log.Printf("✗ load_url error: %v", err)
				return nil, fmt.Errorf("load_url failed: %w", err)
			}
			page, err := LoadURL(urlVal)
			if err != nil {
				log.Printf("✗ load_url error: %v", err)
				return nil, fmt.Errorf("load_url failed: %w", err)
			}
			CurrentElement = page.MustElement("html")
			msg := fmt.Sprintf("navigated to %s", page.MustInfo().URL)
			resp := mcp.NewToolResponse(mcp.NewTextContent(msg))
			log.Printf("✓ load_url response=%q", msg)
			return resp, nil
		},
	))

	// 3) register get_html with logging
	must(server.RegisterTool(
		"get_html",
		"Return the HTML of the current page",
		func(_ map[string]any) (*mcp.ToolResponse, error) {
			log.Printf("→ tool=get_html")
			if CurrentElement == nil {
				err := fmt.Errorf("no page loaded – call load_url first")
				log.Printf("✗ get_html error: %v", err)
				return nil, err
			}
			html := CurrentElement.MustHTML()
			resp := mcp.NewToolResponse(mcp.NewTextContent(html))
			log.Printf("✓ get_html response length=%d", len(html))
			return resp, nil
		},
	))

	// 4) channel to signal shutdown
	done := make(chan struct{})

	// 5) start serving MCP requests with logging
	go func() {
		log.Printf("server.Serve() starting…")
		if err := server.Serve(); err != nil {
			log.Printf("server.Serve() error: %v", err)
		} else {
			log.Printf("server.Serve() exited cleanly")
		}
		close(done)
	}()

	// 6) register shutdown tool with logging
	must(server.RegisterTool(
		"shutdown",
		"Gracefully shut down the MCP server",
		func(_ map[string]any) (*mcp.ToolResponse, error) {
			log.Printf("→ tool=shutdown")
			close(done)
			return mcp.NewToolResponse(mcp.NewTextContent("shutting down")), nil
		},
	))

	// 7) wait for shutdown
	<-done
	log.Printf("=== MCP server exiting ===")
}
