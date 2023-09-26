package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/go-rod/rod/lib/proto"
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
				// TODO: use CurrentElement.Describe(0, false) to get the element's properties
				// TODO: output the resulting struct as a indented JSON string

				// Fetch the partial accessibility tree
				partialAXTree, err := proto.AccessibilityGetPartialAXTree{
					NodeID: nil,
				}.Call(Page)
				if err != nil {
					fmt.Println("Error fetching partial accessibility tree:", err)
					return
				}
				// Print the partial accessibility tree
				fmt.Println(partialAXTree.Nodes)
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
