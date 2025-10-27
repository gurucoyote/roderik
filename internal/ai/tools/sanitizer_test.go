package tools_test

import (
	"testing"

	"roderik/internal/ai/tools"
)

func TestSanitizeName(t *testing.T) {
	got := tools.SanitizeName("roderik", "get_html")
	want := "roderik__get_html"
	if got != want {
		t.Fatalf("sanitize mismatch: got %q want %q", got, want)
	}

	got = tools.SanitizeName("roderik", "capture-screenshot")
	want = "roderik__capture-screenshot"
	if got != want {
		t.Fatalf("sanitize mismatch: got %q want %q", got, want)
	}

	got = tools.SanitizeName("roderik", "run js!")
	want = "roderik__run_js_"
	if got != want {
		t.Fatalf("sanitize mismatch: got %q want %q", got, want)
	}
}

func TestLLMToolsProvideMapping(t *testing.T) {
	toolsList, mapping := tools.LLMTools("roderik")
	if len(toolsList) == 0 {
		t.Fatalf("expected tools for LLM")
	}
	if len(mapping) != len(toolsList) {
		t.Fatalf("expected mapping size %d, got %d", len(toolsList), len(mapping))
	}

	names := make(map[string]bool)
	for _, tool := range toolsList {
		if tool.Name == "" {
			t.Fatalf("tool missing sanitized name: %#v", tool)
		}
		if names[tool.Name] {
			t.Fatalf("duplicate sanitized name: %s", tool.Name)
		}
		names[tool.Name] = true

		orig, ok := mapping[tool.Name]
		if !ok {
			t.Fatalf("missing mapping for %s", tool.Name)
		}
		if orig.Name == "" || orig.Description == "" {
			t.Fatalf("original definition missing data: %#v", orig)
		}

		if tool.InputSchema.Type != "object" {
			t.Fatalf("expected schema object type, got %s", tool.InputSchema.Type)
		}
		// ensure properties exist
		if tool.InputSchema.Properties == nil {
			t.Fatalf("expected properties map for tool %s", tool.Name)
		}
	}
}
