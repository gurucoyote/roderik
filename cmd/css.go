package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/go-rod/rod/lib/proto"
)

var ComputedStyleCmd = &cobra.Command{
	Use:   "computedstyles",
	Short: "Output the computed styles of the currently selected element in a format suitable for a .css file",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			fmt.Println("No current element selected.")
			return
		}

		// Get the element's properties
		elementProperties, err := CurrentElement.Describe(0, false) // depth:0, pierce:false
		if err != nil {
			fmt.Println("Error describing element:", err)
			return
		}

		// Call the CSSGetComputedStyleForNode function
		computedStyle, err := proto.CSSGetComputedStyleForNode{
			NodeID: elementProperties.NodeID,
		}.Call(Page)
		if err != nil {
			fmt.Println("Error getting computed styles:", err)
			return
		}

		// Output the result as a JSON string
		computedStyleJSON, err := json.MarshalIndent(computedStyle, "", "  ")
		if err != nil {
			fmt.Println("Error converting computed styles to JSON:", err)
			return
		}
		fmt.Println(string(computedStyleJSON))
	},
}

func init() {
	RootCmd.AddCommand(ComputedStyleCmd)
}
