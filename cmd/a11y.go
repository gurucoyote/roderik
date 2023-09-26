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
				elementProperties, err := CurrentElement.Describe(0, false) // depth:0, pierce:false
				if err != nil {
					fmt.Println("Error describing element:", err)
					return
				}

				// Fetch the partial accessibility tree for this element
				partialAXTree, err := proto.AccessibilityGetPartialAXTree{
					BackendNodeID: elementProperties.BackendNodeID,
				}.Call(Page)
				if err != nil {
					fmt.Println("Error fetching partial accessibility tree:", err)
					return
				}
				if false {
				// debug: print the tree as json
					treeJSON, err := json.MarshalIndent(partialAXTree, "", "  ")
					if err != nil {
						fmt.Println("Error converting node to JSON:", err)
						return
					}
					fmt.Println(string(treeJSON))
				}
			}
		}
		//TODO: iterate over the Nodes of the partial tree and output relevant info
		// filter out any nodes that have ignore set to true
		// relevant info: 
		// - computed string as text
		// - source
		// - numberof children and ids
		if !hasCurrentElement() {
			return
		}
		// ReportElement(CurrentElement)
	},
}

func init() {
	RootCmd.AddCommand(A11yCmd)
}
