package cmd

import (
	"fmt"
	"strconv"
	"github.com/spf13/cobra"
)

var NextCmd = &cobra.Command{
	Use:   "next [selector]",
	Short: "Navigate to the next element",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
			if !hasCurrentElement() {
				return
			}
			nextElement, err := CurrentElement.Next()
			if err != nil {
				fmt.Println("Error navigating to the next element:", err)
				return
			}
			CurrentElement = nextElement
			fmt.Println("Navigated to the next element.")
			fmt.Println("Tag name of the next element:", nextElement.MustEval("() => this.tagName").String())
			fmt.Println("Text of the next element:", nextElement.MustText())
	},
}

func hasCurrentElement() bool {
	if CurrentElement == nil {
		fmt.Println("Error: CurrentElement is not defined. Please load a page or navigate to an element first.")
		return false
	}
	return true
}

var WalkCmd = &cobra.Command{
	Use:   "walk [steps]",
	Short: "Walk to the next element for a number of steps",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		steps, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Println("Error: Invalid number of steps.")
			return
		}
		for i := 0; i < steps; i++ {
			NextCmd.Run(cmd, []string{})
		}
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
