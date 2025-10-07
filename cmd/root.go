package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"bytes"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"time"
)

func GetUserInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprint(os.Stderr, prompt)
	text, _ := reader.ReadString('\n')
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

var ShowNetActivity bool
var Interactive bool
var Verbose bool
var Stealth bool          // Enable stealth mode
var IgnoreCertErrors bool // New flag for ignoring certificate errors

type EventLog struct {
	mu   sync.Mutex
	logs []string
}

func (l *EventLog) Add(log string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, log)
}

func (l *EventLog) Display() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, log := range l.logs {
		fmt.Println(log)
	}
}

var Browser *rod.Browser
var Page *rod.Page
var CurrentElement *rod.Element
var Desktop bool // Indicates we have attached to a desktop Chrome instance
var tempUserDataDir string
var browserInitErr error
var desktopWSURL string
var desktopHostUsed string

var RootCmd = &cobra.Command{
	Use:   "roderik",
	Short: "A command-line tool for web scraping and automation",
	Long:  `Roderik is a command-line tool that allows you to navigate, inspect, and interact with elements on a webpage. It uses the Go Rod library for web scraping and automation. You can use it to walk through the DOM, get information about elements, and perform actions like clicking or typing.`,
	Args:  cobra.MinimumNArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cmd.Name() == "win-chrome" {
			return
		}

		if Desktop {
			if Browser == nil {
				logFn := func(format string, a ...interface{}) {
					if Verbose {
						fmt.Fprintf(os.Stderr, format, a...)
					}
				}
				if _, _, err := connectToWindowsDesktopChrome(logFn); err != nil {
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
			tmp, err := PrepareBrowser()
			if err != nil {
				browserInitErr = err
				Page = nil
				Browser = nil
				return
			}
			browserInitErr = nil
			Browser = tmp
			if Stealth {
				Page = stealth.MustPage(Browser)
			} else {
				Page = Browser.MustPage("about:blank")
			}
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
		// set interactive mode for this root command by default
		Interactive = true

		targetURL := args[0]
		// Load the target URL
		Page, err := LoadURL(targetURL)
		if err != nil {
			fmt.Println("Error loading URL:", err)
			return
		}

		headings, err := queryElements(Page, "h1, h2, h3, h4, h5, h6")
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
	// Ensure user data directory exists
	userDataDir := filepath.Join(".", "user_data")
	if _, err := os.Stat(userDataDir); os.IsNotExist(err) {
		err = os.Mkdir(userDataDir, 0755)
		if err != nil {
			return nil, err
		}
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
		tempDir, mkErr := os.MkdirTemp(userDataDir, "profile-")
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
	browser := rod.New().ControlURL(controlURL).MustConnect()

	return browser, nil
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
	eventLog := &EventLog{}

	Page.EnableDomain(proto.NetworkEnable{})
	go Page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
		msg := fmt.Sprintf("Request sent: %s", e.Request.URL)
		if ShowNetActivity {
			fmt.Fprintln(os.Stderr, msg)
		}
		eventLog.Add(msg)
	})()
	go Page.EachEvent(func(e *proto.NetworkResponseReceived) {
		msg := fmt.Sprintf("Response received: %s Status: %d", e.Response.URL, e.Response.Status)
		if ShowNetActivity {
			fmt.Fprintln(os.Stderr, msg)
		}
		eventLog.Add(msg)
	})()
	// setup event listener for navigate events
	go Page.EachEvent(func(e *proto.PageFrameNavigated) {
		fmt.Fprintln(os.Stderr, "Navigated to:", e.Frame.URL)
		if el, err := Page.Timeout(5 * time.Second).Element("body"); err == nil {
			CurrentElement = el
			elementList = nil
			currentIndex = 0
		} else if Verbose {
			fmt.Fprintf(os.Stderr, "warning: failed to reset body after navigation: %v\n", err)
		}
	})()
	// setup event listener for dialogs
	go Page.EachEvent(func(e *proto.PageJavascriptDialogOpening) {
		fmt.Println("Dialog type: ", e.Type, "Dialog message: ", e.Message)
		switch e.Type {
		case "prompt":
			userInput := GetUserInput(e.Message + " ")
			_ = proto.PageHandleJavaScriptDialog{Accept: true, PromptText: userInput}.Call(Page)
		case "confirm":
			confirmation := AskForConfirmation(e.Message + " (y/n) ")
			_ = proto.PageHandleJavaScriptDialog{Accept: confirmation, PromptText: ""}.Call(Page)
		case "alert":
			fmt.Println(e.Message)
			_ = proto.PageHandleJavaScriptDialog{Accept: true, PromptText: ""}.Call(Page)
		}
	})()

	// Listen for all events of console output.
	if true {
		go Page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
			fmt.Fprintln(os.Stderr, "console:", Page.MustObjectsToJSON(e.Args))
		})()
	}

	if err := Page.Navigate(targetURL); err != nil {
		return nil, err
	}
	Page.MustWaitLoad()
	return Page, nil
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
	tagName := el.MustEval("() => this.tagName").String()
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
		currentURL := Page.MustInfo().URL
		_, err := LoadURL(currentURL)
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

	profCmd := exec.Command("cmd.exe", "/C", "echo", "%USERPROFILE%")
	profOut, err := profCmd.Output()
	if err != nil {
		logf("warning: failed to resolve %%USERPROFILE%%: %v\n", err)
	}
	winProfile := strings.TrimSpace(string(profOut))
	if winProfile == "" {
		logf("warning: USERPROFILE expanded to empty, using default data-dir\n")
	}
	userDataDir := winProfile + `\AppData\Local\Google\Chrome\User Data\WSL2`
	logf("using Windows user-data-dir = %s\n", userDataDir)

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
		Page = Browser.MustPage("").Context(context.Background())
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

var WinChromeCmd = &cobra.Command{
	Use:   "win-chrome",
	Short: "Launch and attach to Windows Chrome from WSL2",
	Long:  `Launches Chrome on Windows via WSL2, connects to it, and navigates to https://traumwind.de.`,
	Run: func(cmd *cobra.Command, args []string) {
		logFn := func(format string, a ...interface{}) {
			fmt.Printf(format, a...)
		}
		fmt.Fprintln(os.Stderr, "[deprecated] win-chrome will be removed; prefer --desktop")
		wsURL, hostUsed, err := connectToWindowsDesktopChrome(logFn)
		if err != nil {
			fmt.Println("Could not attach to Windows Chrome:", err)
			return
		}
		targetURL := "https://traumwind.de"
		if len(args) > 0 {
			targetURL = args[0]
		}
		var page *rod.Page
		page, err = LoadURL(targetURL)
		if err != nil {
			fmt.Println("Error loading URL:", err)
			return
		}
		Page = page
		Interactive = true
		fmt.Printf("Desktop session ready via host %s (ws=%s)\n", hostUsed, wsURL)
	},
}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&ShowNetActivity, "net-activity", "n", false, "Enable display of network events")
	RootCmd.PersistentFlags().BoolVarP(&Interactive, "interactive", "i", false, "Enable interactive mode")
	RootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Enable verbose mode")
	RootCmd.PersistentFlags().BoolVarP(&IgnoreCertErrors, "ignore-cert-errors", "k", false, "Ignore certificate errors") // Register the new flag
	RootCmd.PersistentFlags().BoolVarP(&Stealth, "stealth", "s", false, "Enable stealth mode")
	RootCmd.PersistentFlags().BoolVarP(&Desktop, "desktop", "d", false, "Attach to Windows desktop Chrome (WSL2 only)")

	RootCmd.AddCommand(ClearCmd)
	RootCmd.AddCommand(ExitCmd)
	RootCmd.AddCommand(ReloadCmd)
	RootCmd.AddCommand(WinChromeCmd)
}
