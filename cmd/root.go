package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"encoding/json"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/spf13/cobra"
)

var Page *rod.Page
var CurrentElement *rod.Element

var RootCmd = &cobra.Command{
	Use:   "roderik",
	Short: "A brief description of your application",
	Long:  `A longer description that spans multiple lines and likely contains examples and usage of using your application.`,
	Args:  cobra.MinimumNArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
	},
	Run: func(cmd *cobra.Command, args []string) {

		targetURL := args[0]
		fmt.Println("Target URL:", targetURL)

		// Prepare the browser and load the target URL
		Page = prepareBrowserAndLoadURL(targetURL)
		info := Page.MustInfo()
		fmt.Println("Opened URL:", info.URL, info.Title)
		headings := Page.MustElements("h1, h2, h3, h4, h5, h6")
		if len(headings) > 0 {
			CurrentElement = headings[0]
		}
		// Report on the headings
		reportOnHeadings(Page)
	},
}

func init() {
	RootCmd.AddCommand(NextCmd)
	RootCmd.AddCommand(PrevCmd)
	RootCmd.AddCommand(WalkCmd)
	RootCmd.AddCommand(ParentCmd)
	RootCmd.AddCommand(ChildCmd)
	RootCmd.AddCommand(ShapeCmd)
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

func reportOnHeadings(Page *rod.Page) {
	// Get all headings
	headings := Page.MustElements("h1, h2, h3, h4, h5, h6")
	// Print the count of headings
	fmt.Println("Count of headings:", len(headings))

	// Output the description and font-family of the first heading element
	if len(headings) > 0 {
		firstHeading := headings[0]
		fontFamily := firstHeading.MustEval(`() => getComputedStyle(this).fontFamily`).String()
		fmt.Println("Font Family of the first heading:", fontFamily)
		CurrentElement = firstHeading 
		/*
		computedStyles := firstHeading.MustEval(`() => getComputedStyle(this)`)
		fmt.Println("computed styles", PrettyFormat(computedStyles))
		// description := firstHeading.MustDescribe()
		// fmt.Println("Description: ", PrettyFormat(description))
		*/
	}
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
      limitedText := fmt.Sprintf("%.100s", text)

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
