package cmd

import (
	"fmt"
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
	Use:   "walk",
	Short: "Walk to the next element for a number of steps",
	Run: func(cmd *cobra.Command, args []string) {
		steps, _ := cmd.Flags().GetInt("steps")
		if steps < 0 {
			steps = -steps
			for i := 0; i < steps; i++ {
				PrevCmd.Run(cmd, []string{})
			}
		} else {
			for i := 0; i < steps; i++ {
				NextCmd.Run(cmd, []string{})
			}
		}
		if CurrentElement != nil {
			fmt.Println("Tag name of the first element after walk:", CurrentElement.MustEval("() => this.tagName").String())
			fmt.Println("Text of the first element after walk:", CurrentElement.MustText())
		}
	},
}

func init() {
	WalkCmd.Flags().Int("steps", 4, "Number of steps to walk")
}

var PrevCmd = &cobra.Command{
	Use:   "prev [selector]",
	Short: "Navigate to the previous element",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		prevElement, err := CurrentElement.Previous()
		if err != nil {
			fmt.Println("Error navigating to the previous element:", err)
			return
		}
		CurrentElement = prevElement
		fmt.Println("Navigated to the previous element.")
		fmt.Println("Tag name of the previous element:", prevElement.MustEval("() => this.tagName").String())
		fmt.Println("Text of the previous element:", prevElement.MustText())
	},
}
