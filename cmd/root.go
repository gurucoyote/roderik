package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
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
