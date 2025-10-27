package history

import (
	"encoding/json"
	"strings"

	"roderik/internal/ai/llm"
)

// HistoryMessage implements the llm.Message interface for stored messages
type HistoryMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

func (m *HistoryMessage) GetRole() string {
	return m.Role
}

func (m *HistoryMessage) GetContent() string {
	// Concatenate all text content blocks
	var content string
	for _, block := range m.Content {
		if block.Type == "text" {
			content += block.Text + " "
		}
	}
	return strings.TrimSpace(content)
}

func (m *HistoryMessage) GetToolCalls() []llm.ToolCall {
	var calls []llm.ToolCall
	for _, block := range m.Content {
		if block.Type == "tool_use" {
			calls = append(calls, &HistoryToolCall{
				id:   block.ID,
				name: block.Name,
				args: block.Input,
			})
		}
	}
	return calls
}

func (m *HistoryMessage) IsToolResponse() bool {
	for _, block := range m.Content {
		if block.Type == "tool_result" {
			return true
		}
	}
	return false
}

func (m *HistoryMessage) GetToolResponseID() string {
	for _, block := range m.Content {
		if block.Type == "tool_result" {
			return block.ToolUseID
		}
	}
	return ""
}

func (m *HistoryMessage) GetUsage() (int, int) {
	return 0, 0 // History doesn't track usage
}

const (
	maxUserContentLen      = 800
	maxAssistantContentLen = 900
	maxToolContentLen      = 900
)

// NewUserMessage creates a user history entry with trimmed content.
func NewUserMessage(text string) *HistoryMessage {
	content := summarizeText(text, maxUserContentLen)
	return &HistoryMessage{
		Role: "user",
		Content: []ContentBlock{
			{
				Type: "text",
				Text: content,
			},
		},
	}
}

// CloneAssistantMessage converts an assistant/provider message into a compact history entry.
func CloneAssistantMessage(msg llm.Message) llm.Message {
	if msg == nil {
		return nil
	}
	role := msg.GetRole()
	if role == "" {
		role = "assistant"
	}
	h := &HistoryMessage{Role: role}

	if text := summarizeText(msg.GetContent(), maxAssistantContentLen); text != "" {
		h.Content = append(h.Content, ContentBlock{
			Type: "text",
			Text: text,
		})
	}

	for _, call := range msg.GetToolCalls() {
		if call == nil {
			continue
		}
		args := call.GetArguments()
		data, _ := json.Marshal(args)
		h.Content = append(h.Content, ContentBlock{
			Type: "tool_use",
			ID:   call.GetID(),
			Name: call.GetName(),
			Input: func() json.RawMessage {
				if len(data) == 0 {
					return nil
				}
				return data
			}(),
		})
	}

	if len(h.Content) == 0 {
		return nil
	}
	return h
}

// CloneToolMessage converts a tool response message into a compact history entry.
func CloneToolMessage(msg llm.Message) llm.Message {
	if msg == nil {
		return nil
	}
	text := summarizeText(msg.GetContent(), maxToolContentLen)
	if text == "" && msg.GetToolResponseID() == "" {
		return nil
	}

	block := ContentBlock{
		Type:      "tool_result",
		ToolUseID: msg.GetToolResponseID(),
		Text:      text,
	}

	return &HistoryMessage{
		Role:    "tool",
		Content: []ContentBlock{block},
	}
}

func summarizeText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" || limit <= 0 {
		return ""
	}
	normalized := strings.Join(strings.Fields(text), " ")
	if len([]rune(normalized)) <= limit {
		return normalized
	}
	runes := []rune(normalized)
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

// HistoryToolCall implements llm.ToolCall for stored tool calls
type HistoryToolCall struct {
	id   string
	name string
	args json.RawMessage
}

func (t *HistoryToolCall) GetID() string {
	return t.id
}

func (t *HistoryToolCall) GetName() string {
	return t.name
}

func (t *HistoryToolCall) GetArguments() map[string]interface{} {
	var args map[string]interface{}
	if err := json.Unmarshal(t.args, &args); err != nil {
		return make(map[string]interface{})
	}
	return args
}

// ContentBlock represents a block of content in a message
type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Content   interface{}     `json:"content,omitempty"`
}
