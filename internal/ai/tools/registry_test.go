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
		"network_list",
		"network_save",
		"network_set_logging",
	}

	for _, name := range required {
		if _, ok := tools.Lookup(name); !ok {
			t.Fatalf("expected tool %q to be present in registry", name)
		}
	}
}

func TestLookupHasDescriptions(t *testing.T) {
	defNames := []string{
		"load_url",
		"get_html",
		"text",
		"capture_screenshot",
		"to_markdown",
		"network_list",
		"network_save",
		"network_set_logging",
	}
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

	saveDef, ok := tools.Lookup("network_save")
	if !ok {
		t.Fatalf("network_save not found")
	}
	if len(saveDef.Parameters) == 0 {
		t.Fatalf("network_save expected parameters")
	}
	var requestParam *tools.Parameter
	var returnParam *tools.Parameter
	for i := range saveDef.Parameters {
		param := &saveDef.Parameters[i]
		if param.Name == "request_id" {
			requestParam = param
		}
		if param.Name == "return" {
			returnParam = param
		}
	}
	if requestParam == nil || !requestParam.Required || requestParam.Type != tools.ParamString {
		t.Fatalf("network_save request_id parameter mismatch: %#v", requestParam)
	}
	if returnParam == nil || len(returnParam.Enum) == 0 {
		t.Fatalf("network_save return parameter enum missing: %#v", returnParam)
	}

	loggingDef, ok := tools.Lookup("network_set_logging")
	if !ok {
		t.Fatalf("network_set_logging not found")
	}
	if len(loggingDef.Parameters) != 1 {
		t.Fatalf("network_set_logging expected 1 parameter, got %d", len(loggingDef.Parameters))
	}
	lp := loggingDef.Parameters[0]
	if lp.Name != "enabled" || lp.Type != tools.ParamBoolean {
		t.Fatalf("network_set_logging enabled parameter mismatch: %#v", lp)
	}
}
