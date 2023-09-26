package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var A11yCmd = &cobra.Command{
	Use:   "a11y",
	Short: "Access the accessibility tree of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			url := args[0]
			if isValidURL(url) {
				Page, err := LoadURL(url)
				if err != nil {
					fmt.Println("Error loading URL:", err)
					return
				}
				CurrentElement = Page.MustElement("body")
			}
		}
		if !hasCurrentElement() {
			return
		}
		ReportElement(CurrentElement)
	},
}

func init() {
	RootCmd.AddCommand(A11yCmd)
}
