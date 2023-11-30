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
)

func GetUserInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
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
			Page = Browser.MustPage("about:blank")
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

		headings, _ := Page.Elements("h1, h2, h3, h4, h5, h6")
		if len(headings) > 0 {
			CurrentElement = headings[0]
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
			fmt.Printf(msg)
		}
		eventLog.Add(msg)
	})()
	go Page.EachEvent(func(e *proto.NetworkResponseReceived) {
		msg := fmt.Sprintf("Response received: %s Status: %d", e.Response.URL, e.Response.Status)
		if ShowNetActivity {
			fmt.Printf(msg)
		}
		eventLog.Add(msg)
	})()
	// setup event listener for navigate events
	go Page.EachEvent(func(e *proto.PageFrameNavigated) {
		fmt.Println("Navigated to: ", e.Frame.URL)
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
			fmt.Println("console: ", Page.MustObjectsToJSON(e.Args))
		})()
	}

	err := Page.Navigate(targetURL)
	if err != nil {
		return nil, err
	}
	Page.WaitLoad()
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

func init() {
	RootCmd.PersistentFlags().BoolVarP(&ShowNetActivity, "net-activity", "n", false, "Enable display of network events")
	RootCmd.PersistentFlags().BoolVarP(&Interactive, "interactive", "i", false, "Enable interactive mode")
	RootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Enable verbose mode")
	RootCmd.PersistentFlags().BoolVarP(&IgnoreCertErrors, "ignore-cert-errors", "k", false, "Ignore certificate errors") // Register the new flag

	RootCmd.AddCommand(ClearCmd)
	RootCmd.AddCommand(ExitCmd)
	RootCmd.AddCommand(ReloadCmd)
}
