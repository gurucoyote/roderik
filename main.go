package main

import (
	"fmt"
	"encoding/json"
	"os"
	"path/filepath"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/spf13/cobra"
)

var page *rod.Page
var currentElement *rod.Element

var nextCmd = &cobra.Command{
	Use:   "next [selector]",
	Short: "Navigate to the next element",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement the logic for next command
	},
}

var prevCmd = &cobra.Command{
	Use:   "prev [selector]",
	Short: "Navigate to the previous element",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement the logic for prev command
	},
}

var rootCmd = &cobra.Command{
	Use:   "roderik",
	Short: "A brief description of your application",
	Long:  `A longer description that spans multiple lines and likely contains examples and usage of using your application.`,
	Args:  cobra.MinimumNArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		targetURL := args[0]
		fmt.Println("Target URL:", targetURL)

		// Prepare the browser and load the target URL
		page = prepareBrowserAndLoadURL(targetURL)
		fmt.Println("Connected to browser at URL:", page.MustInfo().URL)
	},
	Run: func(cmd *cobra.Command, args []string) {
		info := page.MustInfo()
		fmt.Println("Opened URL:", info.URL, info.Title)

		// Report on the headings
		headings := page.MustElements("h1, h2, h3, h4, h5, h6")
		if len(headings) > 0 {
			currentElement = headings[0]
		}

		reportOnHeadings(page)
	},
}

func init() {
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(prevCmd)
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

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
func prepareBrowserAndLoadURL(targetURL string) *rod.Page {
	// Ensure user data directory exists
	userDataDir := filepath.Join(".", "user_data")
	if _, err := os.Stat(userDataDir); os.IsNotExist(err) {
		os.Mkdir(userDataDir, 0755)
	}

	// Get the browser executable path
	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).
		Set("disable-web-security").
		Set("disable-setuid-sandbox").
		Set("no-sandbox").
		Set("no-first-run", "true").
		Set("disable-gpu").
		Set("user-data-dir", userDataDir).
		Headless(true).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()

	return browser.MustPage(targetURL).MustWaitLoad()
}

func reportOnHeadings(page *rod.Page) {
	// Get all headings
	headings := page.MustElements("h1, h2, h3, h4, h5, h6")

	// Print the count of headings
	fmt.Println("Count of headings:", len(headings))

	// Print each heading
	for _, heading := range headings {
		// Get the heading level
		level := heading.MustEval("() => this.tagName").String() // [1]
		// fmt.Println(level)

		// Get the heading text
		text := heading.MustText()

		fmt.Printf("%s: %s\n", level, text)
	}

	// Output the description and font-family of the first heading element
	if len(headings) > 0 {
		firstHeading := headings[0]
		fontFamily := firstHeading.MustEval(`() => getComputedStyle(this).fontFamily`).String()
		fmt.Println("Font Family of the first heading:", fontFamily)
		computedStyles := firstHeading.MustEval(`() => getComputedStyle(this)`)
		fmt.Println("computed styles", PrettyFormat(computedStyles))
		// description := firstHeading.MustDescribe()
		// fmt.Println("Description: ", PrettyFormat(description))
	}
}
