package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var A11yCmd = &cobra.Command{
	Use:   "a11y",
	Short: "Access the accessibility tree of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		currentTag := CurrentElement.MustDescribe().LocalName
		childrenCount := len(CurrentElement.MustChildren())
		fmt.Printf("Current tag: %s, Children count: %d\n", currentTag, childrenCount)
	},
}

func init() {
	RootCmd.AddCommand(A11yCmd)
}
