package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
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

func runMCP(cmd *cobra.Command, args []string) {
	// FORCE the standard logger to stderr
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

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

	// 1) create the server over stdio
	server := NewServer(os.Stdin, os.Stdout)


	// register load_url
	server.RegisterTool("load_url", func(raw json.RawMessage) (interface{}, error) {
		log.Printf("→ tool=load_url raw args=%s", string(raw))
		var args LoadURLArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			log.Printf("✗ load_url unmarshal error: %v", err)
			return nil, fmt.Errorf("load_url failed: %w", err)
		}
		page, err := LoadURL(args.URL)
		if err != nil {
			log.Printf("✗ load_url error: %v", err)
			return nil, fmt.Errorf("load_url failed: %w", err)
		}
		CurrentElement = page.MustElement("html")
		msg := fmt.Sprintf("navigated to %s", page.MustInfo().URL)
		log.Printf("✓ load_url response=%q", msg)
		return msg, nil
	})

	// register get_html
	server.RegisterTool("get_html", func(_ json.RawMessage) (interface{}, error) {
		log.Printf("→ tool=get_html")
		if CurrentElement == nil {
			err := fmt.Errorf("no page loaded – call load_url first")
			log.Printf("✗ get_html error: %v", err)
			return nil, err
		}
		html := CurrentElement.MustHTML()
		log.Printf("✓ get_html response length=%d", len(html))
		return html, nil
	})

	// 4) channel to signal shutdown
	done := make(chan struct{})

	// 5) start serving MCP requests with logging
	go func() {
		if err := server.Serve(); err != nil {
			log.Printf("server.Serve() error: %v", err)
		} else {
			log.Printf("server.Serve() exited cleanly")
		}
	}()

	// register shutdown tool
	server.RegisterTool("shutdown", func(_ json.RawMessage) (interface{}, error) {
		log.Printf("→ tool=shutdown")
		close(done)
		return "shutting down", nil
	})

	// 7) wait for shutdown
	<-done
}
