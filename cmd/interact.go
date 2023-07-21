package cmd

import (
	"github.com/spf13/cobra"
)

var ClickCmd = &cobra.Command{
	Use:   "click",
	Short: "Click on the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		err := CurrentElement.Click()
		if err != nil {
			fmt.Println("Error clicking on the current element:", err)
			return
		}
	},
}

var TypeCmd = &cobra.Command{
	Use:   "type",
	Short: "Type text into the current element",
	Run: func(cmd *cobra.Command, args []string) {
		// Add your code here
	},
}

func init() {
	RootCmd.AddCommand(ClickCmd)
	RootCmd.AddCommand(TypeCmd)
}
