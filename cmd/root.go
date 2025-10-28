package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func GetUserInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprint(os.Stderr, prompt)
	text, _ := reader.ReadString('\n')
	LogUserInput(text)
	return text
}

func AskForConfirmation(prompt string) bool {
	response := GetUserInput(prompt)
	firstChar := strings.ToLower(string(response[0]))
	if firstChar == "y" {
		return true
	}
	return false
}

var (
	ShowNetActivity  bool
	Interactive      bool
	Verbose          bool
	Stealth          bool // Enable stealth mode
	IgnoreCertErrors bool // New flag for ignoring certificate errors

	logFilePath    string
	logFile        *os.File
	logSetupOnce   sync.Once
	logSetupErr    error
	logPipeWriters []*os.File
)

var netActivityEnabled atomic.Bool

var (
	originalStdout = os.Stdout
	originalStderr = os.Stderr
)

func LogUserInput(input string) {
	if logFile == nil {
		return
	}
	trimmed := strings.TrimRight(input, "\r\n")
	if trimmed == "" {
		trimmed = "(empty)"
	}
	timestamp := time.Now().Format(time.RFC3339)
	fmt.Fprintf(logFile, "[INPUT %s] %s\n", timestamp, trimmed)
	_ = logFile.Sync()
}

func ensureLoggingSetup() error {
	logSetupOnce.Do(func() {
		path := strings.TrimSpace(logFilePath)
		if path == "" {
			return
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logSetupErr = fmt.Errorf("open log file %q: %w", path, err)
			return
		}
		logFile = f
		if err := teeStream(&os.Stdout, originalStdout, f); err != nil {
			logSetupErr = fmt.Errorf("redirect stdout: %w", err)
			return
		}
		if err := teeStream(&os.Stderr, originalStderr, f); err != nil {
			logSetupErr = fmt.Errorf("redirect stderr: %w", err)
			return
		}
		log.SetOutput(io.MultiWriter(originalStderr, f))
	})
	return logSetupErr
}

func teeStream(target **os.File, original *os.File, logDest *os.File) error {
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	*target = w
	mw := io.MultiWriter(original, newTimestampWriter(logDest))
	go func() {
		_, _ = io.Copy(mw, r)
	}()
	logPipeWriters = append(logPipeWriters, w)
	return nil
}

type timestampWriter struct {
	dst     io.Writer
	mu      sync.Mutex
	pending []byte
}

func newTimestampWriter(dst io.Writer) io.Writer {
	if dst == nil {
		return dst
	}
	return &timestampWriter{dst: dst}
}

func (w *timestampWriter) Write(p []byte) (int, error) {
	if w == nil {
		return len(p), nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	consumed := 0
	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		if idx == -1 {
			w.pending = append(w.pending, p...)
			consumed += len(p)
			break
		}

		w.pending = append(w.pending, p[:idx]...)
		line := string(w.pending)
		w.pending = w.pending[:0]
		consumed += idx + 1
		p = p[idx+1:]

		timestamp := time.Now().Format(time.RFC3339)
		var out bytes.Buffer
		out.Grow(len(timestamp) + len(line) + 4)
		out.WriteByte('[')
		out.WriteString(timestamp)
		out.WriteString("] ")
		out.WriteString(line)
		out.WriteByte('\n')

		if _, err := w.dst.Write(out.Bytes()); err != nil {
			return consumed, err
		}
	}
	return consumed, nil
}

func isTerminalFile(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func StdoutIsTerminal() bool {
	return isTerminalFile(originalStdout)
}

func StdinIsTerminal() bool {
	return isTerminalFile(os.Stdin)
}

type NetworkResponseInfo struct {
	Status            int
	StatusText        string
	MIMEType          string
	Headers           map[string]string
	EncodedDataLength float64
	FromDiskCache     bool
	FromPrefetchCache bool
	ResponseTimestamp time.Time
}

type NetworkFinishedInfo struct {
	EncodedDataLength float64
	FinishedTimestamp time.Time
}

type NetworkFailureInfo struct {
	ErrorText        string
	Canceled         bool
	ResourceType     proto.NetworkResourceType
	BlockedReason    proto.NetworkBlockedReason
	FailureTimestamp time.Time
}

type NetworkBody struct {
	Data         []byte
	RetrievedAt  time.Time
	FromStream   bool
	OriginalSize int
}

type NetworkLogEntry struct {
	RequestID        string
	ProtoRequestID   proto.NetworkRequestID
	URL              string
	Method           string
	ResourceType     proto.NetworkResourceType
	DocumentURL      string
	FrameID          proto.PageFrameID
	LoaderID         proto.NetworkLoaderID
	InitiatorType    string
	RequestHeaders   map[string]string
	RequestTimestamp time.Time
	Response         *NetworkResponseInfo
	Finished         *NetworkFinishedInfo
	Failure          *NetworkFailureInfo
	Body             *NetworkBody
}

type NetworkEventLog struct {
	mu       sync.Mutex
	messages []string
	entries  map[string]*NetworkLogEntry
	order    []string
}

type NetworkLogFilter struct {
	MIMESubstrings []string
	Suffixes       []string
	StatusCodes    []int
	TextContains   []string
	Methods        []string
	Domains        []string
	ResourceTypes  []proto.NetworkResourceType
}

func newNetworkEventLog() *NetworkEventLog {
	return &NetworkEventLog{
		entries: make(map[string]*NetworkLogEntry),
	}
}

func cloneHeaders(headers proto.NetworkHeaders) map[string]string {
	if headers == nil {
		return nil
	}
	cloned := make(map[string]string, len(headers))
	for k, v := range headers {
		cloned[k] = v.String()
	}
	return cloned
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	cloned := make(map[string]string, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

func (r *NetworkResponseInfo) clone() *NetworkResponseInfo {
	if r == nil {
		return nil
	}
	cpy := *r
	cpy.Headers = cloneStringMap(r.Headers)
	return &cpy
}

func (f *NetworkFinishedInfo) clone() *NetworkFinishedInfo {
	if f == nil {
		return nil
	}
	cpy := *f
	return &cpy
}

func (f *NetworkFailureInfo) clone() *NetworkFailureInfo {
	if f == nil {
		return nil
	}
	cpy := *f
	return &cpy
}

func (b *NetworkBody) clone() *NetworkBody {
	if b == nil {
		return nil
	}
	cpy := *b
	if b.Data != nil {
		cpy.Data = append([]byte(nil), b.Data...)
	}
	return &cpy
}

func (e *NetworkLogEntry) clone() *NetworkLogEntry {
	if e == nil {
		return nil
	}
	cpy := *e
	cpy.RequestHeaders = cloneStringMap(e.RequestHeaders)
	cpy.Response = e.Response.clone()
	cpy.Finished = e.Finished.clone()
	cpy.Failure = e.Failure.clone()
	cpy.Body = e.Body.clone()
	return &cpy
}

func (l *NetworkEventLog) AddMessage(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, msg)
}

func (l *NetworkEventLog) recordEntry(reqID proto.NetworkRequestID) *NetworkLogEntry {
	key := string(reqID)
	entry, ok := l.entries[key]
	if !ok {
		entry = &NetworkLogEntry{RequestID: key, ProtoRequestID: reqID}
		l.entries[key] = entry
		l.order = append(l.order, key)
	}
	if entry.ProtoRequestID == "" {
		entry.ProtoRequestID = reqID
	}
	return entry
}

func (l *NetworkEventLog) RecordRequest(e *proto.NetworkRequestWillBeSent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.recordEntry(e.RequestID)
	entry.URL = e.Request.URL
	entry.Method = e.Request.Method
	entry.ResourceType = e.Type
	entry.DocumentURL = e.DocumentURL
	entry.FrameID = e.FrameID
	entry.LoaderID = e.LoaderID
	if e.Initiator != nil {
		entry.InitiatorType = string(e.Initiator.Type)
	}
	entry.RequestHeaders = cloneHeaders(e.Request.Headers)
	entry.RequestTimestamp = time.Now()
}

func (l *NetworkEventLog) RecordResponse(e *proto.NetworkResponseReceived) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.recordEntry(e.RequestID)
	entry.ResourceType = e.Type
	entry.Response = &NetworkResponseInfo{
		Status:            int(e.Response.Status),
		StatusText:        e.Response.StatusText,
		MIMEType:          e.Response.MIMEType,
		Headers:           cloneHeaders(e.Response.Headers),
		EncodedDataLength: e.Response.EncodedDataLength,
		FromDiskCache:     e.Response.FromDiskCache,
		FromPrefetchCache: e.Response.FromPrefetchCache,
		ResponseTimestamp: time.Now(),
	}
}

func (l *NetworkEventLog) RecordFinished(e *proto.NetworkLoadingFinished) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.recordEntry(e.RequestID)
	entry.Finished = &NetworkFinishedInfo{
		EncodedDataLength: e.EncodedDataLength,
		FinishedTimestamp: time.Now(),
	}
}

func (l *NetworkEventLog) RecordFailure(e *proto.NetworkLoadingFailed) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.recordEntry(e.RequestID)
	entry.ResourceType = e.Type
	entry.Failure = &NetworkFailureInfo{
		ErrorText:        e.ErrorText,
		Canceled:         e.Canceled,
		ResourceType:     e.Type,
		BlockedReason:    e.BlockedReason,
		FailureTimestamp: time.Now(),
	}
}

func (l *NetworkEventLog) StoreBody(reqID proto.NetworkRequestID, data []byte, fromStream bool, originalSize int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.recordEntry(reqID)
	bodyCopy := append([]byte(nil), data...)
	entry.Body = &NetworkBody{
		Data:         bodyCopy,
		RetrievedAt:  time.Now(),
		FromStream:   fromStream,
		OriginalSize: originalSize,
	}
}

func (l *NetworkEventLog) EntryByID(id string) (*NetworkLogEntry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[id]
	if !ok {
		return nil, false
	}
	return entry.clone(), true
}

func (l *NetworkEventLog) Entries() []*NetworkLogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	results := make([]*NetworkLogEntry, 0, len(l.order))
	for _, id := range l.order {
		if entry, ok := l.entries[id]; ok {
			results = append(results, entry.clone())
		}
	}
	return results
}

func (l *NetworkEventLog) Messages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	msgs := make([]string, len(l.messages))
	copy(msgs, l.messages)
	return msgs
}

func (l *NetworkEventLog) FilterEntries(filter NetworkLogFilter) []*NetworkLogEntry {
	entries := l.Entries()
	if isEmptyFilter(filter) {
		return entries
	}
	results := make([]*NetworkLogEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if filter.matches(entry) {
			results = append(results, entry)
		}
	}
	return results
}

func isEmptyFilter(f NetworkLogFilter) bool {
	return len(f.MIMESubstrings) == 0 && len(f.Suffixes) == 0 && len(f.StatusCodes) == 0 &&
		len(f.TextContains) == 0 && len(f.Methods) == 0 && len(f.Domains) == 0 && len(f.ResourceTypes) == 0
}

func normalizeStrings(values []string) []string {
	res := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(strings.ToLower(v))
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

func (f NetworkLogFilter) matches(entry *NetworkLogEntry) bool {
	if len(f.Methods) > 0 {
		method := strings.ToLower(entry.Method)
		match := false
		for _, m := range f.Methods {
			if method == strings.ToLower(m) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	if len(f.ResourceTypes) > 0 {
		match := false
		for _, rt := range f.ResourceTypes {
			if entry.ResourceType == rt {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	if len(f.MIMESubstrings) > 0 {
		if entry.Response == nil {
			return false
		}
		mime := strings.ToLower(entry.Response.MIMEType)
		match := false
		for _, substr := range f.MIMESubstrings {
			if strings.Contains(mime, strings.ToLower(substr)) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	if len(f.StatusCodes) > 0 {
		if entry.Response == nil {
			return false
		}
		match := false
		for _, code := range f.StatusCodes {
			if entry.Response.Status == code {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	if len(f.Domains) > 0 || len(f.Suffixes) > 0 || len(f.TextContains) > 0 {
		u, err := url.Parse(entry.URL)
		var host, pathStr string
		if err == nil {
			host = strings.ToLower(u.Host)
			pathStr = u.Path
		} else {
			pathStr = entry.URL
		}
		if len(f.Domains) > 0 {
			match := false
			for _, d := range f.Domains {
				if strings.Contains(host, strings.ToLower(d)) {
					match = true
					break
				}
			}
			if !match {
				return false
			}
		}
		if len(f.Suffixes) > 0 {
			name := strings.ToLower(path.Base(pathStr))
			match := false
			for _, suf := range f.Suffixes {
				if strings.HasSuffix(name, strings.ToLower(suf)) {
					match = true
					break
				}
			}
			if !match {
				return false
			}
		}
		if len(f.TextContains) > 0 {
			content := strings.ToLower(entry.URL)
			matchAll := true
			for _, txt := range f.TextContains {
				if !strings.Contains(content, strings.ToLower(txt)) {
					matchAll = false
					break
				}
			}
			if !matchAll {
				return false
			}
		}
	}

	return true
}

var (
	pageEventMu   sync.Mutex
	pageEventPage *rod.Page
)

var (
	eventLogMu     sync.RWMutex
	activeEventLog = newNetworkEventLog()
)

var (
	desktopConnector   = connectToWindowsDesktopChrome
	prepareBrowserFunc = PrepareBrowser
)

const defaultDesktopProfileDir = "WSL2"

var (
	profileFlag                 string
	profileTitleFlag            string
	resolvedDesktopProfileDir   = defaultDesktopProfileDir
	resolvedDesktopProfileTitle string
	applyDesktopProfileTitle    bool
	desktopProfileSelectionDone bool
	resolvedLocalProfileDir     string
	pendingRootTarget           string
)

func setActiveEventLog(log *NetworkEventLog) {
	eventLogMu.Lock()
	activeEventLog = log
	eventLogMu.Unlock()
}

func appendEventLog(msg string) {
	if log := getActiveEventLog(); log != nil {
		log.AddMessage(msg)
	}
}

func getActiveEventLog() *NetworkEventLog {
	eventLogMu.RLock()
	defer eventLogMu.RUnlock()
	return activeEventLog
}

func setNetworkActivityEnabled(enabled bool) bool {
	prev := netActivityEnabled.Swap(enabled)
	ShowNetActivity = enabled
	return prev != enabled
}

func syncNetworkActivityFlag() {
	setNetworkActivityEnabled(ShowNetActivity)
}

func isNetworkActivityEnabled() bool {
	return netActivityEnabled.Load()
}

func recordNetworkRequest(e *proto.NetworkRequestWillBeSent) {
	if log := getActiveEventLog(); log != nil {
		log.RecordRequest(e)
	}
}

func recordNetworkResponse(e *proto.NetworkResponseReceived) {
	if log := getActiveEventLog(); log != nil {
		log.RecordResponse(e)
	}
}

func recordNetworkFinished(e *proto.NetworkLoadingFinished) {
	if log := getActiveEventLog(); log != nil {
		log.RecordFinished(e)
	}
}

func recordNetworkFailed(e *proto.NetworkLoadingFailed) {
	if log := getActiveEventLog(); log != nil {
		log.RecordFailure(e)
	}
}

func retrieveNetworkBody(page *rod.Page, entry *NetworkLogEntry) ([]byte, error) {
	if entry == nil {
		return nil, fmt.Errorf("entry is nil")
	}
	if entry.Body != nil {
		return append([]byte(nil), entry.Body.Data...), nil
	}
	if page == nil {
		return nil, fmt.Errorf("no page loaded to retrieve body")
	}
	if entry.ProtoRequestID == "" {
		return nil, fmt.Errorf("entry missing request id")
	}
	res, err := proto.NetworkGetResponseBody{RequestID: entry.ProtoRequestID}.Call(page)
	if err != nil {
		return nil, fmt.Errorf("get response body: %w", err)
	}
	body := []byte(res.Body)
	if res.Base64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(res.Body)
		if err != nil {
			return nil, fmt.Errorf("decode base64 body: %w", err)
		}
		body = decoded
	}
	if log := getActiveEventLog(); log != nil {
		log.StoreBody(entry.ProtoRequestID, body, false, len(body))
		if updated, ok := log.EntryByID(entry.RequestID); ok {
			entry.Body = updated.Body
		} else {
			entry.Body = &NetworkBody{
				Data:         append([]byte(nil), body...),
				RetrievedAt:  time.Now(),
				FromStream:   false,
				OriginalSize: len(body),
			}
		}
	} else {
		entry.Body = &NetworkBody{
			Data:         append([]byte(nil), body...),
			RetrievedAt:  time.Now(),
			FromStream:   false,
			OriginalSize: len(body),
		}
	}
	return append([]byte(nil), body...), nil
}

var Browser *rod.Browser
var Page *rod.Page
var CurrentElement *rod.Element
var Desktop bool // Indicates we have attached to a desktop Chrome instance
var tempUserDataDir string
var browserInitErr error
var desktopWSURL string
var desktopHostUsed string

const defaultNavigationTimeout = 45 * time.Second

var registerPageEvents = func(p *rod.Page) {
	p.EnableDomain(proto.NetworkEnable{})
	go p.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
		msg := fmt.Sprintf("Request sent: %s", e.Request.URL)
		if isNetworkActivityEnabled() {
			fmt.Fprintln(os.Stderr, msg)
		}
		recordNetworkRequest(e)
		appendEventLog(msg)
	})()
	go p.EachEvent(func(e *proto.NetworkResponseReceived) {
		msg := fmt.Sprintf("Response received: %s Status: %d", e.Response.URL, e.Response.Status)
		if isNetworkActivityEnabled() {
			fmt.Fprintln(os.Stderr, msg)
		}
		recordNetworkResponse(e)
		appendEventLog(msg)
	})()
	go p.EachEvent(func(e *proto.NetworkLoadingFinished) {
		recordNetworkFinished(e)
	})()
	go p.EachEvent(func(e *proto.NetworkLoadingFailed) {
		recordNetworkFailed(e)
	})()
	go p.EachEvent(func(e *proto.PageFrameNavigated) {
		fmt.Fprintln(os.Stderr, "Navigated to:", e.Frame.URL)
		if el, err := p.Timeout(5 * time.Second).Element("body"); err == nil {
			CurrentElement = el
			elementList = nil
			currentIndex = 0
		} else if Verbose {
			fmt.Fprintf(os.Stderr, "warning: failed to reset body after navigation: %v\n", err)
		}
	})()
	go p.EachEvent(func(e *proto.PageJavascriptDialogOpening) {
		fmt.Println("Dialog type: ", e.Type, "Dialog message: ", e.Message)
		switch e.Type {
		case "prompt":
			userInput := GetUserInput(e.Message + " ")
			_ = proto.PageHandleJavaScriptDialog{Accept: true, PromptText: userInput}.Call(p)
		case "confirm":
			confirmation := AskForConfirmation(e.Message + " (y/n) ")
			_ = proto.PageHandleJavaScriptDialog{Accept: confirmation, PromptText: ""}.Call(p)
		case "alert":
			fmt.Println(e.Message)
			_ = proto.PageHandleJavaScriptDialog{Accept: true, PromptText: ""}.Call(p)
		}
	})()
	go p.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
		output, err := formatConsoleArgs(p, e.Args)
		if err != nil {
			if Verbose {
				fmt.Fprintf(os.Stderr, "warning: failed to format console args: %v\n", err)
			}
			return
		}
		fmt.Fprintln(os.Stderr, "console:", output)
	})()
}

func formatConsoleArgs(p *rod.Page, args []*proto.RuntimeRemoteObject) (string, error) {
	if p == nil {
		return "", fmt.Errorf("nil page")
	}
	values := make([]interface{}, 0, len(args))
	for _, arg := range args {
		j, err := p.ObjectToJSON(arg)
		if err != nil {
			return "", err
		}
		values = append(values, j.Val())
	}
	return fmt.Sprint(values), nil
}

func ensurePageEventHandlers(p *rod.Page) {
	if p == nil {
		return
	}
	pageEventMu.Lock()
	defer pageEventMu.Unlock()
	if pageEventPage == p {
		return
	}

	registerPageEvents(p)
	pageEventPage = p
}

func newPageForBrowser(b *rod.Browser) (*rod.Page, error) {
	if b == nil {
		return nil, fmt.Errorf("browser is nil")
	}
	if Stealth {
		return stealth.Page(b)
	}
	return b.Page(proto.TargetCreateTarget{URL: "about:blank"})
}

func ensurePageReady() error {
	if Desktop {
		if Browser != nil && Page != nil {
			return nil
		}
		logFn := func(format string, a ...interface{}) {
			if Verbose {
				fmt.Fprintf(os.Stderr, format, a...)
			}
		}
		if _, _, err := desktopConnector(logFn); err != nil {
			return err
		}
		if Page == nil {
			return fmt.Errorf("desktop chrome attached but no page available")
		}
		return nil
	}

	if Browser == nil || Page == nil {
		tmp, err := prepareBrowserFunc()
		if err != nil {
			Browser = nil
			Page = nil
			return err
		}
		Browser = tmp
		page, err := newPageForBrowser(Browser)
		if err != nil {
			Browser = nil
			Page = nil
			return err
		}
		Page = page
	}

	return nil
}

var RootCmd = &cobra.Command{
	Use:   "roderik",
	Short: "A command-line tool for web scraping and automation",
	Long:  `Roderik is a command-line tool that allows you to navigate, inspect, and interact with elements on a webpage. It uses the Go Rod library for web scraping and automation. You can use it to walk through the DOM, get information about elements, and perform actions like clicking or typing.`,
	Args:  cobra.ArbitraryArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		args = extractTargetFromProfileFlag(cmd, args)
		syncNetworkActivityFlag()
		if err := ensureLoggingSetup(); err != nil {
			browserInitErr = err
			Page = nil
			Browser = nil
			return
		}
		if Desktop {
			logFn := func(format string, a ...interface{}) {
				if Verbose {
					fmt.Fprintf(os.Stderr, format, a...)
				}
			}

			if cmd.Name() == "mcp" {
				if err := ensureDesktopProfileSelectionNonInteractive(logFn); err != nil {
					browserInitErr = err
					Page = nil
					Browser = nil
					return
				}
				browserInitErr = nil
				return
			}

			if err := ensureDesktopProfileSelection(logFn); err != nil {
				browserInitErr = err
				Page = nil
				Browser = nil
				return
			}

			if Browser == nil {
				if _, _, err := desktopConnector(logFn); err != nil {
					browserInitErr = err
					Page = nil
					Browser = nil
					return
				}
			}
			browserInitErr = nil
			return
		}

		if Page == nil {
			// Prepare the browser
			tmp, err := prepareBrowserFunc()
			if err != nil {
				browserInitErr = err
				Page = nil
				Browser = nil
				return
			}
			browserInitErr = nil
			Browser = tmp
			page, err := newPageForBrowser(Browser)
			if err != nil {
				browserInitErr = err
				Page = nil
				Browser = nil
				return
			}
			Page = page
		}
		// fmt.Println(Page.MustInfo())
	},
	Run: func(cmd *cobra.Command, args []string) {
		if browserInitErr != nil {
			fmt.Println("Error preparing browser:", browserInitErr)
			return
		}
		if Page == nil {
			fmt.Println("Error preparing browser: browser not initialized")
			return
		}
		args = normalizeRootCmdArgs(args)
		if len(args) == 0 {
			fmt.Println("Error: target URL argument is required")
			return
		}

		if cmd.Flags().Changed("interactive") {
			value, err := cmd.Flags().GetBool("interactive")
			if err == nil {
				Interactive = value
			}
		} else {
			Interactive = isTerminalFile(os.Stdin) && isTerminalFile(originalStdout)
		}

		targetURL := args[0]
		// Load the target URL
		Page, err := LoadURL(targetURL)
		if err != nil {
			fmt.Println("Error loading URL:", err)
			return
		}

		headings, err := queryElementsFunc(Page, "h1, h2, h3, h4, h5, h6")
		if err != nil {
			if Verbose {
				fmt.Println("Error finding headings:", err)
			}
			headings = nil
		}
		// setup navigable heading list
		elementList = headings

		if len(elementList) > 0 {
			currentIndex = 0
			CurrentElement = elementList[currentIndex]
		} else {
			CurrentElement, err = Page.Element("body")
			if err != nil {
				fmt.Println("Page seems to have no body: ", err)
				return
			}
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if tempUserDataDir != "" {
			if err := os.RemoveAll(tempUserDataDir); err != nil && Verbose {
				fmt.Fprintf(os.Stderr, "warning: failed to remove temporary user data dir %s: %v\n", tempUserDataDir, err)
			}
			tempUserDataDir = ""
		}
	},
}

func PrepareBrowser() (*rod.Browser, error) {
	baseUserDataDir, err := ensureUserDataRoot()
	if err != nil {
		return nil, err
	}

	profileDirName, err := sanitizeProfileDirName(profileFlag)
	if err != nil {
		return nil, err
	}

	userDataDir := baseUserDataDir
	if profileDirName != "" {
		userDataDir = filepath.Join(baseUserDataDir, profileDirName)
		if err := os.MkdirAll(userDataDir, 0755); err != nil {
			return nil, err
		}
		resolvedLocalProfileDir = userDataDir
		fmt.Fprintf(os.Stderr, "[profile] using local dir=%s\n", userDataDir)
	} else {
		resolvedLocalProfileDir = userDataDir
	}

	// Get the browser executable path
	path, found := launcher.LookPath()
	if !found {
		return nil, fmt.Errorf("browser executable path not found")
	}

	launchWithProfile := func(profileDir string) (string, error) {
		l := launcher.New().Bin(path).
			Set("disable-web-security").
			Set("disable-setuid-sandbox").
			Set("no-sandbox").
			Set("no-first-run", "true").
			Set("disable-gpu").
			Set("user-data-dir", profileDir)
		if IgnoreCertErrors {
			l.Set("ignore-certificate-errors")
		}
		return l.Headless(true).Launch()
	}

	controlURL, err := launchWithProfile(userDataDir)
	if err != nil && isProfileLockError(err) {
		tempDir, mkErr := os.MkdirTemp(baseUserDataDir, "profile-")
		if mkErr != nil {
			return nil, fmt.Errorf("failed to create temporary user data dir: %w", mkErr)
		}
		tempUserDataDir = tempDir
		if Verbose {
			fmt.Fprintf(os.Stderr, "falling back to temporary user data dir %s due to profile lock\n", tempUserDataDir)
		}
		controlURL, err = launchWithProfile(tempUserDataDir)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}
	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to browser: %w", err)
	}

	return browser, nil
}

func sanitizeProfileDirName(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", nil
	}
	if trimmed == "." || trimmed == ".." {
		return "", fmt.Errorf("profile directory cannot be %q", trimmed)
	}
	if strings.ContainsAny(trimmed, "/\\") {
		return "", fmt.Errorf("profile directory %q may not contain path separators", input)
	}
	if strings.ContainsRune(trimmed, ':') {
		return "", fmt.Errorf("profile directory %q may not contain ':'", input)
	}
	return trimmed, nil
}

func normalizeRootCmdArgs(args []string) []string {
	if len(args) > 0 {
		pendingRootTarget = ""
		return args
	}
	if pendingRootTarget != "" {
		args = []string{pendingRootTarget}
		pendingRootTarget = ""
	}
	return args
}

func extractTargetFromProfileFlag(cmd *cobra.Command, args []string) []string {
	if len(args) > 0 {
		pendingRootTarget = ""
		return args
	}
	candidate := strings.TrimSpace(profileFlag)
	if candidate == "" || !isValidURL(candidate) {
		return args
	}
	if err := cmd.Flags().Set("profile", ""); err != nil && Verbose {
		fmt.Fprintf(os.Stderr, "warning: failed to reset profile flag: %v\n", err)
	}
	profileFlag = ""
	pendingRootTarget = candidate
	return args
}

func isProfileLockError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "ProcessSingleton") || strings.Contains(errStr, "SingletonLock")
}

func isValidURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func LoadURL(targetURL string) (*rod.Page, error) {
	// setup network aktivity logging
	eventLog := newNetworkEventLog()
	setActiveEventLog(eventLog)
	ensurePageEventHandlers(Page)

	navCtx := Page.Timeout(defaultNavigationTimeout)
	defer func() {
		Page = navCtx.CancelTimeout()
	}()

	if err := navCtx.Navigate(targetURL); err != nil {
		if !isNavigationAborted(err) {
			return nil, fmt.Errorf("navigate to %s failed: %w", targetURL, err)
		}
		if info, infoErr := navCtx.Info(); infoErr != nil || info.URL == "" {
			return nil, fmt.Errorf("navigation aborted while loading %s: %w", targetURL, err)
		}
	}

	if err := navCtx.WaitLoad(); err != nil {
		switch {
		case isNavigationAborted(err):
			// ignore transient aborted errors from redirects
		case errors.Is(err, context.DeadlineExceeded):
			if !pageReady(navCtx) {
				return nil, fmt.Errorf("timed out waiting for %s to finish loading: %w", targetURL, err)
			}
		default:
			return nil, err
		}
	}

	return Page, nil
}

func isNavigationAborted(err error) bool {
	return err != nil && strings.Contains(err.Error(), "net::ERR_ABORTED")
}

func pageReady(p *rod.Page) bool {
	if p == nil {
		return false
	}
	res, err := p.Eval(`() => document.readyState`)
	if err != nil || res == nil {
		return false
	}
	state := strings.ToLower(res.Value.Str())
	return state == "complete" || state == "interactive"
}

// PrettyFormat function
func PrettyFormat(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// prettyPrintJson function
func prettyPrintJson(s string) string {
	var i interface{}
	json.Unmarshal([]byte(s), &i)
	b, _ := json.MarshalIndent(i, "", "  ")
	return string(b)
}
func ReportElement(el *rod.Element) {
	tag, err := el.Eval("() => this.tagName")
	if err != nil {
		fmt.Println("Error getting tag name:", err)
		return
	}
	tagName := tag.Value.Str()
	children, err := el.Elements("*")
	if err != nil {
		fmt.Println("Error getting children:", err)
		return
	}
	childrenCount := len(children)
	text, err := el.Text()
	if err != nil {
		fmt.Println("Error getting text:", err)
		return
	}

	// Limit the text to maxChars characters
	limitedText := fmt.Sprintf("%.50s", text)

	fmt.Printf("%s, %d children, %s\n", tagName, childrenCount, limitedText)
}

func Box(el *rod.Element) error {
	shape, err := el.Shape()
	if err != nil {
		return err
	}
	box := shape.Box()
	fmt.Println("box: ", PrettyFormat(box))
	return nil
}

var ClearCmd = &cobra.Command{
	Use:     "clear",
	Aliases: []string{"cls"},
	Short:   "Clear the terminal screen",
	Long:    `This command will clear the terminal screen.`,
	Run: func(cmd *cobra.Command, args []string) {
		if runtime.GOOS == "windows" {
			cmd := exec.Command("cmd", "/c", "cls")
			cmd.Stdout = os.Stdout
			cmd.Run()
		} else {
			cmd := exec.Command("clear")
			cmd.Stdout = os.Stdout
			cmd.Run()
		}
	},
}

var ReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload the current page",
	Long:  `This command will reload the current page.`,
	Run: func(cmd *cobra.Command, args []string) {
		info, err := Page.Info()
		if err != nil {
			fmt.Println("Error retrieving current page info:", err)
			return
		}
		currentURL := info.URL
		_, err = LoadURL(currentURL)
		if err != nil {
			fmt.Println("Error reloading URL:", err)
			return
		}
		fmt.Println("Page reloaded successfully.")
	},
}

var ExitCmd = &cobra.Command{
	Use:     "exit",
	Aliases: []string{"q", "Q", "bye"},
	Short:   "Exit the application",
	Long:    `This command will exit the application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Goodbye!")
		if Browser != nil {
			if err := Browser.Close(); err != nil && Verbose {
				fmt.Fprintf(os.Stderr, "warning: failed to close browser: %v\n", err)
			}
		}
		Browser = nil
		Page = nil
		CurrentElement = nil
		os.Exit(0)
	},
}

func findChromeOnWindows() (string, error) {
	// query the Windows registry for the Chrome path
	// reg.exe is automatically on PATH under WSL2
	regCmd := exec.Command(
		"reg.exe", "query",
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`,
		"/ve",
	)
	out, err := regCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(
			"failed to query registry: %w (output=%q)",
			err, out,
		)
	}

	// Look for the line that contains "REG_SZ" and split it there.
	// reg query output looks like:
	//    (Default)    REG_SZ    C:\Program Files\...\chrome.exe
	var winPath string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "REG_SZ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				// Join everything after the first two tokens to preserve spaces in path
				winPath = strings.Join(parts[2:], " ")
			}
			break
		}
	}
	if winPath == "" {
		return "", fmt.Errorf("could not find Chrome path in registry output: %q", out)
	}

	// If this binary is itself running on Windows, assume winPath is valid
	if runtime.GOOS == "windows" {
		return winPath, nil
	}
	// Under WSL/Linux, convert Windows path to WSL path and verify it exists
	wslCmd := exec.Command("wslpath", "-u", winPath)
	wslOut, err := wslCmd.Output()
	if err != nil {
		return "", fmt.Errorf("wslpath conversion failed: %w", err)
	}
	linuxPath := strings.TrimSpace(string(wslOut))
	if linuxPath == "" {
		return "", fmt.Errorf("empty path after wslpath conversion")
	}
	if _, err := os.Stat(linuxPath); err != nil {
		return "", fmt.Errorf("chrome.exe not found at %s: %w", linuxPath, err)
	}
	// return the *Windows* path so `cmd.exe start` can launch it
	return winPath, nil
}

func connectToWindowsDesktopChrome(logf func(string, ...interface{})) (string, string, error) {
	if logf == nil {
		logf = func(string, ...interface{}) {}
	}

	if Browser != nil && Desktop {
		logf("reusing existing desktop Chrome session\n")
		return desktopWSURL, desktopHostUsed, nil
	}

	isWSL := false
	if runtime.GOOS != "windows" {
		if data, err := os.ReadFile("/proc/version"); err == nil {
			if bytes.Contains(data, []byte("Microsoft")) {
				isWSL = true
			}
		}
	}

	hosts := []string{"127.0.0.1", "localhost"}
	if isWSL {
		resolv, err := os.ReadFile("/etc/resolv.conf")
		if err != nil {
			return "", "", fmt.Errorf("cannot read resolv.conf: %w", err)
		}
		var hostIP string
		for _, line := range strings.Split(string(resolv), "\n") {
			if strings.HasPrefix(line, "nameserver") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					hostIP = parts[1]
				}
				break
			}
		}
		if hostIP == "" {
			return "", "", fmt.Errorf("could not determine host IP from /etc/resolv.conf")
		}
		logf("using host IP = %s\n", hostIP)
		hosts = append([]string{hostIP}, hosts...)
	}

	winChrome, err := findChromeOnWindows()
	if err != nil {
		return "", "", err
	}

	userDataRoot, err := resolveWindowsUserDataRoot(logf)
	if err != nil {
		return "", "", err
	}
	profileDirName := resolvedDesktopProfileDir
	if profileDirName == "" {
		profileDirName = defaultDesktopProfileDir
	}
	userDataDir := userDataRoot + `\` + profileDirName
	logf("using Windows user-data-dir = %s\n", userDataDir)

	if applyDesktopProfileTitle && resolvedDesktopProfileTitle != "" {
		if err := ensureDesktopProfileTitle(logf, userDataRoot, profileDirName, resolvedDesktopProfileTitle); err != nil {
			logf("warning: unable to set profile title: %v\n", err)
		}
	}

	args0 := []string{
		"/C", "start", "",
		winChrome,
		"--remote-debugging-port=9222",
		"--remote-debugging-address=0.0.0.0",
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
	}
	logf("spawning cmd.exe %v\n", args0)
	cmd0 := exec.Command("cmd.exe", args0...)
	if err := cmd0.Start(); err != nil {
		return "", "", fmt.Errorf("failed to start Windows Chrome: %w", err)
	}

	var (
		wsURL    string
		hostUsed string
	)
	const (
		maxAttempts = 20
		pause       = 300 * time.Millisecond
	)
	for _, h := range hosts {
		for i := 0; i < maxAttempts; i++ {
			time.Sleep(pause)
			u := fmt.Sprintf("http://%s:9222/json/version", h)
			logf("  polling %s (try %d/%d)\n", u, i+1, maxAttempts)
			resp, err := http.Get(u)
			if err != nil {
				logf("    GET error: %v\n", err)
				continue
			}
			if resp.StatusCode != http.StatusOK {
				logf("    HTTP status: %d\n", resp.StatusCode)
				resp.Body.Close()
				continue
			}
			body, _ := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			var info map[string]interface{}
			if err := json.Unmarshal(body, &info); err != nil {
				logf("    JSON error: %v\n", err)
				continue
			}
			if s, ok := info["webSocketDebuggerUrl"].(string); ok {
				wsURL = s
				hostUsed = h
				break
			}
		}
		if wsURL != "" {
			break
		}
		logf("no debugger URL on host %s, trying next\n", h)
	}
	if wsURL == "" {
		return "", "", fmt.Errorf("could not get WebSocket URL – is Chrome running with --remote-debugging?")
	}
	logf("raw wsURL = %s\n", wsURL)

	if strings.Contains(wsURL, "0.0.0.0") && hostUsed != "" {
		wsURL = strings.Replace(wsURL, "0.0.0.0", hostUsed, 1)
		logf("rewrote wsURL to %s\n", wsURL)
	}

	logf("connecting to WebSocket URL: %s\n", wsURL)
	browser := rod.New().ControlURL(wsURL)
	if err := browser.Connect(); err != nil {
		return "", "", fmt.Errorf("failed to connect to Chrome DevTools at %s: %w", wsURL, err)
	}
	Browser = browser.Timeout(30 * time.Second)
	pages, err := Browser.Pages()
	if err != nil {
		return "", "", fmt.Errorf("failed to list Chrome pages: %w", err)
	}
	if len(pages) == 0 {
		newPage, err := Browser.Page(proto.TargetCreateTarget{URL: ""})
		if err != nil {
			return "", "", fmt.Errorf("failed to open new Chrome page: %w", err)
		}
		Page = newPage.Context(context.Background())
	} else {
		Page = pages[0].Context(context.Background())
		if _, err := Page.Activate(); err != nil && Verbose {
			fmt.Fprintf(os.Stderr, "warning: failed to activate first page: %v\n", err)
		}
	}
	if body, err := Page.Timeout(5 * time.Second).Element("body"); err == nil {
		CurrentElement = body
	}
	Desktop = true
	desktopWSURL = wsURL
	desktopHostUsed = hostUsed

	return wsURL, hostUsed, nil
}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&ShowNetActivity, "net-activity", "n", false, "Enable display of network events")
	RootCmd.PersistentFlags().BoolVarP(&Interactive, "interactive", "i", false, "Enable interactive mode")
	RootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Enable verbose mode")
	RootCmd.PersistentFlags().BoolVarP(&IgnoreCertErrors, "ignore-cert-errors", "k", false, "Ignore certificate errors") // Register the new flag
	RootCmd.PersistentFlags().BoolVarP(&Stealth, "stealth", "s", false, "Enable stealth mode")
	RootCmd.PersistentFlags().BoolVarP(&Desktop, "desktop", "d", false, "Attach to Windows desktop Chrome (WSL2 only)")
	RootCmd.PersistentFlags().StringVarP(&logFilePath, "logfile", "l", "", "Append stdout, stderr, and user input to the specified log file")
	RootCmd.PersistentFlags().StringVar(&profileFlag, "profile", "", "Chrome profile directory to use (omit to pick interactively)")
	RootCmd.PersistentFlags().StringVar(&profileTitleFlag, "profile-title", "", "Override the friendly window title for the selected profile")
	if pf := RootCmd.PersistentFlags().Lookup("profile"); pf != nil {
		pf.NoOptDefVal = ""
	}

	RootCmd.AddCommand(ClearCmd)
	RootCmd.AddCommand(ExitCmd)
	RootCmd.AddCommand(ReloadCmd)
}
