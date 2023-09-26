package cmd

import (
	"encoding/json"
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
				// Get the element's properties
				elementProperties, err := CurrentElement.Describe(0, false)
				if err != nil {
					fmt.Println("Error describing element:", err)
					return
				}
				// Convert the element properties to an indented JSON string
				elementPropertiesJSON, err := json.MarshalIndent(elementProperties, "", "  ")
				if err != nil {
					fmt.Println("Error converting element properties to JSON:", err)
					return
				}
				// Output the element properties
				fmt.Println(string(elementPropertiesJSON))

				// Fetch the partial accessibility tree
				partialAXTree, err := proto.AccessibilityGetPartialAXTree{
					BackendNodeID: elementProperties.BackendNodeID,
				}.Call(Page)
				if err != nil {
					fmt.Println("Error fetching partial accessibility tree:", err)
					return
				}
				// Debug: Iterate over the nodes in the partial accessibility tree and print their properties
				for i, node := range partialAXTree.Nodes {
					fmt.Printf("Node %d:\n", i)
					nodeJSON, err := json.MarshalIndent(node, "", "  ")
					if err != nil {
						fmt.Println("Error converting node to JSON:", err)
						return
					}
					fmt.Println(string(nodeJSON))
				}
			}
		}
		if !hasCurrentElement() {
			return
		}
		// ReportElement(CurrentElement)
	},
}

func init() {
	RootCmd.AddCommand(A11yCmd)
}
