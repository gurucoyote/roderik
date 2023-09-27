package cmd

import (
	"github.com/spf13/cobra"
)

var ComputedStyleCmd = &cobra.Command{
	Use:   "computedstyles",
	Short: "Output the computed styles of the currently selected element in a format suitable for a .css file",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement command logic
	},
}

func init() {
	RootCmd.AddCommand(ComputedStyleCmd)
}
