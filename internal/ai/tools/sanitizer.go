package tools

import (
    "fmt"
    "regexp"

    "roderik/internal/ai/llm"
)

var invalidChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// SanitizeName namespaces a tool for exposure to LLMs while removing invalid characters.
func SanitizeName(server, tool string) string {
    safe := invalidChars.ReplaceAllString(tool, "_")
    return fmt.Sprintf("%s__%s", server, safe)
}

// LLMTools returns sanitized tool definitions and a mapping back to originals.
func LLMTools(server string) ([]llm.Tool, map[string]Definition) {
    defs := List()
    out := make([]llm.Tool, 0, len(defs))
    mapping := make(map[string]Definition, len(defs))

    for _, def := range defs {
        sanitized := SanitizeName(server, def.Name)
        schema := llm.Schema{
            Type:       "object",
            Properties: map[string]interface{}{},
        }
        for _, param := range def.Parameters {
            prop := map[string]interface{}{
                "type": param.Type.JSONType(),
            }
            if param.Description != "" {
                prop["description"] = param.Description
            }
            if len(param.Enum) > 0 {
                prop["enum"] = param.Enum
            }
            schema.Properties[param.Name] = prop
            if param.Required {
                schema.Required = append(schema.Required, param.Name)
            }
        }
        out = append(out, llm.Tool{
            Name:        sanitized,
            Description: def.Description,
            InputSchema: schema,
        })
        mapping[sanitized] = def
    }

    return out, mapping
}
