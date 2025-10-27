package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Result captures the outcome of a tool call.
type Result struct {
	Text        string
	Binary      []byte
	ContentType string
	FilePath    string
	InlineURI   string
}

// Handler executes a tool by name.
type Handler func(ctx context.Context, args map[string]interface{}) (Result, error)

var (
	handlersMu sync.RWMutex
	handlers   = make(map[string]Handler)
)

// ErrUnknownTool indicates the requested tool has no registered handler.
var ErrUnknownTool = errors.New("unknown tool")

// RegisterHandler associates a tool name with an executable handler.
func RegisterHandler(name string, h Handler) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	handlers[name] = h
}

// Call executes the handler registered for the given tool name.
func Call(ctx context.Context, name string, args map[string]interface{}) (Result, error) {
	handlersMu.RLock()
	h, ok := handlers[name]
	handlersMu.RUnlock()
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrUnknownTool, name)
	}
	return h(ctx, args)
}

// ResetHandlersForTest clears registered handlers (test helper).
func ResetHandlersForTest() {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	handlers = make(map[string]Handler)
}
