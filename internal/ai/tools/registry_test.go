package tools_test

import (
	"testing"

	"roderik/internal/ai/tools"
)

func TestListIncludesCoreTools(t *testing.T) {
	defs := tools.List()
	if len(defs) == 0 {
		t.Fatalf("expected tool definitions, got none")
	}

	required := []string{
    "load_url",
    "get_html",
    "text",
    "capture_screenshot",
    "capture_pdf",
    "to_markdown",
    "search",
    "click",
    "run_js",
    "duck",
    "shutdown",
	}

	for _, name := range required {
		if _, ok := tools.Lookup(name); !ok {
			t.Fatalf("expected tool %q to be present in registry", name)
		}
	}
}

func TestLookupHasDescriptions(t *testing.T) {
    defNames := []string{"load_url", "get_html", "text", "capture_screenshot", "to_markdown"}
    for _, name := range defNames {
        def, ok := tools.Lookup(name)
        if !ok {
            t.Fatalf("expected tool %q to exist", name)
        }
        if def.Description == "" {
            t.Fatalf("expected description for tool %q", name)
        }
    }
}

func TestParameterMetadata(t *testing.T) {
    def, ok := tools.Lookup("load_url")
    if !ok {
        t.Fatalf("load_url not found")
    }
    if len(def.Parameters) != 1 {
        t.Fatalf("load_url expected 1 parameter, got %d", len(def.Parameters))
    }
    p := def.Parameters[0]
    if p.Name != "url" || !p.Required || p.Type != tools.ParamString {
        t.Fatalf("load_url parameter mismatch: %#v", p)
    }

    textDef, ok := tools.Lookup("text")
    if !ok {
        t.Fatalf("text not found")
    }
    if len(textDef.Parameters) != 1 {
        t.Fatalf("text expected 1 parameter, got %d", len(textDef.Parameters))
    }
    tp := textDef.Parameters[0]
    if tp.Type != tools.ParamNumber || tp.Name != "length" || tp.Required {
        t.Fatalf("text length parameter mismatch: %#v", tp)
    }

    typeDef, ok := tools.Lookup("type")
    if !ok {
        t.Fatalf("type not found")
    }
    if len(typeDef.Parameters) != 1 {
        t.Fatalf("type expected 1 parameter, got %d", len(typeDef.Parameters))
    }
    tp = typeDef.Parameters[0]
    if tp.Type != tools.ParamString || tp.Name != "text" || !tp.Required {
        t.Fatalf("type text parameter mismatch: %#v", tp)
    }
}
