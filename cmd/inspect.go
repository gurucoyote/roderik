package cmd

import (
	"encoding/json"
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
		text, err := CurrentElement.Text()
		if err != nil {
			fmt.Println("Error getting text:", err)
			return
		}
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
				CurrentElement = Page.MustElement("html")
			}
		}
		if !hasCurrentElement() {
			return
		}
		html, err := CurrentElement.HTML()
		if err != nil {
			fmt.Println("Error getting HTML:", err)
			return
		}
		fmt.Println(html)
	},
}

var DescribeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Describe the current element as a JSON string",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		// Get the element's properties
		elementProperties, err := CurrentElement.Describe(0, true) // depth:0, pierce:false
		if err != nil {
			fmt.Println("Error describing element:", err)
			return
		}
		// Convert elementProperties to a JSON string with indentation
		jsonString, err := json.MarshalIndent(elementProperties, "", "  ")
		if err != nil {
			fmt.Println("Error converting to JSON:", err)
			return
		}
		fmt.Println(string(jsonString))
	},
}

var XPathCmd = &cobra.Command{
	Use:   "xpath",
	Short: "Get the optimized xpath of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		xpath, err := CurrentElement.GetXPath(true) // true for optimized xpath
		if err != nil {
			fmt.Println("Error getting xpath:", err)
			return
		}
		fmt.Println(xpath)
	},
}

func init() {
	RootCmd.AddCommand(BoxCmd)
	RootCmd.AddCommand(TextCmd)
	RootCmd.AddCommand(HtmlCmd)
	RootCmd.AddCommand(DescribeCmd)
	RootCmd.AddCommand(XPathCmd)
}
