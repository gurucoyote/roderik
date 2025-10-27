package cmd

import "testing"

func TestSynthesizeInlineToolCalls(t *testing.T) {
	content := `<tool_call>roderik__duck
<arg_key>query</arg_key>
<arg_value>Martin Spernau author biography</arg_value>
</tool_call>
<tool_call>roderik__duck
<arg_key>query</arg_key>
<arg_value>Martin Spernau author biography</arg_value>
</tool_call>`

	calls := synthesizeInlineToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 synthesized tool call, got %d", len(calls))
	}

	if name := calls[0].GetName(); name != "roderik__duck" {
		t.Fatalf("unexpected tool name %q", name)
	}

	args := calls[0].GetArguments()
	if args["query"] != "Martin Spernau author biography" {
		t.Fatalf("unexpected query arg: %#v", args["query"])
	}
}

func TestSynthesizeInlineToolCallsNoMatch(t *testing.T) {
	if calls := synthesizeInlineToolCalls("no tool calls here"); len(calls) != 0 {
		t.Fatalf("expected no synthesized calls, got %d", len(calls))
	}
}
