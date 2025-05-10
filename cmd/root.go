package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/spf13/cobra"
	"github.com/go-rod/stealth"
	"bytes"
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
var Stealth bool // Enable stealth mode
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

var Browser rod.Browser
var Page *rod.Page
var CurrentElement *rod.Element

var RootCmd = &cobra.Command{
	Use:   "roderik",
	Short: "A command-line tool for web scraping and automation",
	Long:  `Roderik is a command-line tool that allows you to navigate, inspect, and interact with elements on a webpage. It uses the Go Rod library for web scraping and automation. You can use it to walk through the DOM, get information about elements, and perform actions like clicking or typing.`,
	Args:  cobra.MinimumNArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if Page == nil {
			// Prepare the browser
			Browser, err := PrepareBrowser()
			if err != nil {
				fmt.Println("Error preparing browser:", err)
				return
			}
			if Stealth {
				Page = stealth.MustPage(Browser)
			} else {
				Page = Browser.MustPage("about:blank")
			}
		}
		// fmt.Println(Page.MustInfo())
	},
	Run: func(cmd *cobra.Command, args []string) {
		// set interactive mode for this root command by default
		Interactive = true

		targetURL := args[0]
		// Load the target URL
		Page, err := LoadURL(targetURL)
		if err != nil {
			fmt.Println("Error loading URL:", err)
			return
		}

		// headings, _ := Page.Elements("h1, h2, h3, h4, h5, h6")
		var headings []*rod.Element
		for _, tag := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
			elems, _ := Page.Elements(tag)
			headings = append(headings, elems...)
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
		// This function will always be run after any command (including sub-commands) is executed
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
	u := launcher.New().Bin(path).
		Set("disable-web-security").
		Set("disable-setuid-sandbox").
		Set("no-sandbox").
		Set("no-first-run", "true").
		Set("disable-gpu").
		Set("user-data-dir", userDataDir)
	if IgnoreCertErrors {
		u.Set("ignore-certificate-errors") // Set the flag to ignore certificate errors if specified
	}
	controlURL := u.Headless(true).MustLaunch()
	browser := rod.New().ControlURL(controlURL).MustConnect()

	return browser, nil
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
		CurrentElement = Page.MustElement("body")
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

	err := Page.Navigate(targetURL)
	if err != nil {
		return nil, err
	}
	// wait for full load and idle so injected helpers are ready
	Page.MustWaitLoad().MustWaitIdle()
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
	childrenCount := len(el.MustElements("*"))
	text := el.MustText()

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

	// convert to WSL path *only* to check that the binary really exists
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

var WinChromeCmd = &cobra.Command{
	Use:   "win-chrome",
	Short: "Launch and attach to Windows Chrome from WSL2",
	Long:  `Launches Chrome on Windows via WSL2, connects to it, and navigates to https://traumwind.de.`,
	Run: func(cmd *cobra.Command, args []string) {
		// 1) detect WSL2 vs native Windows and prepare hosts list
		isWSL := false
		if runtime.GOOS != "windows" {
			if data, err := os.ReadFile("/proc/version"); err == nil {
				if bytes.Contains(data, []byte("Microsoft")) {
					isWSL = true
				}
			}
		}
		var hosts []string
		if isWSL {
			// WSL2: derive host IP from nameserver
			resolv, err := os.ReadFile("/etc/resolv.conf")
			if err != nil {
				fmt.Println("cannot read resolv.conf:", err)
				return
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
				fmt.Println("could not determine host IP")
				return
			}
			fmt.Println("using host IP =", hostIP)
			hosts = []string{hostIP, "127.0.0.1", "localhost"}
		} else {
			// native Windows: use loopback only
			hosts = []string{"127.0.0.1"}
		}

		// 2) launch Windows Chrome
		winChrome, err := findChromeOnWindows()
		if err != nil {
			fmt.Println("Could not locate Windows Chrome:", err)
			return
		}
		// 2a) find the real Windows user profile so Chrome can read/write the data dir
		profCmd := exec.Command("cmd.exe", "/C", "echo", "%USERPROFILE%")
		profOut, err := profCmd.Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, "warning: failed to resolve %USERPROFILE%:", err)
		}
		winProfile := strings.TrimSpace(string(profOut))
		if winProfile == "" {
			fmt.Fprintln(os.Stderr, "warning: %USERPROFILE% expanded to empty, using default data-dir")
		}
		userDataDir := winProfile + `\AppData\Local\Google\Chrome\User Data\WSL2`
		fmt.Println("using Windows user-data-dir =", userDataDir)
		args0 := []string{
			"/C", "start", "",
			winChrome,
			"--remote-debugging-port=9222",
			"--remote-debugging-address=0.0.0.0",
			"--user-data-dir=" + userDataDir,
			"--no-first-run",
			"--no-default-browser-check",
		}
		fmt.Println("spawning cmd.exe", args0)
		cmd0 := exec.Command("cmd.exe", args0...)
		if err := cmd0.Start(); err != nil {
			fmt.Println("failed to start Windows Chrome:", err)
			return
		}

		// 3) wait & poll /json/version on multiple possible hosts
		var wsURL, hostUsed string
		const (
			maxAttempts = 20
			pause       = 300 * time.Millisecond
		)
		// hosts list initialized above
		for _, h := range hosts {
			for i := 0; i < maxAttempts; i++ {
				time.Sleep(pause)
				u := fmt.Sprintf("http://%s:9222/json/version", h)
				fmt.Printf("  polling %s (try %d/%d)\n", u, i+1, maxAttempts)
				resp, err := http.Get(u)
				if err != nil {
					fmt.Printf("    GET error: %v\n", err)
					continue
				}
				if resp.StatusCode != http.StatusOK {
					fmt.Printf("    HTTP status: %d\n", resp.StatusCode)
					resp.Body.Close()
					continue
				}
				body, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				var info map[string]interface{}
				if err := json.Unmarshal(body, &info); err != nil {
					fmt.Printf("    JSON error: %v\n", err)
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
			fmt.Printf("no debugger URL on host %s, trying next\n", h)
		}
		if wsURL == "" {
			fmt.Println("Could not get WebSocket URL – is Chrome running with --remote-debugging?")
			return
		}
		fmt.Println("raw wsURL =", wsURL)

		// 4) rewrite any 0.0.0.0 in the WS URL to the actual host we used
		if strings.Contains(wsURL, "0.0.0.0") && hostUsed != "" {
			wsURL = strings.Replace(wsURL, "0.0.0.0", hostUsed, 1)
			fmt.Println("rewrote wsURL to", wsURL)
		}

		// 5) before dialing, show the exact WebSocket URL
		fmt.Println("connecting to WebSocket URL:", wsURL)

		// 5) finally connect
		browser := rod.New().
			ControlURL(wsURL).
			Timeout(5 * time.Second).
			MustConnect()

		page := browser.MustPage("about:blank")
		page.MustNavigate("https://traumwind.de")
	},
}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&ShowNetActivity, "net-activity", "n", false, "Enable display of network events")
	RootCmd.PersistentFlags().BoolVarP(&Interactive, "interactive", "i", false, "Enable interactive mode")
	RootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Enable verbose mode")
	RootCmd.PersistentFlags().BoolVarP(&IgnoreCertErrors, "ignore-cert-errors", "k", false, "Ignore certificate errors") // Register the new flag
	RootCmd.PersistentFlags().BoolVarP(&Stealth, "stealth", "s", false, "Enable stealth mode")

	RootCmd.AddCommand(ClearCmd)
	RootCmd.AddCommand(ExitCmd)
	RootCmd.AddCommand(ReloadCmd)
	RootCmd.AddCommand(WinChromeCmd)
}
