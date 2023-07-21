package cmd

import (
	"github.com/spf13/cobra"
)

var NextCmd = &cobra.Command{
	Use:   "next [selector]",
	Short: "Navigate to the next element",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement the logic for next command
	},
}

var PrevCmd = &cobra.Command{
	Use:   "prev [selector]",
	Short: "Navigate to the previous element",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement the logic for prev command
	},
}
