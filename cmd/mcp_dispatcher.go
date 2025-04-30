package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Request is the MCP request envelope.
type Request struct {
	ID   int             `json:"id"`
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

// Response is the MCP response envelope.
type Response struct {
	ID      int         `json:"id"`
	Content interface{} `json:"content"`
}

// Handler processes raw JSON args and returns a result or error.
type Handler func(json.RawMessage) (interface{}, error)

// Server holds registered tool handlers and I/O streams.
type Server struct {
	tools map[string]Handler
	in    io.Reader
	out   io.Writer
}

// NewServer creates a new MCP Server on the given I/O streams.
func NewServer(in io.Reader, out io.Writer) *Server {
	return &Server{tools: make(map[string]Handler), in: in, out: out}
}

// RegisterTool adds a tool handler to the server.
func (s *Server) RegisterTool(name string, h Handler) {
	s.tools[name] = h
}

// Serve starts reading requests, dispatching to handlers, and writing responses.
func (s *Server) Serve() error {
	scanner := bufio.NewScanner(s.in)
	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return fmt.Errorf("decode req: %w", err)
		}
		h, ok := s.tools[req.Tool]
		if !ok {
			b, _ := json.Marshal(map[string]interface{}{
				"id":    req.ID,
				"error": "unknown tool: " + req.Tool,
			})
			s.out.Write(b)
			s.out.Write([]byte("\n"))
			continue
		}
		result, err := h(req.Args)
		envelope := Response{ID: req.ID, Content: result}
		if err != nil {
			envelope.Content = map[string]string{"error": err.Error()}
		}
		b, _ := json.Marshal(envelope)
		s.out.Write(b)
		s.out.Write([]byte("\n"))
	}
	return scanner.Err()
}
