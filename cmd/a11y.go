package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
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
				// Iterate over the Nodes of the partial tree and output relevant info
				for _, node := range partialAXTree.Nodes {
					// Filter out any nodes that have ignore set to true
					if node.Ignored {
						continue
					}
					// Relevant info: computed string as text, source, number of children and ids
					fmt.Println("Node ID:", node.NodeID)
					if node.Name != nil {
						fmt.Println("Computed string:", node.Name.Value)
						for _, source := range node.Name.Sources {
							fmt.Println("Source:", source.Type)
						}
					}
					fmt.Println("Number of children:", len(node.ChildIds))
					fmt.Println("Child IDs:", node.ChildIds)
				}
				if Verbose {
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
		if !hasCurrentElement() {
			return
		}
		// ReportElement(CurrentElement)
	},
}

func init() {
	RootCmd.AddCommand(A11yCmd)
}
