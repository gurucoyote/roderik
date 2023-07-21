package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

type Element struct {
	// Define the fields of the Element struct here
}

var NextCmd = &cobra.Command{
	Use:   "next [selector]",
	Short: "Navigate to the next element",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
			if !hasCurrentElement() {
				return
			}
		// ReportElement(CurrentElement)
			nextElement, err := CurrentElement.Next()
			if err != nil {
				fmt.Println("Error navigating to the next element:", err)
				return
			}
			CurrentElement = nextElement
			ReportElement(nextElement)
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
		if CurrentElement == nil {
			return
		}
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
	},
}

func init() {
	WalkCmd.Flags().Int("steps", 4, "Number of steps to walk")
}

var PrevCmd = &cobra.Command{
	Use:   "prev",
	Short: "Navigate to the previous element",
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
			ReportElement(prevElement)
	},
}

var ChildCmd = &cobra.Command{
	Use:   "child",
	Short: "Navigate to the first child of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		childElement := CurrentElement.MustElement(":first-child")
		CurrentElement = childElement
		fmt.Println("Navigated to the child element.")
		ReportElement(CurrentElement)
	},
}

var ParentCmd = &cobra.Command{
	Use:   "parent",
	Short: "Navigate to the parent of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		parentElement, err := CurrentElement.Parent()
		if err != nil {
			fmt.Println("Error navigating to the parent element:", err)
			return
		}
		CurrentElement = parentElement
		fmt.Println("Navigated to the parent element.")
		ReportElement(CurrentElement)
	},
}

var BoxCmd = &cobra.Command{
	Use:   "box",
	Short: "Get the box of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		Box(CurrentElement)
	},
}
