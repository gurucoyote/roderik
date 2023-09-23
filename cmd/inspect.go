package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"strconv"
)

var BoxCmd = &cobra.Command{
	Use:   "box",
	Short: "Get the box of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		Box(CurrentElement)
	},
}

var TextCmd = &cobra.Command{
	Use:   "text [length]",
	Short: "Print the text of the current element",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		text := CurrentElement.MustText()
		if len(args) > 0 {
			length, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Println("Error: Invalid length argument")
				return
			}
			if length < len(text) {
				text = text[:length]
			}
		}
		fmt.Println(text)
	},
}

var HtmlCmd = &cobra.Command{
	Use:   "html",
	Short: "Print the HTML of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			url := args[0]
			if isValidURL(url) {
				Page, err := LoadURL(url)
				if err != nil {
					fmt.Println("Error loading URL:", err)
					return
				}
				html := Page.MustHTML()
				fmt.Println(html)
				return
			}
		}
		if !hasCurrentElement() {
			return
		}
		html := CurrentElement.MustHTML()
		fmt.Println(html)
	},
}

func init() {
	RootCmd.AddCommand(BoxCmd)
	RootCmd.AddCommand(TextCmd)
	RootCmd.AddCommand(HtmlCmd)
}
