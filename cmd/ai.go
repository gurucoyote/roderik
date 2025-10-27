package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"roderik/internal/ai/history"
	"roderik/internal/ai/llm"
	"roderik/internal/ai/llm/openai"
	aitools "roderik/internal/ai/tools"
)

const (
	defaultAIModel           = "gpt-5"
	defaultAIHistoryWindow   = 16
	defaultAIRequestTimeout  = 90 * time.Second
	systemPromptHeader       = "You are Roderik's integrated AI assistant. Use the provided browser tools to inspect pages, gather evidence, and complete tasks carefully."
	systemPromptGuidelines   = "Guidelines:\n- Prefer calling tools to inspect the live browser when information is uncertain.\n- Confirm before performing destructive or irreversible actions.\n- Keep responses concise when no further action is required.\n- When a tool call returns data, summarize the key points before continuing."
	systemPromptContextIntro = "Current browser context:"
	systemPromptToolsIntro   = "Available tools:"
)

var (
	aiHistoryWindow int

	chatSession *ChatSession

	aiCmd = &cobra.Command{
		Use:     "ai [message]",
		Aliases: []string{"chat"},
		Short:   "Chat with the integrated AI assistant",
		Long:    "Send a prompt to the built-in AI assistant. The assistant can call Roderik tools to interact with the active browser session.",
		Args:    cobra.ArbitraryArgs,
		RunE:    runAICommand,
	}
)

func init() {
	aiCmd.Flags().IntVar(&aiHistoryWindow, "history-window", defaultAIHistoryWindow, "Number of recent AI chat messages to retain (0 keeps the full history)")
	RootCmd.AddCommand(aiCmd)
}

type ChatSession struct {
	provider      llm.Provider
	tools         []llm.Tool
	toolRegistry  map[string]aitools.Definition
	history       []llm.Message
	historyWindow int
}

func runAICommand(cmd *cobra.Command, args []string) error {
	input := strings.TrimSpace(strings.Join(args, " "))
	if input == "" {
		return fmt.Errorf("ai command requires a prompt; try `roderik ai inspect the login form`")
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	session, err := ensureChatSession()
	if err != nil {
		return err
	}
	session.SetHistoryWindow(aiHistoryWindow)

	ctx, cancel := context.WithTimeout(ctx, defaultAIRequestTimeout)
	defer cancel()

	reply, err := session.Send(ctx, input)
	if err != nil {
		return err
	}

	if reply != "" {
		fmt.Println(reply)
	}
	return nil
}

func ensureChatSession() (*ChatSession, error) {
	if chatSession != nil {
		return chatSession, nil
	}

	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	baseURL := strings.TrimSpace(os.Getenv("OPENAI_API_BASE"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	}

	model := strings.TrimSpace(os.Getenv("RODERIK_AI_MODEL"))
	if model == "" {
		model = defaultAIModel
	}

	maxTokens := 0
	if raw := strings.TrimSpace(os.Getenv("RODERIK_AI_MAX_TOKENS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			maxTokens = parsed
		}
	}

	tools, mapping := aitools.LLMTools("roderik")
	provider := openai.NewProvider(apiKey, baseURL, model, "", maxTokens)
	if Verbose {
		provider.SetDebugLogger(func(msg string) {
			fmt.Fprintln(os.Stderr, msg)
		})
	}

	chatSession = &ChatSession{
		provider:      provider,
		tools:         tools,
		toolRegistry:  mapping,
		historyWindow: aiHistoryWindow,
	}
	logAI("session initialized (model=%s base_url=%s tools=%d history_window=%d)", model, baseURL, len(tools), aiHistoryWindow)
	return chatSession, nil
}

func (s *ChatSession) SetHistoryWindow(n int) {
	s.historyWindow = n
	s.prune()
}

func (s *ChatSession) Send(ctx context.Context, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("prompt cannot be empty")
	}

	logAI("user prompt: %s", truncateForLog(input, 512))

	userMsg := &history.HistoryMessage{
		Role: "user",
		Content: []history.ContentBlock{
			{Type: "text", Text: input},
		},
	}
	s.history = append(s.history, userMsg)
	s.prune()

	const maxIterations = 8
	for i := 0; i < maxIterations; i++ {
		s.provider.SetSystemPrompt(buildSystemPrompt(s.tools))

		logAI("llm iteration %d: sending %d history messages with %d tools", i+1, len(s.history), len(s.tools))
		resp, err := s.provider.CreateMessage(ctx, "", s.history, s.tools)
		if err != nil {
			return "", err
		}

		s.history = append(s.history, resp)
		s.prune()

		toolCalls := resp.GetToolCalls()
		if len(toolCalls) == 0 {
			final := truncateForLog(resp.GetContent(), 512)
			logAI("assistant response (iteration %d): %s", i+1, final)
			return resp.GetContent(), nil
		}

		logAI("assistant requested %d tool call(s)", len(toolCalls))
		for _, call := range toolCalls {
			sanitized := call.GetName()
			def, ok := s.toolRegistry[sanitized]
			var resultContent interface{}
			var callErr error
			if !ok {
				callErr = fmt.Errorf("tool %q is not registered", sanitized)
				logAI("tool call skipped: %s (unknown sanitized name)", sanitized)
			} else {
				argsJSON := marshalForLog(call.GetArguments())
				logAI("tool call start: %s (sanitized=%s) args=%s", def.Name, sanitized, argsJSON)
				result, err := aitools.Call(ctx, def.Name, call.GetArguments())
				if err != nil {
					callErr = err
					logAI("tool call error: %s %v", def.Name, err)
				} else {
					resultContent = toolResultPayload(result)
					logAI("tool call success: %s result=%s", def.Name, truncateForLog(summarizeToolResult(result), 512))
				}
			}

			if callErr != nil {
				resultContent = map[string]interface{}{
					"error": callErr.Error(),
				}
			}

			toolMsg, err := s.provider.CreateToolResponse(call.GetID(), resultContent)
			if err != nil {
				return "", err
			}
			s.history = append(s.history, toolMsg)
			s.prune()
		}
	}

	return "", fmt.Errorf("exceeded maximum tool iterations without assistant response")
}

func logAI(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "[AI] "+format+"\n", a...)
}

func truncateForLog(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	if limit <= 3 {
		return s[:limit]
	}
	return s[:limit-3] + "..."
}

func marshalForLog(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return truncateForLog(string(data), 512)
}

func summarizeToolResult(res aitools.Result) string {
	switch {
	case res.Text != "":
		return res.Text
	case len(res.Binary) > 0:
		return fmt.Sprintf("binary response (%d bytes, content_type=%s)", len(res.Binary), res.ContentType)
	case res.FilePath != "":
		return fmt.Sprintf("file saved at %s", res.FilePath)
	case res.InlineURI != "":
		return fmt.Sprintf("inline URI %s", res.InlineURI)
	default:
		return "tool completed with no output"
	}
}

func (s *ChatSession) prune() {
	if s.historyWindow <= 0 {
		return
	}
	if len(s.history) <= s.historyWindow {
		return
	}
	start := len(s.history) - s.historyWindow
	s.history = append([]llm.Message(nil), s.history[start:]...)
}

func buildSystemPrompt(tools []llm.Tool) string {
	var b strings.Builder
	b.WriteString(systemPromptHeader)
	b.WriteString("\n\n")

	if ctx := browserContextSummary(); ctx != "" {
		b.WriteString(systemPromptContextIntro)
		b.WriteString("\n")
		b.WriteString(ctx)
		b.WriteString("\n\n")
	}

	if len(tools) > 0 {
		b.WriteString(systemPromptToolsIntro)
		b.WriteString("\n")
		for _, tool := range tools {
			b.WriteString("- ")
			b.WriteString(tool.Name)
			if tool.Description != "" {
				b.WriteString(": ")
				b.WriteString(tool.Description)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(systemPromptGuidelines)
	return b.String()
}

func browserContextSummary() string {
	if Page == nil {
		return ""
	}
	info, err := Page.Info()
	if err != nil {
		if Verbose {
			fmt.Fprintf(os.Stderr, "warning: failed to fetch page info: %v\n", err)
		}
		return ""
	}

	var lines []string
	if url := strings.TrimSpace(info.URL); url != "" {
		lines = append(lines, "- URL: "+url)
	}
	if title := strings.TrimSpace(info.Title); title != "" {
		lines = append(lines, "- Title: "+title)
	}
	if CurrentElement != nil {
		lines = append(lines, "- A DOM element is currently selected.")
	}
	return strings.Join(lines, "\n")
}

func toolResultPayload(res aitools.Result) interface{} {
	payload := map[string]interface{}{}
	if res.Text != "" {
		payload["text"] = res.Text
	}
	if len(res.Binary) > 0 {
		payload["binary_base64"] = base64.StdEncoding.EncodeToString(res.Binary)
	}
	if res.ContentType != "" {
		payload["content_type"] = res.ContentType
	}
	if res.FilePath != "" {
		payload["file_path"] = res.FilePath
	}
	if res.InlineURI != "" {
		payload["inline_uri"] = res.InlineURI
	}

	switch len(payload) {
	case 0:
		return "tool completed with no output"
	case 1:
		if text, ok := payload["text"]; ok {
			return text
		}
	}
	return payload
}
