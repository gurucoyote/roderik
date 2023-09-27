package cmd

import (
	// "encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	// "github.com/go-rod/rod/lib/proto"
)

var ComputedStyleCmd = &cobra.Command{
	Use:   "computedstyles",
	Short: "Output the computed styles of the currently selected element in a format suitable for a .css file",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			fmt.Println("No current element selected.")
			return
		}
		// Evaluate JavaScript code to get the computed styles of the element
		// Print the computed styles
		styles := CurrentElement.MustEval(`() => {
		// Get the computed style for the element
		var style = window.getComputedStyle(this);
		var styleObject = {};
		// Iterate over each style
		for (var i = 0; i < style.length; i++) {
			var prop = style[i];
			var value = style.getPropertyValue(prop);
			// Only add the style to the object if it has a value
			if (value) {
				styleObject[prop] = value;
			}
		}
		return styleObject;
	}`)
		fmt.Println(PrettyFormat(styles))
	},
}

func init() {
	RootCmd.AddCommand(ComputedStyleCmd)
}
