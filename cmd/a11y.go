package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
	"github.com/spf13/cobra"
)

var QaccCmd = &cobra.Command{
	Use:   "qacc",
	Short: "Query the accessibility tree of the current element",
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
		// Get the element's properties
		elementProperties, err := CurrentElement.Describe(0, false) // depth:0, pierce:false
		if err != nil {
			fmt.Println("Error describing element:", err)
			return
		}

		// Query the accessibility tree for this element
		queryAXTree, err := proto.AccessibilityQueryAXTree{
			BackendNodeID: elementProperties.BackendNodeID,
			// I think we can specify a depth, not sure if it makes sense to da that here
		}.Call(Page)
		if err != nil {
			fmt.Println("Error querying accessibility tree:", err)
			return
		}
		// Iterate over the Nodes of the queried tree and output relevant info
		for _, node := range queryAXTree.Nodes {
			// Filter out any nodes that have ignore set to true
			if node.Ignored {
				continue
			}
			if Verbose {
			// Relevant info: computed string as text, source, number of children, ids and role
			fmt.Println("Node ID:", node.NodeID)
			fmt.Println("Role:", node.Role.Value)
			fmt.Println("Backend DOM Node ID:", node.BackendDOMNodeID)
			fmt.Println("Parent ID:", node.ParentID)
			if node.Name != nil {
				fmt.Println("Name:", node.Name.Value)
			}
			fmt.Println("Number of children:", len(node.ChildIds))
			fmt.Println("Child IDs:", node.ChildIds)
		} else {
			fmt.Print( node.NodeID, ": ")
			switch node.Role.Value.String(){
			case "\"paragraph\"":
				fmt.Println()
			default:
			fmt.Print( node.Role.Value.String(), ": ")
		}
			if node.Name != nil {
				fmt.Println( node.Name.Value)
			}
		}
		}
		if Verbose {
			// debug: print the tree as json
			treeJSON, err := json.MarshalIndent(queryAXTree, "", "  ")
			if err != nil {
				fmt.Println("Error converting node to JSON:", err)
				return
			}
			fmt.Println(string(treeJSON))
		}
	},
}

func init() {
	RootCmd.AddCommand(QaccCmd)
}
