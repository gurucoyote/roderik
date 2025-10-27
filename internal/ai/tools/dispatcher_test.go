package tools_test

import (
    "context"
    "testing"

    "roderik/internal/ai/tools"
)

func TestRegisterAndCall(t *testing.T) {
    t.Helper()

    // ensure registry starts clean
    tools.ResetHandlersForTest()

    tools.RegisterHandler("load_url", func(ctx context.Context, args map[string]interface{}) (tools.Result, error) {
        return tools.Result{Text: "ok"}, nil
    })

    res, err := tools.Call(context.Background(), "load_url", map[string]interface{}{"url": "https://example.com"})
    if err != nil {
        t.Fatalf("call failed: %v", err)
    }
    if res.Text != "ok" {
        t.Fatalf("unexpected result: %#v", res)
    }
}

func TestCallUnknownTool(t *testing.T) {
    tools.ResetHandlersForTest()
    _, err := tools.Call(context.Background(), "unknown", nil)
    if err == nil {
        t.Fatalf("expected error for unknown tool")
    }
}
