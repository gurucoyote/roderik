package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/go-rod/rod/lib/proto"
)

var ClickCmd = &cobra.Command{
	Use:   "click",
	Short: "Click on the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		err := CurrentElement.Click(proto.InputMouseButtonLeft, 1)
		if err != nil {
			fmt.Println("Error clicking on the current element:", err)
			return
		}
	},
}

var RClickCmd = &cobra.Command{
	Use:   "rclick",
	Short: "Right click on the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		err := CurrentElement.Click(proto.InputMouseButtonRight, 1)
		if err != nil {
			fmt.Println("Error right clicking on the current element:", err)
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
	RootCmd.AddCommand(RClickCmd)
	RootCmd.AddCommand(TypeCmd)
}
