package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	neturl "net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"roderik/internal/ai/history"
	"roderik/internal/ai/llm"
	"roderik/internal/ai/llm/openai"
	"roderik/internal/ai/profile"
	aitools "roderik/internal/ai/tools"
)

const (
	defaultAIModel           = "gpt-5"
	defaultAIHistoryWindow   = 16
	defaultAIRequestTimeout  = 90 * time.Second
	systemPromptHeader       = "You are Roderik's integrated AI assistant. Use the provided browser tools to inspect pages, gather evidence, and complete tasks carefully."
	systemPromptGuidelines   = "Guidelines:\n- Prefer calling tools to inspect the live browser when information is uncertain.\n- Confirm before performing destructive or irreversible actions.\n- Keep responses concise when no further action is required.\n- When a tool call returns data, summarize the key points before continuing.\n- Default to the currently loaded page for evidence; only use external search tools (e.g., duck) when the user explicitly requests web search.\n- Tools operate on the currently focused element; use parent/child/head/next or reload the page to broaden scope before summarizing full-page content.\n- After navigation, Roderik auto-focuses the first visible heading; verify or adjust the selection before assuming page-wide context."
	systemPromptContextIntro = "Current browser context:"
	systemPromptToolsIntro   = "Available tools:"
	systemPromptTimeIntro    = "Current datetime (system clock):"
)

var (
	aiHistoryWindow int
	aiModelProfile  string

	chatSession *ChatSession

	aiCmd = &cobra.Command{
		Use:          "ai [message]",
		Aliases:      []string{"chat"},
		Short:        "Chat with the integrated AI assistant",
		Long:         "Send a prompt to the built-in AI assistant. The assistant can call Roderik tools to interact with the active browser session.",
		Args:         cobra.ArbitraryArgs,
		RunE:         runAICommand,
		SilenceUsage: true,
	}
)

func init() {
	aiCmd.Flags().IntVar(&aiHistoryWindow, "history-window", defaultAIHistoryWindow, "Number of recent AI chat messages to retain (0 keeps the full history)")
	aiCmd.Flags().StringVarP(&aiModelProfile, "model", "m", "", "Model profile to use for the AI assistant (defaults to config or environment)")
	RootCmd.AddCommand(aiCmd)
}

type ChatSession struct {
	provider              llm.Provider
	tools                 []llm.Tool
	toolRegistry          map[string]aitools.Definition
	history               []llm.Message
	historyWindow         int
	baseSystemPrompt      string
	totalPromptTokens     int64
	totalCompletionTokens int64
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

	loader := profile.Loader{}
	modelProfile, err := loader.Load(aiModelProfile)
	if err != nil {
		return nil, err
	}

	apiKey := strings.TrimSpace(modelProfile.APIKey)
	if apiKey == "" {
		profileName := modelProfile.Name
		if profileName == "" {
			profileName = "(default)"
		}
		return nil, fmt.Errorf("no API key configured; set OPENAI_API_KEY or define one for the %q model profile", profileName)
	}

	providerName := strings.ToLower(strings.TrimSpace(modelProfile.Provider))
	if providerName == "" {
		providerName = "openai"
	}

	if providerName != "openai" {
		profileName := modelProfile.Name
		if profileName == "" {
			profileName = "(default)"
		}
		return nil, fmt.Errorf("model profile %q references unsupported provider %q", profileName, modelProfile.Provider)
	}

	if strings.TrimSpace(modelProfile.Model) == "" {
		modelProfile.Model = defaultAIModel
	}

	tools, mapping := aitools.LLMTools("roderik")
	provider := openai.NewProvider(apiKey, modelProfile.BaseURL, modelProfile.Model, modelProfile.SystemPrompt, modelProfile.MaxTokens)
	if Verbose {
		provider.SetDebugLogger(func(msg string) {
			fmt.Fprintln(os.Stderr, msg)
		})
	}

	chatSession = &ChatSession{
		provider:         provider,
		tools:            tools,
		toolRegistry:     mapping,
		historyWindow:    aiHistoryWindow,
		baseSystemPrompt: strings.TrimSpace(modelProfile.SystemPrompt),
	}
	logAI("Ready with profile %s (%s via %s)",
		profileNameOrDefault(modelProfile.Name),
		modelProfile.Model,
		providerName,
	)
	debugAI("session initialized profile=%s provider=%s model=%s base_url=%s max_tokens=%d tools=%d history_window=%d",
		profileNameOrDefault(modelProfile.Name),
		providerName,
		modelProfile.Model,
		modelProfile.BaseURL,
		modelProfile.MaxTokens,
		len(tools),
		aiHistoryWindow,
	)
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

	userMsg := history.NewUserMessage(input)
	s.history = append(s.history, userMsg)
	s.prune()

	const maxIterations = 8
	var lastToolSummary string
	turnSteps := make([]string, 0, 8)
	turnPromptTokens := 0
	turnCompletionTokens := 0
	for i := 0; i < maxIterations; i++ {
		focusHint := focusAwareHint()
		toolsForCall := s.toolsWithFocusHint(focusHint)
		fullPrompt := buildSystemPrompt(toolsForCall)
		if s.baseSystemPrompt != "" {
			fullPrompt = s.baseSystemPrompt + "\n\n" + fullPrompt
		}
		s.provider.SetSystemPrompt(fullPrompt)
		historyLen := len(s.history)
		resp, err := s.provider.CreateMessage(ctx, "", s.history, toolsForCall)
		if err != nil {
			if isTimeoutError(err) {
				debugAI("llm iteration %d timeout: %v", i+1, err)
				if len(s.history) > 0 && s.history[len(s.history)-1] == userMsg {
					s.history = s.history[:len(s.history)-1]
				}
				s.prune()
				return "Timed out waiting for the model (90s). Please retry or simplify the request.", nil
			}
			return "", err
		}
		promptTokens, completionTokens := resp.GetUsage()
		s.totalPromptTokens += int64(promptTokens)
		s.totalCompletionTokens += int64(completionTokens)
		turnPromptTokens += promptTokens
		turnCompletionTokens += completionTokens
		debugAI(
			"llm iteration %d: history=%d tools=%d prompt_tokens=%s completion_tokens=%s",
			i+1,
			historyLen,
			len(toolsForCall),
			formatTokenCount(promptTokens),
			formatTokenCount(completionTokens),
		)

		if cloned := history.CloneAssistantMessage(resp); cloned != nil {
			s.history = append(s.history, cloned)
		}
		s.prune()

		toolCalls := resp.GetToolCalls()
		if len(toolCalls) == 0 {
			if inline := synthesizeInlineToolCalls(resp.GetContent()); len(inline) > 0 {
				debugAI("detected inline tool call markup count=%d", len(inline))
				toolCalls = inline
			}
		}
		if len(toolCalls) == 0 {
			debugAI("assistant response iteration=%d", i+1)
			logTurnSummary(turnSteps, turnPromptTokens, turnCompletionTokens, s.totalPromptTokens, s.totalCompletionTokens)
			return resp.GetContent(), nil
		}

		debugAI("assistant requested %d tool call(s)", len(toolCalls))
		for _, call := range toolCalls {
			sanitized := call.GetName()
			def, ok := s.toolRegistry[sanitized]
			var resultContent interface{}
			var callErr error
			stepLabel := sanitized
			stepSummary := ""
			argsLabel := formatToolArgs(call.GetArguments())
			if !ok {
				callErr = fmt.Errorf("tool %q is not registered", sanitized)
				stepSummary = "tool not registered"
				logAI("✖ unable to use %s: tool not registered", sanitized)
			} else {
				stepLabel = def.Name
				if argsLabel != "" {
					logAI("▶ %s %s", def.Name, argsLabel)
				} else {
					logAI("▶ %s", def.Name)
				}
				result, err := aitools.Call(ctx, def.Name, call.GetArguments())
				if err != nil {
					callErr = err
					stepSummary = err.Error()
					logAI("✖ %s → %v", def.Name, err)
				} else {
					resultContent = toolResultPayload(result)
					stepSummary = summarizeToolResult(result)
					logAI("✔ %s → %s", def.Name, truncateForLog(stepSummary, 256))
				}
			}

			if callErr != nil {
				resultContent = map[string]interface{}{
					"error": callErr.Error(),
				}
				errMsg := truncateForLog(stepSummary, 80)
				turnSteps = append(turnSteps, fmt.Sprintf("%s (error: %s)", stepLabel, errMsg))
				lastToolSummary = errMsg
			} else if stepSummary != "" {
				turnSteps = append(turnSteps, summarizeStep(stepLabel, stepSummary))
				lastToolSummary = stepSummary
			} else {
				turnSteps = append(turnSteps, stepLabel)
				lastToolSummary = stepLabel
			}

			toolMsg, err := s.provider.CreateToolResponse(call.GetID(), resultContent)
			if err != nil {
				return "", err
			}
			if cloned := history.CloneToolMessage(toolMsg); cloned != nil {
				s.history = append(s.history, cloned)
			}
			s.prune()
		}
	}

	debugAI("assistant hit tool iteration limit; returning fallback message")
	logTurnSummary(turnSteps, turnPromptTokens, turnCompletionTokens, s.totalPromptTokens, s.totalCompletionTokens)
	if lastToolSummary != "" {
		message := fmt.Sprintf("I ran multiple tools but still couldn't finish. Most recent result: %s", truncateForLog(lastToolSummary, 256))
		return message, nil
	}
	return "I ran multiple tools but still couldn't finish. Please adjust the request or guide me to a different source.", nil
}

func logAI(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "AI ▶ "+format+"\n", a...)
}

func debugAI(format string, a ...interface{}) {
	if !Verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "[AI debug] "+format+"\n", a...)
}

func profileNameOrDefault(name string) string {
	if strings.TrimSpace(name) == "" {
		return "(default)"
	}
	return name
}

type inlineToolCall struct {
	id   string
	name string
	args map[string]interface{}
}

func (c inlineToolCall) GetName() string {
	return c.name
}

func (c inlineToolCall) GetArguments() map[string]interface{} {
	return c.args
}

func (c inlineToolCall) GetID() string {
	return c.id
}

var (
	inlineToolPattern     = regexp.MustCompile(`(?s)<tool_call>(.*?)</tool_call>`)
	inlineToolNamePattern = regexp.MustCompile(`roderik__[a-z0-9_]+`)
	inlineArgPairPattern  = regexp.MustCompile(`(?s)<arg_key>(.*?)</arg_key>\s*<arg_value>(.*?)</arg_value>`)
)

func synthesizeInlineToolCalls(content string) []llm.ToolCall {
	if !strings.Contains(content, "<tool_call>") {
		return nil
	}

	matches := inlineToolPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seenSegments := make(map[string]struct{}, len(matches))
	var calls []llm.ToolCall
	for _, match := range matches {
		segment := match[1]
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if _, ok := seenSegments[segment]; ok {
			continue
		}
		seenSegments[segment] = struct{}{}

		name := inlineToolNamePattern.FindString(segment)
		if name == "" {
			// Fallback to first non-tag token
			lines := strings.Split(segment, "\n")
			for _, line := range lines {
				trim := strings.TrimSpace(line)
				if trim == "" {
					continue
				}
				if strings.HasPrefix(trim, "<") {
					continue
				}
				name = trim
				break
			}
		}
		if name == "" {
			continue
		}

		args := make(map[string]interface{})
		pairs := inlineArgPairPattern.FindAllStringSubmatch(segment, -1)
		for _, pair := range pairs {
			key := strings.TrimSpace(pair[1])
			val := strings.TrimSpace(pair[2])
			if key == "" {
				continue
			}
			args[key] = val
		}

		calls = append(calls, inlineToolCall{
			id:   fmt.Sprintf("inline-%d", len(calls)+1),
			name: name,
			args: args,
		})
	}

	if len(calls) == 0 {
		return nil
	}
	return calls
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
		// still enforce invariants
		s.history = sanitizeHistory(s.history)
		return
	}
	start := len(s.history) - s.historyWindow
	s.history = append([]llm.Message(nil), s.history[start:]...)
	s.history = sanitizeHistory(s.history)
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

	if ts := currentDateTimeSummary(); ts != "" {
		b.WriteString(systemPromptTimeIntro)
		b.WriteString("\n- ")
		b.WriteString(ts)
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
	if summary, isHeading, isLink := focusedElementSummary(); summary != "" {
		lines = append(lines, summary)
		lines = append(lines, "- Focus tip: tools operate on the focused element; adjust focus (parent/child/head/next) or reload before expecting broader context.")
		if isHeading {
			lines = append(lines, "- Selection tip: current focus is a heading; expand to its parent or body before extracting section details.")
		}
		if isLink {
			lines = append(lines, "- Selection tip: current focus is a link; follow it with click or move to its parent to read nearby text.")
		}
	}
	return strings.Join(lines, "\n")
}

func currentDateTimeSummary() string {
	now := time.Now().Format(time.RFC3339)
	return strings.TrimSpace(now)
}

func focusElementDescriptor() (string, bool, bool) {
	if CurrentElement == nil {
		return "", false, false
	}

	props, err := CurrentElement.Describe(0, false)
	if err != nil {
		if Verbose {
			fmt.Fprintf(os.Stderr, "warning: failed to describe current element: %v\n", err)
		}
		return "unable to describe current selection.", false, false
	}
	if props == nil {
		return "", false, false
	}

	tag := strings.ToLower(strings.TrimSpace(props.NodeName))
	if tag == "" {
		tag = "element"
	}

	attrSummary := summarizeKeyAttributes(props.Attributes)

	textSnippet := ""
	if txt, err := CurrentElement.Text(); err == nil {
		txt = strings.Join(strings.Fields(txt), " ")
		textSnippet = truncateContextText(txt, 120)
	} else if Verbose {
		fmt.Fprintf(os.Stderr, "warning: failed to read text for current element: %v\n", err)
	}

	var b strings.Builder
	b.WriteString("<")
	b.WriteString(tag)
	if attrSummary != "" {
		b.WriteByte(' ')
		b.WriteString(attrSummary)
	}
	b.WriteString(">")
	if textSnippet != "" {
		b.WriteString(` text≈"`)
		b.WriteString(textSnippet)
		b.WriteByte('"')
	}

	isHeading := strings.HasPrefix(tag, "h") && len(tag) == 2 && tag[1] >= '1' && tag[1] <= '6'
	isLink := tag == "a"
	return b.String(), isHeading, isLink
}

func focusedElementSummary() (string, bool, bool) {
	desc, isHeading, isLink := focusElementDescriptor()
	if desc == "" {
		return "", isHeading, isLink
	}
	return "- Focused element: " + desc, isHeading, isLink
}

func summarizeKeyAttributes(attrs []string) string {
	if len(attrs) == 0 {
		return ""
	}
	var parts []string
	for i := 0; i+1 < len(attrs); i += 2 {
		key := strings.ToLower(strings.TrimSpace(attrs[i]))
		value := strings.TrimSpace(attrs[i+1])
		if value == "" {
			continue
		}
		switch key {
		case "id", "class", "role", "name", "aria-label":
			parts = append(parts, fmt.Sprintf(`%s="%s"`, key, truncateContextText(value, 60)))
		}
	}
	return strings.Join(parts, " ")
}

func truncateContextText(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 || s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func sanitizeHistory(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return messages
	}
	clean := make([]llm.Message, 0, len(messages))
	var lastToolCallIDs map[string]struct{}

	appendMessage := func(msg llm.Message) {
		if msg == nil {
			return
		}
		role := msg.GetRole()
		if len(clean) == 0 && role != "user" {
			return
		}
		switch role {
		case "tool":
			if len(clean) == 0 {
				return
			}
			if lastToolCallIDs == nil || len(lastToolCallIDs) == 0 {
				return
			}
			if id := msg.GetToolResponseID(); id != "" {
				if _, ok := lastToolCallIDs[id]; !ok {
					return
				}
				// tool response consumed; keep mapping for potential multiple results
			}
		default:
			// recompute tool call ids for subsequent tool messages
			if calls := msg.GetToolCalls(); len(calls) > 0 {
				lastToolCallIDs = make(map[string]struct{}, len(calls))
				for _, call := range calls {
					if call == nil {
						continue
					}
					lastToolCallIDs[call.GetID()] = struct{}{}
				}
			} else {
				lastToolCallIDs = nil
			}
		}
		clean = append(clean, msg)
	}

	for _, msg := range messages {
		appendMessage(msg)
	}
	return clean
}

func logTurnSummary(steps []string, turnPrompt, turnCompletion int, totalPrompt, totalCompletion int64) {
	summary := "no tools used"
	if len(steps) > 0 {
		summary = strings.Join(steps, " → ")
	}

	turnPromptHuman := formatTokensHuman(int64(turnPrompt))
	turnCompletionHuman := formatTokensHuman(int64(turnCompletion))
	totalHuman := formatTokensHuman(totalPrompt + totalCompletion)

	logAI("Summary: %s | tokens this turn %s prompt / %s completion (total %s)",
		summary,
		turnPromptHuman,
		turnCompletionHuman,
		totalHuman,
	)
}

func summarizeStep(name, detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return name
	}
	detail = truncateForLog(detail, 80)
	return fmt.Sprintf("%s → %s", name, detail)
}

func formatTokensHuman(n int64) string {
	if n <= 0 {
		return "0"
	}

	type unit struct {
		value float64
		label string
	}

	thresholds := []unit{
		{1_000_000_000, "B"},
		{1_000_000, "M"},
		{1_000, "k"},
	}

	for _, t := range thresholds {
		if float64(n) >= t.value {
			scaled := float64(n) / t.value
			if scaled >= 100 || scaled == float64(int64(scaled)) {
				return fmt.Sprintf("%d%s", int64(scaled+0.5), t.label)
			}
			return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f%s", scaled, t.label), "0"), ".")
		}
	}

	return strconv.FormatInt(n, 10)
}

func (s *ChatSession) toolsWithFocusHint(hint string) []llm.Tool {
	if hint == "" || len(s.tools) == 0 {
		return s.tools
	}

	modified := false
	out := make([]llm.Tool, len(s.tools))
	copy(out, s.tools)
	for i, tool := range out {
		def, ok := s.toolRegistry[tool.Name]
		if !ok || !def.FocusAware {
			continue
		}
		out[i].Description = fmt.Sprintf("%s\n\nFocus tip: %s", def.Description, hint)
		modified = true
	}

	if !modified {
		return s.tools
	}
	return out
}

func focusAwareHint() string {
	desc, _, _ := focusElementDescriptor()
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return ""
	}
	desc = truncateForLog(desc, 160)
	if strings.HasSuffix(desc, ".") {
		desc = strings.TrimSuffix(desc, ".")
	}
	return fmt.Sprintf("Current focus: %s. Use parent/head/elem or refocus on <body> when you need the full page.", desc)
}

func formatTokenCount(n int) string {
	if n <= 0 {
		return "?"
	}
	return strconv.Itoa(n)
}

func formatToolArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}

	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := args[k]
		text := truncateForLog(fmt.Sprint(v), 80)
		if strings.ContainsAny(text, " \t\n\"") {
			text = fmt.Sprintf("\"%s\"", text)
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, text))
	}

	return strings.Join(parts, " ")
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var urlErr *neturl.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return true
		}
		return isTimeoutError(urlErr.Err)
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
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
